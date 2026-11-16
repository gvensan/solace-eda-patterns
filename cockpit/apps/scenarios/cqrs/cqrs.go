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
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
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
	// Command ids start with the time checkout started, so a restarted
	// checkout never reuses one the order service has already placed.
	start := time.Now().Format("150405")
	solace.Logf(*role, "sending a place-order command every %s", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for n := 1; ; n++ {
		id := fmt.Sprintf("%s-%d", start, n)
		cust := customerID(rand.Intn(customers))
		amount := 10 + rand.Intn(90)
		payload := fmt.Sprintf(`{"command":"%s","customer":"%s","amount":%d}`, id, cust, amount)
		if err := solace.PublishConfirmed(svc, pub, commandTopic+cust, payload); err != nil {
			solace.Errf(*role, "command %s refused: %s", id, err)
		} else {
			solace.Logf(*role, "command %s: place an order for %s (%d.00), accepted by the broker", id, cust, amount)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// databasePath is the order service's database: a file under the
// dashboard's state folder, so Clear broker config removes it with the
// scenario's state.
func databasePath() string {
	base := os.Getenv("COCKPIT_STATE_DIR")
	if base == "" {
		base = "../../.state"
	}
	dir := filepath.Join(base, "cqrs")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "orders.jsonl")
}

type order struct {
	Order    int    `json:"order"`
	Customer string `json:"customer"`
	Amount   int    `json:"amount"`
	Command  string `json:"command,omitempty"`
	PlacedAt int64  `json:"placedAt,omitempty"`
}

func (o order) event() string {
	return fmt.Sprintf(`{"order":%d,"customer":"%s","amount":%d,"placedAt":%d}`, o.Order, o.Customer, o.Amount, o.PlacedAt)
}

// shownOrders is how many of the latest orders the write side's table shows.
const shownOrders = 8

// Write is the command side. It takes each command from the command queue,
// gives the order the next number, writes it to its own database, publishes
// "order placed" and only then acknowledges the command. The database is a
// file, one order a line, so a restart carries on from the last number.
//
// Each order keeps the id of its command. A crash after saving an order but
// before acknowledging its command brings the command back; the order is
// found by that id and its event published again, so one command never
// becomes two orders, and an event lost in the crash still goes out.
func Write(args []string) {
	fs := flag.NewFlagSet("cqrs write", flag.ExitOnError)
	user := fs.String("user", "svc-cqrs-write", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	clientName := fs.String("client-name", "cqrs-orders", "client name the broker reports this app by")
	queue := fs.String("queue", "q.cqrs.commands", "command queue")
	forget := fs.Bool("forget", false, "delete the order database, and exit")
	// For testing recovery only: exit as if killed once order n is saved,
	// before its event is published, or once it is published, before its
	// command is acknowledged.
	crashSaved := fs.Int("crash-after-save", 0, "test only: exit right after saving this order")
	crashPublished := fs.Int("crash-after-publish", 0, "test only: exit right after publishing this order's event")
	fs.Parse(args)

	path := databasePath()
	if *forget {
		_ = os.Remove(path)
		solace.Logf(*role, "deleted the order database")
		return
	}

	// A kill mid-write can leave half a line at the end; it is skipped.
	var book []order
	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var o order
			if json.Unmarshal(sc.Bytes(), &o) == nil && o.Order > 0 {
				book = append(book, o)
			}
		}
		f.Close()
		solace.Logf(*role, "opened its order database: %d orders", len(book))
	} else {
		solace.Logf(*role, "new order database: no orders yet")
	}
	db, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		solace.Errf(*role, "could not open the order database: %s", err)
		os.Exit(1)
	}
	defer db.Close()

	held := solace.NewHolding("The order database: every order, as placed", "order", "customer", "amount")
	report := func() {
		var rows [][]string
		for i := len(book) - 1; i >= 0 && len(rows) < shownOrders; i-- {
			o := book[i]
			rows = append(rows, []string{"#" + strconv.Itoa(o.Order), o.Customer, fmt.Sprintf("%d.00", o.Amount)})
		}
		note := "No orders yet."
		if len(book) > 0 {
			note = fmt.Sprintf("%d orders in all, the latest %d shown. Latest: order #%d.", len(book), len(rows), book[len(book)-1].Order)
		}
		held.Set(rows, note)
	}
	report()

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()

	next := 0
	if len(book) > 0 {
		next = book[len(book)-1].Order
	}
	placed := map[string]order{}
	for _, o := range book {
		if o.Command != "" {
			placed[o.Command] = o
		}
	}
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		id := solace.FieldString(p, "command")
		amount, _ := solace.FieldInt(p, "amount")
		cust := solace.FieldString(p, "customer")
		o, again := placed[id]
		if !again {
			next++
			o = order{Order: next, Customer: cust, Amount: amount, Command: id, PlacedAt: time.Now().UnixMilli()}
			line, _ := json.Marshal(o)
			if _, err := db.Write(append(line, '\n')); err != nil {
				solace.Errf(*role, "command %s: could not save order #%d: %s", id, next, err)
				return
			}
			placed[id] = o
			book = append(book, o)
			report()
			if o.Order == *crashSaved {
				solace.Errf(*role, "test: exiting after saving order #%d, before publishing its event", o.Order)
				os.Exit(137)
			}
		}
		if err := solace.PublishConfirmed(svc, pub, eventTopic+o.Customer, o.event()); err != nil {
			// Taking the next command would publish a later order first, and
			// the view, which ignores any order at or below its latest, would
			// never apply this one. A restart publishes it again, in order.
			solace.Errf(*role, "command %s: could not publish the event: %s", id, err)
			solace.Errf(*role, "stopping, so a restart publishes order #%d's event before any later order", o.Order)
			os.Exit(1)
		}
		if !again && o.Order == *crashPublished {
			solace.Errf(*role, "test: exiting after publishing order #%d, before acknowledging command %s", o.Order, id)
			os.Exit(137)
		}
		if again {
			solace.Logf(*role, "command %s was already placed as order #%d before the restart: not placed twice, its event published again", id, o.Order)
		} else {
			solace.Logf(*role, "command %s: order #%d placed for %s, event published", id, o.Order, cust)
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge command %s: %s", id, err)
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

	held := solace.NewHolding("Order history: one line per customer", "customer", "orders", "spent")
	// Called with mu held.
	report := func() {
		ids := make([]string, 0, len(view))
		for id := range view {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		rows := make([][]string, len(ids))
		applied := 0
		for i, id := range ids {
			rows[i] = []string{id, strconv.Itoa(view[id].orders), fmt.Sprintf("%d.00", view[id].spent)}
			applied += view[id].orders
		}
		// The count shows a view that lost events (a restart) even when its
		// latest order matches the write side's.
		note := "Built from no events yet."
		if lastOrder > 0 {
			note = fmt.Sprintf("Built from %d orders, up to order #%d, applied %.1f s after it was placed.", applied, lastOrder, lag.Seconds())
		}
		held.Set(rows, note)
	}
	report()

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
		// Order numbers rise one writer, one queue: one no higher than the
		// last applied is an event the write side published again.
		if order <= lastOrder {
			mu.Unlock()
			solace.Logf(*role, "order #%d again: already applied, ignored", order)
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(*role, "could not acknowledge order #%d: %s", order, err)
			}
			return
		}
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
		report()
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
