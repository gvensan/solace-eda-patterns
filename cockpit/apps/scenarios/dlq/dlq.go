// Package dlq implements the dead letter queue scenario.
//
// The shop's order intake reads new orders from a queue and passes them on
// for shipping. Now and then an order arrives that can never be processed:
// here, one with no delivery address. Retrying it forever would waste the
// intake's time and, on a queue read in order, hold up every order behind it.
// The queue gives it a few attempts, then moves it to a dead message queue
// (Solace's name for a dead letter queue) where someone can look at it, and
// the orders behind it keep flowing.
package dlq

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// Shop publishes good orders at a steady pace, so there is always normal
// traffic around a bad one.
func Shop(args []string) {
	fs := flag.NewFlagSet("dlq shop", flag.ExitOnError)
	user := fs.String("user", "svc-dlq-shop", "client username to connect as")
	role := fs.String("role", "shop", "label for log lines")
	every := fs.Duration("every", 500*time.Millisecond, "pause between orders")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "dlq-shop")
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	solace.Logf(*role, "placing an order every %s. Press Stop to end.", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()

	for n := 1; ; n++ {
		order := fmt.Sprintf("O-%05d", 40000+n)
		payload := fmt.Sprintf(`{"order":"%s","address":"%d Market Street","eligible":"default"}`, order, n)
		if err := solace.PublishKeyed(svc, pub, "dlq/shop/order/placed/v1/"+order, payload, ""); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else if n%10 == 0 {
			solace.Logf(*role, "%d orders placed (latest %s)", n, order)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Poison publishes one order that can never be processed (it has no
// address), and exits. --eligible says what to tell the broker about
// moving it to a dead message queue: "default" leaves the flag unset, which
// the Go API sends as eligible; "false" marks it not eligible.
func Poison(args []string) {
	fs := flag.NewFlagSet("dlq poison", flag.ExitOnError)
	user := fs.String("user", "svc-dlq-shop", "client username to connect as")
	role := fs.String("role", "shop", "label for log lines")
	eligible := fs.String("eligible", "default", "default or false")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "dlq-poison")
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	order := fmt.Sprintf("P-%d", time.Now().Unix()%100000)
	payload := fmt.Sprintf(`{"order":"%s","address":"","eligible":"%s"}`, order, *eligible)
	b := svc.MessageBuilder()
	if *eligible == "false" {
		b = b.WithProperty(config.MessagePropertyPersistentDMQEligible, false)
	}
	msg, err := b.BuildWithStringPayload(payload)
	if err != nil {
		solace.Errf(*role, "could not build the order: %s", err)
		os.Exit(1)
	}
	if err := pub.PublishAwaitAcknowledgement(msg, resource.TopicOf("dlq/shop/order/placed/v1/"+order), 5*time.Second, nil); err != nil {
		solace.Errf(*role, "the broker did not take order %s: %s", order, err)
		os.Exit(1)
	}
	solace.Logf(*role, "placed order %s with no delivery address (DMQ-eligible: %s)", order, *eligible)
}

// Intake reads orders and passes the good ones on. An order without an
// address is settled FAILED, which hands it back to the queue for another
// attempt; the queue decides when to give up on it.
func Intake(args []string) {
	fs := flag.NewFlagSet("dlq intake", flag.ExitOnError)
	user := fs.String("user", "svc-dlq-intake", "client username to connect as")
	role := fs.String("role", "intake", "label for log lines")
	clientName := fs.String("client-name", "dlq-intake", "client name the broker reports this app by")
	queue := fs.String("queue", "q.dlq.orders", "queue to read orders from")
	work := fs.Duration("work", 150*time.Millisecond, "time to check one order")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{
		ClientAck: true,
		Outcomes:  []config.MessageSettlementOutcome{config.PersistentReceiverFailedOutcome},
	})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "checking orders from %s.", *queue)
	fmt.Println()

	// The Go API does not expose a message's delivery count, so attempts are
	// counted here, by order.
	var mu sync.Mutex
	attempts := map[string]int{}
	accepted := 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(*work)
		payload := solace.PayloadOf(msg)
		order := solace.FieldString(payload, "order")

		if solace.FieldString(payload, "address") != "" {
			mu.Lock()
			accepted++
			n := accepted
			mu.Unlock()
			if n%10 == 0 {
				solace.Logf(*role, "accepted %d orders (latest %s)", n, order)
			}
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(*role, "could not acknowledge %s: %s", order, err)
			}
			return
		}

		mu.Lock()
		attempts[order]++
		n := attempts[order]
		mu.Unlock()
		line := fmt.Sprintf("[%s] FAILED %s: no delivery address, attempt %d (eligible=%s)",
			*role, order, n, solace.FieldString(payload, "eligible"))
		if msg.IsRedelivered() {
			line += "  (redelivered)"
		}
		fmt.Println(line)
		if err := rcv.Settle(msg, config.PersistentReceiverFailedOutcome); err != nil {
			solace.Errf(*role, "could not settle %s: %s", order, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	fmt.Println()
	solace.Logf(*role, "stopped after accepting %d order(s).", accepted)
}

// Watch shows what lands on the dead message queue.
//
// It reads without acknowledging, so every parked order stays on the DMQ for
// someone to deal with; the Go API has no queue browser, which would be the
// natural tool. A restarted watcher shows the same orders again, marked
// redelivered.
func Watch(args []string) {
	fs := flag.NewFlagSet("dlq watch", flag.ExitOnError)
	user := fs.String("user", "svc-dlq-ops", "client username to connect as")
	role := fs.String("role", "dmq-watch", "label for log lines")
	queue := fs.String("queue", "q.dlq.orders.dmq", "dead message queue to watch")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "dlq-dmq-watch")
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "watching %s. Parked orders are shown, not removed.", *queue)
	fmt.Println()

	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		payload := solace.PayloadOf(msg)
		line := fmt.Sprintf("[%s] parked %s, published to %s (eligible=%s)", *role,
			solace.FieldString(payload, "order"), msg.GetDestinationName(), solace.FieldString(payload, "eligible"))
		if msg.IsRedelivered() {
			line += "  (shown before)"
		}
		fmt.Println(line)
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}
