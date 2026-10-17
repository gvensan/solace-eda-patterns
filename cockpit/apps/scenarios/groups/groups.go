// Package groups implements the consumer groups scenario.
//
// A product catalogue. Every price change goes to two teams: search, which
// updates the search index, and alerts, which emails customers watching a
// product. Each team needs every price change, and within a team the members
// share the work.
//
// Solace has no consumer group object. Each group is its own queue, every
// queue subscribed to the same topic, so each holds its own copy of every
// change and its own backlog. The group's members share their queue. The
// queues are partitioned by product, so a group sees one product's changes
// in order.
package groups

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

// Catalog publishes price changes for 20 products, keyed by product, each
// with a sequence number per product and the time it was made.
func Catalog(args []string) {
	fs := flag.NewFlagSet("cg catalog", flag.ExitOnError)
	user := fs.String("user", "svc-cg-catalog", "client username to connect as")
	role := fs.String("role", "catalog", "label for log lines")
	rate := fs.Float64("rate", 4, "price changes per second")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "cg-catalog")
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	solace.Logf(*role, "%.0f price changes a second across 20 products, keyed by product.", *rate)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	seq := map[string]int{}
	tick := time.NewTicker(time.Duration(float64(time.Second) / *rate))
	defer tick.Stop()
	for sent := 1; ; sent++ {
		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d price changes.", sent-1)
			return
		case <-tick.C:
		}
		sku := fmt.Sprintf("SKU-%02d", 1+rand.Intn(20))
		seq[sku]++
		payload := fmt.Sprintf(`{"sku":"%s","seq":%d,"price":"%.2f","at":%d}`,
			sku, seq[sku], 5+rand.Float64()*95, time.Now().UnixMilli())
		if err := solace.PublishKeyed(svc, pub, "cg/catalog/price/changed/v1/"+sku, payload, sku); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		}
		if sent%40 == 0 {
			solace.Logf(*role, "%d price changes published", sent)
		}
	}
}

// Member is one member of a group: it handles the price changes its share
// of the group's queue brings it, checks each product's changes arrive in
// order, and says how late a change is when it is more than a few seconds
// old.
func Member(args []string) {
	fs := flag.NewFlagSet("cg member", flag.ExitOnError)
	group := fs.String("group", "", "the group this member belongs to: search or alerts")
	role := fs.String("role", "", "label for log lines, e.g. search-1")
	clientName := fs.String("client-name", "", "client name the broker reports this member by")
	work := fs.Duration("work", 50*time.Millisecond, "time to handle one change")
	fs.Parse(args)
	if *group == "" || *role == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop cg member --group <search|alerts> --role <name> --client-name <name>")
		os.Exit(2)
	}
	user := "svc-cg-" + *group
	queue := "q.cg." + *group

	svc := solace.ConnectAs(*role, user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, user, svc, queue, solace.QueueOpts{ClientAck: true, Shared: true})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "a member of the %s group, sharing %s.", *group, queue)
	fmt.Println()

	var mu sync.Mutex
	last := map[string]int{}
	handled := 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(*work)
		payload := solace.PayloadOf(msg)
		sku := solace.FieldString(payload, "sku")
		seq, _ := solace.FieldInt(payload, "seq")
		at, _ := solace.FieldInt(payload, "at")
		late := time.Since(time.UnixMilli(int64(at)))

		mu.Lock()
		note := ""
		prev, seen := last[sku]
		switch {
		case seen && seq <= prev:
			note = fmt.Sprintf("  OUT OF ORDER: %s #%d after #%d", sku, seq, prev)
		case seen && seq > prev+1:
			// A member only sees its share: the changes in between went to
			// another member of this group, or were lost. It cannot tell which.
			note = fmt.Sprintf("  GAP: %s #%d after #%d (another member had the ones between, or they are lost)", sku, seq, prev)
		}
		if seq > prev {
			last[sku] = seq
		}
		handled++
		mu.Unlock()

		when := ""
		if late > 3*time.Second {
			when = fmt.Sprintf(", late by %.0fs", late.Seconds())
		}
		fmt.Printf("[%s] %s #%d now %s (%s group%s)%s\n", *role, sku, seq,
			solace.FieldString(payload, "price"), *group, when, note)
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
	solace.Logf(*role, "stopped after handling %d price changes.", handled)
}
