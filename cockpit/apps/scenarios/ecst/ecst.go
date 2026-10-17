// Package ecst implements the event-carried state transfer scenario.
//
// A shop. The customer service owns customer records and publishes the whole
// record every time one changes. The order service publishes each order with
// only the customer's id. Two services need the customer's details for an
// order:
//
//   - Shipping keeps its own copy of every customer, built from the
//     customer events, and looks the address up locally. That is event-carried
//     state transfer: the event carries the state.
//   - Invoicing keeps nothing and asks the customer service for each order,
//     by request-reply. That is the usual alternative, a thin event plus a
//     call back.
//
// The cards show what each choice costs: invoicing stops when the customer
// service does, and shipping is only as good as its copy.
package ecst

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
	customerTopic = "ecst/crm/customer/updated/v1/"
	lookupTopic   = "ecst/crm/customer/get/v1/"
	orderTopic    = "ecst/shop/order/placed/v1/"
)

var (
	names  = []string{"Ann Lee", "Bo Chen", "Cy Diaz", "Di Moss", "Ed Park", "Flo Ruiz", "Gus Tan", "Hal Webb"}
	cities = []string{"Lyon", "Leeds", "Porto", "Turin", "Ghent", "Bonn", "Cork", "Graz", "Malmo", "Split"}
)

type customer struct {
	id, name, city string
	version        int
}

func (c customer) json() string {
	return fmt.Sprintf(`{"customer":"%s","version":%d,"name":"%s","city":"%s"}`, c.id, c.version, c.name, c.city)
}

func customerID(i int) string { return fmt.Sprintf("C-%02d", i+1) }

