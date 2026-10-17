// Package dlq implements the dead message queue scenario.
//
// The shop's order intake reads new orders from a queue and passes them on
// for shipping. Now and then an order arrives that can never be processed:
// here, one with no delivery address. Retrying it forever would waste the
// intake's time and, on a queue read in order, hold up every order behind it.
// The queue gives it a few attempts, then moves it to a dead message queue
// (what other brokers and the pattern guide call a dead letter queue), where
// someone can look at it, and the orders behind it keep flowing.
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

// Expiring places orders that are only worth processing for a few seconds,
// and exits: each holds a payment authorisation that lapses, so the
// publisher gives it a time to live. A queue that respects time to live
// removes an order that has waited longer and, because it names a dead
// message queue, moves it there. Run while the intake is stopped, they
// expire before anyone reads them.
func Expiring(args []string) {
	fs := flag.NewFlagSet("dlq expiring", flag.ExitOnError)
	user := fs.String("user", "svc-dlq-shop", "client username to connect as")
	role := fs.String("role", "shop", "label for log lines")
	count := fs.Int("count", 3, "orders to place")
	ttl := fs.Duration("ttl", 5*time.Second, "time to live of each order; 0 means it never expires")
	fs.Parse(args)

	// The API takes a time to live in whole milliseconds, and 0 means the
	// message never expires (it is also the default). So 0 sends none, and
	// anything that would round down to 0, or is negative, is refused
	// rather than quietly meaning "never".
	if *ttl < 0 || (*ttl > 0 && ttl.Milliseconds() == 0) {
		solace.Errf(*role, "time to live %s: use 0 (never expires) or 1ms or more", *ttl)
		os.Exit(2)
	}
	hold := "none"
	if *ttl > 0 {
		hold = ttl.String()
	}

	svc := solace.ConnectAs(*role, *user, "dlq-expiring")
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	for n := 1; n <= *count; n++ {
		order := fmt.Sprintf("H-%d", time.Now().UnixMilli()%100000)
		payload := fmt.Sprintf(`{"order":"%s","address":"%d Market Street","eligible":"default","hold":"%s"}`, order, n, hold)
		b := svc.MessageBuilder()
		if *ttl > 0 {
			b = b.WithProperty(config.MessagePropertyPersistentTimeToLive, ttl.Milliseconds())
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
		if *ttl > 0 {
			solace.Logf(*role, "placed order %s, valid for %s (time to live)", order, *ttl)
		} else {
			solace.Logf(*role, "placed order %s with no time to live: it never expires", order)
		}
		time.Sleep(10 * time.Millisecond)
	}
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
		// Why it was parked is not on the message: an expired order arrives
		// with no expiry time (the API reports the Unix epoch). The broker
		// counts the reasons on q.dlq.orders, which the diagram shows. What
		// the shop said about the order is in its payload.
		if hold := solace.FieldString(payload, "hold"); hold != "" && hold != "none" {
			line += fmt.Sprintf(", was valid for %s", hold)
		}
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
