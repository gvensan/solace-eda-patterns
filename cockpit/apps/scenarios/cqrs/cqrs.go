// Package cqrs implements the command query responsibility segregation
// scenario.
//
// A shop splits writing from reading. Checkout sends commands ("place this
// order") to a queue. The order service, the write side, takes each command,
// gives the order its number and publishes an "order placed" event. The order
// history service, the read side, builds its own view from those events and
// answers questions from it ("how many orders has C-03 placed?"), asked by
// the account page. The two sides share no store: the events on the broker
// are the only link, so each can stop, slow down or scale on its own, and
// the view is always a little behind the writes.
package cqrs

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

const (
	commandTopic = "cqrs/shop/command/place-order/v1/"
	eventTopic   = "cqrs/shop/order/placed/v1/"
	queryTopic   = "cqrs/shop/query/order-history/v1/"
	customers    = 6
)

func customerID(i int) string { return fmt.Sprintf("C-%02d", i+1) }

// Checkout sends a "place order" command a second to the command queue's
// topic and waits for the broker to accept it. It does not wait for the
// order service: a command on a queue is kept until it is handled.
func Checkout(args []string) {
	fs := flag.NewFlagSet("cqrs checkout", flag.ExitOnError)
	user := fs.String("user", "svc-cqrs-checkout", "client username to connect as")
	role := fs.String("role", "checkout", "label for log lines")
	clientName := fs.String("client-name", "cqrs-checkout", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between commands")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	solace.Logf(*role, "sending a place-order command every %s", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for n := 1; ; n++ {
		cust := customerID(rand.Intn(customers))
		amount := 10 + rand.Intn(90)
		payload := fmt.Sprintf(`{"command":%d,"customer":"%s","amount":%d}`, n, cust, amount)
		if err := solace.PublishConfirmed(svc, pub, commandTopic+cust, payload); err != nil {
			solace.Errf(*role, "command #%d refused: %s", n, err)
		} else {
			solace.Logf(*role, "command #%d: place an order for %s (%d.00), accepted by the broker", n, cust, amount)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Write is the command side. It takes each command from the command queue,
// gives the order the next number, publishes "order placed" and only then
// acknowledges the command.
func Write(args []string) {
	fs := flag.NewFlagSet("cqrs write", flag.ExitOnError)
	user := fs.String("user", "svc-cqrs-write", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	clientName := fs.String("client-name", "cqrs-orders", "client name the broker reports this app by")
	queue := fs.String("queue", "q.cqrs.commands", "command queue")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()

	// The order number would come from the write side's own database. It
	// starts from the clock so a restart never reuses a number.
	next := int(time.Now().Unix() % 100000)
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		n, _ := solace.FieldInt(p, "command")
		amount, _ := solace.FieldInt(p, "amount")
		cust := solace.FieldString(p, "customer")
		next++
		event := fmt.Sprintf(`{"order":%d,"customer":"%s","amount":%d,"placedAt":%d}`, next, cust, amount, time.Now().UnixMilli())
		if err := solace.PublishConfirmed(svc, pub, eventTopic+cust, event); err != nil {
			solace.Errf(*role, "command #%d: could not publish the event: %s", n, err)
			return
		}
		solace.Logf(*role, "command #%d: order #%d placed for %s, event published", n, next, cust)
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge command #%d: %s", n, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}

type history struct {
	orders, spent int
}

// View is the query side. It applies each "order placed" event to its own
// view, an order count and total per customer, and answers questions from
// that view by request-reply. It never reads the write side's data.
// SIGUSR1 makes it slow (--slow per event), SIGUSR2 normal again.
func View(args []string) {
	fs := flag.NewFlagSet("cqrs view", flag.ExitOnError)
	user := fs.String("user", "svc-cqrs-view", "client username to connect as")
	role := fs.String("role", "history", "label for log lines")
	clientName := fs.String("client-name", "cqrs-history", "client name the broker reports this app by")
	queue := fs.String("queue", "q.cqrs.view", "queue with order events")
	slow := fs.Duration("slow", 2*time.Second, "time to apply one event when slowed down")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	var mu sync.Mutex
	view := map[string]*history{}
	lastOrder, lag := 0, time.Duration(0)
	var slowed bool

	answers, err := svc.RequestReply().CreateRequestReplyMessageReceiverBuilder().
		Build(resource.TopicSubscriptionOf(queryTopic + ">"))
	if err == nil {
		err = answers.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start answering queries: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(answers)
	if err := answers.ReceiveAsync(func(req message.InboundMessage, replier sol.Replier) {
		cust := strings.TrimPrefix(req.GetDestinationName(), queryTopic)
		mu.Lock()
		h := view[cust]
		if h == nil {
			h = &history{}
		}
		reply := fmt.Sprintf(`{"customer":"%s","orders":%d,"spent":%d,"asOf":%d,"lagMs":%d}`, cust, h.orders, h.spent, lastOrder, lag.Milliseconds())
		mu.Unlock()
		msg, err := svc.MessageBuilder().BuildWithStringPayload(reply)
		if err == nil {
			err = replier.Reply(msg)
		}
		if err != nil {
			solace.Errf(*role, "could not answer about %s: %s", cust, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the query handler: %s", err)
		os.Exit(1)
	}

	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	solace.Logf(*role, "answering order history questions on %s<customer>", queryTopic)
	fmt.Println()

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range sig {
			on := s == syscall.SIGUSR1
			mu.Lock()
			slowed = on
			mu.Unlock()
			if on {
				solace.Logf(*role, "slowed down: %s to apply each event", *slow)
			} else {
				solace.Logf(*role, "back to normal speed")
			}
		}
	}()

	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		mu.Lock()
		s := slowed
		mu.Unlock()
		if s {
			time.Sleep(*slow)
		}
		p := solace.PayloadOf(msg)
		order, _ := solace.FieldInt(p, "order")
		amount, _ := solace.FieldInt(p, "amount")
		placed, _ := solace.FieldInt(p, "placedAt")
		cust := solace.FieldString(p, "customer")
		mu.Lock()
		h := view[cust]
		if h == nil {
			h = &history{}
			view[cust] = h
		}
		h.orders++
		h.spent += amount
		lastOrder = order
		lag = time.Since(time.UnixMilli(int64(placed)))
		noun := "orders"
		if h.orders == 1 {
			noun = "order"
		}
		line := fmt.Sprintf("order #%d applied: %s now has %d %s, %.1f s after it was placed", order, cust, h.orders, noun, lag.Seconds())
		mu.Unlock()
		solace.Logf(*role, "%s", line)
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge order #%d: %s", order, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}

// Account asks the order history service about a customer every --every,
// and prints the answer with how far behind the writes the view was.
func Account(args []string) {
	fs := flag.NewFlagSet("cqrs account", flag.ExitOnError)
	user := fs.String("user", "svc-cqrs-account", "client username to connect as")
	role := fs.String("role", "account", "label for log lines")
	clientName := fs.String("client-name", "cqrs-account", "client name the broker reports this app by")
	every := fs.Duration("every", 2*time.Second, "pause between questions")
	timeout := fs.Duration("timeout", 2*time.Second, "how long to wait for an answer")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	ask, err := svc.RequestReply().CreateRequestReplyMessagePublisherBuilder().Build()
	if err == nil {
		err = ask.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start asking: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(ask)
	solace.Logf(*role, "asking the order history service about a customer every %s", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for {
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
		cust := customerID(rand.Intn(customers))
		q, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"customer":"%s"}`, cust))
		if err != nil {
			continue
		}
		began := time.Now()
		reply, err := ask.PublishAwaitResponse(q, resource.TopicOf(queryTopic+cust), *timeout, nil)
		if err != nil {
			solace.Errf(*role, "%s: no answer: %s", cust, err)
			continue
		}
		p := solace.PayloadOf(reply)
		orders, _ := solace.FieldInt(p, "orders")
		spent, _ := solace.FieldInt(p, "spent")
		asOf, _ := solace.FieldInt(p, "asOf")
		lagMs, _ := solace.FieldInt(p, "lagMs")
		noun := "orders"
		if orders == 1 {
			noun = "order"
		}
		solace.Logf(*role, "%s: %d %s, %d.00 spent (view as of order #%d, %.1f s behind; answered in %d ms)",
			cust, orders, noun, spent, asOf, float64(lagMs)/1000, time.Since(began).Milliseconds())
	}
}
