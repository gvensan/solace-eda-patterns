// Package partition implements the partitioning scenario.
//
// Each order moves through created, paid, packed, shipped and delivered, and
// fulfilment must see an order's updates in that order: packing an order
// that has not been paid for is a real mistake. Many orders are in flight at
// once, so one consumer is too slow and the work is spread over three. The
// order id travels as the partition key: the broker sends every update for
// one order to the same partition, and so to the same consumer, in order,
// while different orders are handled in parallel.
//
// Every update carries a per-order sequence number (#1 to #5), and the
// consumer reports it, so a gap or a step backwards shows in its log.
package partition

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

// steps is an order's life. Update #n carries steps[n-1].
var steps = []string{"created", "paid", "packed", "shipped", "delivered"}

// measured is the partition that order id O-0001, O-0002 and so on up to
// O-1200 lands on in a queue of six partitions, one digit per id. The broker
// hashes the key to pick a partition and does not document the hash, so
// this was measured on broker 10.26.0 (docs/spikes.md, section 10). It only
// steers which ids the order service picks, so that every partition has
// orders in flight; the broker still does the placing. On a broker that
// hashes differently, some partitions just get less traffic.
const measured = "" +
	"10304414354553435115011210544402451052512135435110433233155150052325155333442132" +
	"51333250111033254150454103530201255504312514133024402250052314410340425012251033" +
	"01543234431530350233124341543421431102223453312354051503523534403055513210401014" +
	"15255234213305025443531522224340350051554051510115040423035115035450240425105330" +
	"22131512534235203101140510415354512225505555005040305050301335513141251114511144" +
	"14215131101332451045044521523305514214512045302141003211122450112311555343333502" +
	"41442013512515443123001334202510002100304513050050132335010354353112204424322022" +
	"24555541000545340450244234540042011345202305254355510553522425022220143255450553" +
	"45405501445211515034412345154354345010121233312504513204450451010524540052521113" +
	"22104130232211422140011322534235144115322331140234201234000000105221341335410354" +
	"52513025310114143031231305335051454334143455043403432211214532003102214152102243" +
	"03002004201152333440312450040400340344044343535532245120140210501003232322145423" +
	"02251201222142340205422153422000222243222545343342551354430550323452304055510101" +
	"52353354525454123154545250230413133030145314444353153404530323402133042201035141" +
	"22134401554405152301100143224442232015535555530421502444345340413434104352242045"

// idsFor lists the order numbers that land on each of the six partitions.
func idsFor() [6][]int {
	var by [6][]int
	for i, c := range measured {
		by[c-'0'] = append(by[c-'0'], i+1)
	}
	return by
}

// Orders publishes status updates for several orders at once, interleaved,
// each keyed by its order id unless --no-key is given.
func Orders(args []string) {
	fs := flag.NewFlagSet("partition orders", flag.ExitOnError)
	user := fs.String("user", "svc-part-orders", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	clientName := fs.String("client-name", "part-orders", "client name the broker reports this app by")
	inFlight := fs.Int("in-flight", 6, "orders in progress at any time")
	rate := fs.Float64("rate", 2, "updates per second")
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

	// Order k in flight always takes its next id from the ids measured to
	// land on partition k mod 6, so all six partitions stay busy: one order
	// each with six in flight, one or two each with ten. Each run
	// starts at a random point in the list, so a restarted order service
	// does not reuse the ids a consumer saw a moment ago.
	by := idsFor()
	cursor := [6]int{}
	for p := range cursor {
		cursor[p] = rand.Intn(len(by[p]))
	}
	newOrder := func(slot int) string {
		p := slot % 6
		id := by[p][cursor[p]%len(by[p])]
		cursor[p]++
		return fmt.Sprintf("O-%04d", id)
	}
	type progress struct {
		id  string
		seq int // the update to send next, from 1
	}
	orders := make([]progress, *inFlight)
	for i := range orders {
		orders[i] = progress{id: newOrder(i), seq: 1}
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
		status := steps[o.seq-1]
		topic := fmt.Sprintf("part/shop/order/%s/v1/%s", status, o.id)
		payload := fmt.Sprintf(`{"order":"%s","seq":%d,"status":"%s"}`, o.id, o.seq, status)
		if err := solace.PublishKeyed(svc, pub, topic, payload, key); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else {
			sent++
			if sent%10 == 0 {
				solace.Logf(*role, "%d updates published (latest %s #%d %s)", sent, o.id, o.seq, status)
			}
		}
		if o.seq++; o.seq > len(steps) {
			*o = progress{id: newOrder(i), seq: 1}
		}

		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d update(s).", sent)
			return
		case <-time.After(every):
		}
	}
}

// Fulfil consumes order updates from the partitioned queue and checks each
// order's sequence numbers arrive one at a time.
//
// It can only see its own share. An order it first meets part way through
// was handled elsewhere until now (it picked the order up); an order whose
// sequence skips a number had that update handled by another consumer,
// which is what happens when updates are not keyed.
//
// Each line also gives the broker's number for the message (msg), which is
// what the dashboard matches against the partitions' message lists to draw
// which partition the update came through.
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
	handled, gaps, disorders := 0, 0, 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(*work)
		payload := solace.PayloadOf(msg)
		order := solace.FieldString(payload, "order")
		seq, _ := solace.FieldInt(payload, "seq")
		status := solace.FieldString(payload, "status")
		key, keyed := solace.MessageKey(msg)

		mu.Lock()
		prev, seen := last[order]
		note := ""
		switch {
		case seq == 1:
			// A new order. Ids are reused after a long while, never in flight.
		case !seen:
			note = fmt.Sprintf("  picked up %s at #%d (its earlier updates went to another consumer)", order, seq)
		case seq <= prev:
			disorders++
			note = fmt.Sprintf("  OUT OF ORDER: %s #%d after #%d", order, seq, prev)
		case seq > prev+1:
			gaps++
			missed := fmt.Sprintf("#%d", prev+1)
			if seq > prev+2 {
				missed = fmt.Sprintf("#%d to #%d", prev+1, seq-1)
			}
			note = fmt.Sprintf("  GAP: %s #%d then #%d (%s went to another consumer)", order, prev, seq, missed)
		}
		last[order] = seq
		if seq == len(steps) {
			delete(last, order)
		}
		handled++
		mu.Unlock()

		keyText := "no key"
		if keyed {
			keyText = "key " + key
		}
		if id, ok := solace.SpoolID(msg); ok {
			keyText += fmt.Sprintf(", msg %d", id)
		}
		line := fmt.Sprintf("[%s] %s #%d %s (%s)", *role, order, seq, status, keyText)
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
	solace.Logf(*role, "stopped after %d update(s): %d gap(s), %d out of order.", handled, gaps, disorders)
}
