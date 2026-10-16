// Package partition implements the partitioning scenario.
//
// Each order moves through created, paid, packed, shipped and delivered, and
// fulfilment must see an order's updates in that order: packing an order
// that has not been paid for is a real mistake. Many orders are in flight at
// once, so one consumer is too slow and the work is spread over two. The
// order id travels as the partition key: the broker sends every update for
// one order to the same partition, and so to the same consumer, in order,
// while different orders are handled in parallel.
package partition

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

// steps is an order's life. A consumer checks each order moves through them
// one at a time.
var steps = []string{"created", "paid", "packed", "shipped", "delivered"}

// Orders publishes status updates for several orders at once, interleaved,
// each keyed by its order id unless --no-key is given.
func Orders(args []string) {
	fs := flag.NewFlagSet("partition orders", flag.ExitOnError)
	user := fs.String("user", "svc-part-orders", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	clientName := fs.String("client-name", "part-orders", "client name the broker reports this app by")
	inFlight := fs.Int("in-flight", 8, "orders in progress at any time")
	rate := fs.Float64("rate", 5, "updates per second")
	noKey := fs.Bool("no-key", false, "publish without a partition key")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	if *noKey {
		solace.Logf(*role, "publishing order updates WITHOUT a partition key.")
	} else {
		solace.Logf(*role, "publishing order updates, keyed by order id.")
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()

	// Order ids carry this run, so a restarted publisher never reuses an id
	// a consumer has already seen finish.
	runTag := time.Now().Unix() % 1000
	next := 0
	newOrder := func() string {
		next++
		return fmt.Sprintf("O-%03d-%04d", runTag, next)
	}
	type progress struct {
		id   string
		step int
	}
	orders := make([]progress, *inFlight)
	for i := range orders {
		orders[i] = progress{id: newOrder()}
	}

	every := time.Duration(float64(time.Second) / *rate)
	sent := 0
	for {
		i := rand.Intn(len(orders))
		o := &orders[i]
		key := o.id
		if *noKey {
			key = ""
		}
		topic := fmt.Sprintf("part/shop/order/%s/v1/%s", steps[o.step], o.id)
		payload := fmt.Sprintf(`{"order":"%s","step":%d,"status":"%s"}`, o.id, o.step+1, steps[o.step])
		if err := solace.PublishKeyed(svc, pub, topic, payload, key); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else {
			sent++
			if sent%10 == 0 {
				solace.Logf(*role, "%d updates published (latest %s %s)", sent, o.id, steps[o.step])
			}
		}
		if o.step++; o.step == len(steps) {
			*o = progress{id: newOrder()}
		}

		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d update(s).", sent)
			return
		case <-time.After(every):
		}
	}
}

// Fulfil consumes order updates from the partitioned queue and checks that
// each order's updates arrive one step at a time.
//
// It can only see its own share. An order it first meets part way through
// was handled elsewhere until now (it took the order over); an order whose
// updates skip a step had that step handled by another consumer, which is
// what happens when updates are not keyed.
func Fulfil(args []string) {
	fs := flag.NewFlagSet("partition fulfil", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines, e.g. fulfilment-1")
	user := fs.String("user", "svc-part-fulfilment", "client username to connect as")
	clientName := fs.String("client-name", "", "client name the broker reports this consumer by")
	queue := fs.String("queue", "q.part.orders", "partitioned queue to consume")
	work := fs.Duration("work", 100*time.Millisecond, "time to handle one update")
	fs.Parse(args)

	if *role == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop partition fulfil --role <role> --client-name <name>")
		os.Exit(2)
	}

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true, Shared: true})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "handling order updates from %s.", *queue)
	fmt.Println()

	var mu sync.Mutex
	last := map[string]int{}
	handled, splits, disorders := 0, 0, 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(*work)
		payload := solace.PayloadOf(msg)
		order := solace.FieldString(payload, "order")
		step, _ := solace.FieldInt(payload, "step")
		status := solace.FieldString(payload, "status")
		key, keyed := solace.MessageKey(msg)

		mu.Lock()
		prev, seen := last[order]
		note := ""
		switch {
		case !seen && step > 1:
			note = fmt.Sprintf("  picked up %s at %s (its earlier updates went elsewhere)", order, status)
		case seen && step <= prev:
			disorders++
			note = fmt.Sprintf("  OUT OF ORDER: %s %s after %s", order, status, steps[prev-1])
		case seen && step > prev+1:
			splits++
			note = fmt.Sprintf("  SPLIT: %s went %s to %s; %s was handled by another consumer",
				order, steps[prev-1], status, strings.Join(steps[prev:step-1], ", "))
		}
		last[order] = step
		if step == len(steps) {
			delete(last, order)
		}
		handled++
		mu.Unlock()

		keyText := "no key"
		if keyed {
			keyText = "key " + key
		}
		line := fmt.Sprintf("[%s] %s %s (%s)", *role, order, status, keyText)
		if msg.IsRedelivered() {
			line += "  (redelivered)"
		}
		fmt.Println(line + note)
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	mu.Lock()
	defer mu.Unlock()
	fmt.Println()
	solace.Logf(*role, "stopped after %d update(s): %d split, %d out of order.", handled, splits, disorders)
}