// Customers owns the customer records. On start it publishes every record
// once, then moves one customer to a new city every --every and publishes the
// whole record again. It also answers lookups by request-reply. SIGUSR1 makes
// it publish every record again, for a consumer that has lost its copy.
func Customers(args []string) {
	fs := flag.NewFlagSet("ecst customers", flag.ExitOnError)
	user := fs.String("user", "svc-ecst-crm", "client username to connect as")
	role := fs.String("role", "customers", "label for log lines")
	clientName := fs.String("client-name", "ecst-customers", "client name the broker reports this app by")
	every := fs.Duration("every", 4*time.Second, "pause between customer changes")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	var mu sync.Mutex
	book := make([]customer, len(names))
	for i := range book {
		book[i] = customer{id: customerID(i), name: names[i], city: cities[i], version: 1}
	}
	publish := func(c customer) {
		if err := solace.PublishConfirmed(svc, pub, customerTopic+c.id, c.json()); err != nil {
			solace.Errf(*role, "could not publish %s: %s", c.id, err)
		}
	}
	publishAll := func(why string) {
		mu.Lock()
		all := append([]customer(nil), book...)
		mu.Unlock()
		for _, c := range all {
			publish(c)
		}
		solace.Logf(*role, "published all %d customer records (%s)", len(all), why)
	}

	// Lookups, for a service that keeps no copy.
	rcv, err := svc.RequestReply().CreateRequestReplyMessageReceiverBuilder().
		Build(resource.TopicSubscriptionOf(lookupTopic + ">"))
	if err == nil {
		err = rcv.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start answering lookups: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(rcv)
	if err := rcv.ReceiveAsync(func(req message.InboundMessage, replier sol.Replier) {
		id := strings.TrimPrefix(req.GetDestinationName(), lookupTopic)
		mu.Lock()
		reply := `{"error":"no such customer"}`
		for _, c := range book {
			if c.id == id {
				reply = c.json()
			}
		}
		mu.Unlock()
		msg, err := svc.MessageBuilder().BuildWithStringPayload(reply)
		if err == nil {
			err = replier.Reply(msg)
		}
		if err != nil {
			solace.Errf(*role, "could not answer the lookup for %s: %s", id, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the lookup handler: %s", err)
		os.Exit(1)
	}

	solace.Logf(*role, "answering lookups on %s>", lookupTopic)
	publishAll("start")
	fmt.Println()

	again := make(chan os.Signal, 1)
	signal.Notify(again, syscall.SIGUSR1)
	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(*every)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-again:
			publishAll("asked to send everything again")
			continue
		case <-tick.C:
		}
		mu.Lock()
		i := rand.Intn(len(book))
		c := &book[i]
		for {
			city := cities[rand.Intn(len(cities))]
			if city != c.city {
				c.city = city
				break
			}
		}
		c.version++
		changed := *c
		mu.Unlock()
		publish(changed)
		solace.Logf(*role, "%s moved to %s (version %d), published the whole record", changed.id, changed.city, changed.version)
	}
}

// Orders publishes an order a second. The event names the customer by id
// and carries nothing else about them.
func Orders(args []string) {
	fs := flag.NewFlagSet("ecst orders", flag.ExitOnError)
	user := fs.String("user", "svc-ecst-orders", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	clientName := fs.String("client-name", "ecst-orders", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between orders")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	solace.Logf(*role, "placing an order every %s on %s<customer>", *every, orderTopic)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for n := 1; ; n++ {
		order := fmt.Sprintf("O-%05d", 10000+n)
		cust := customerID(rand.Intn(len(names)))
		if err := solace.PublishConfirmed(svc, pub, orderTopic+cust, fmt.Sprintf(`{"order":"%s","customer":"%s"}`, order, cust)); err != nil {
			solace.Errf(*role, "order %s refused: %s", order, err)
		} else {
			solace.Logf(*role, "%s placed for %s", order, cust)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Shipping keeps its own copy of every customer, from the customer events on
// its queue, and ships each order to the city in that copy. It never asks
// anyone. An order for a customer it has no copy of waits until the copy
// arrives. The copy lives in memory, so a restart starts with none.
func Shipping(args []string) {
	fs := flag.NewFlagSet("ecst shipping", flag.ExitOnError)
	user := fs.String("user", "svc-ecst-shipping", "client username to connect as")
	role := fs.String("role", "shipping", "label for log lines")
	clientName := fs.String("client-name", "ecst-shipping", "client name the broker reports this app by")
	queue := fs.String("queue", "q.ecst.shipping", "queue with orders and customer records")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	solace.Logf(*role, "starting with no copy of any customer")
	fmt.Println()

	copyOf := map[string]customer{}
	held := map[string][]string{}
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		id := solace.FieldString(p, "customer")
		if strings.HasPrefix(msg.GetDestinationName(), customerTopic) {
			v, _ := solace.FieldInt(p, "version")
			c := customer{id: id, name: solace.FieldString(p, "name"), city: solace.FieldString(p, "city"), version: v}
			copyOf[id] = c
			solace.Logf(*role, "copy of %s updated: %s (version %d)", id, c.city, v)
			for _, order := range held[id] {
				solace.Logf(*role, "%s for %s: copy arrived, ships to %s", order, id, c.city)
			}
			delete(held, id)
		} else {
			order := solace.FieldString(p, "order")
			if c, ok := copyOf[id]; ok {
				solace.Logf(*role, "%s for %s: ships to %s (from my copy, version %d)", order, id, c.city, c.version)
			} else {
				held[id] = append(held[id], order)
				solace.Logf(*role, "%s for %s: no copy of %s yet, parcel held", order, id, id)
			}
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}

// Invoicing keeps no copy. For each order it asks the customer service for
// the customer's name, and waits up to --timeout. If nobody answers, it
// tries the same order again and does not take the next one: an invoice
// needs the name.
func Invoicing(args []string) {
	fs := flag.NewFlagSet("ecst invoicing", flag.ExitOnError)
	user := fs.String("user", "svc-ecst-invoicing", "client username to connect as")
	role := fs.String("role", "invoicing", "label for log lines")
	clientName := fs.String("client-name", "ecst-invoicing", "client name the broker reports this app by")
	queue := fs.String("queue", "q.ecst.invoicing", "queue with orders")
	timeout := fs.Duration("timeout", 2*time.Second, "how long to wait for each lookup")
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
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	solace.Logf(*role, "asking the customer service about every order, waiting up to %s", *timeout)
	fmt.Println()

	stopping := make(chan struct{})
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		order, id := solace.FieldString(p, "order"), solace.FieldString(p, "customer")
		for {
			q, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"customer":"%s"}`, id))
			if err != nil {
				solace.Errf(*role, "could not build the lookup: %s", err)
				return
			}
			began := time.Now()
			reply, err := ask.PublishAwaitResponse(q, resource.TopicOf(lookupTopic+id), *timeout, nil)
			if err == nil {
				name := solace.FieldString(solace.PayloadOf(reply), "name")
				solace.Logf(*role, "%s billed to %s (asked the customer service, %d ms)", order, name, time.Since(began).Milliseconds())
				break
			}
			solace.Errf(*role, "%s: lookup of %s failed: %s", order, id, err)
			solace.Errf(*role, "%s: not billed yet, asking again", order)
			select {
			case <-stopping:
				return
			case <-time.After(time.Second):
			}
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge %s: %s", order, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
	close(stopping)
}
