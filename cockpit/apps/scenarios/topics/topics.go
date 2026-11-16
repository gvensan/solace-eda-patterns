// Package topics implements the hierarchical topics scenario.
//
// A retail chain's stores publish every sale on a topic that says where it
// happened and what was sold, level by level:
//
//	ht/retail/<region>/<store>/<department>/sale/v1/<sale>
//
// Each team subscribes with wildcards to the slice it needs: a region, a
// department across every store, or everything. The broker matches topics
// to subscriptions; no team filters what it receives, and the stores do not
// know who listens. The levels are a contract: a subscription that miscounts
// them, or a publisher that adds one, quietly changes who gets what.
package topics

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

var (
	regions     = []string{"eu", "us", "apac"}
	departments = []string{"electronics", "grocery", "clothing", "garden"}
)

// Stores publishes sales from stores across the chain. --store and
// --region fix one store; --extra-level puts one more level after the
// store, as a new till system might.
func Stores(args []string) {
	fs := flag.NewFlagSet("ht stores", flag.ExitOnError)
	user := fs.String("user", "svc-ht-stores", "client username to connect as")
	role := fs.String("role", "stores", "label for log lines")
	clientName := fs.String("client-name", "ht-stores", "client name the broker reports this app by")
	every := fs.Duration("every", 250*time.Millisecond, "time between sales")
	store := fs.String("store", "", "publish for this one store only")
	region := fs.String("region", "", "the region of --store")
	extra := fs.String("extra-level", "", "an extra topic level after the store")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartDirectPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	shape := "ht/retail/<region>/<store>/<department>/sale/v1/<sale>"
	if *extra != "" {
		shape = "ht/retail/<region>/<store>/" + *extra + "/<department>/sale/v1/<sale>"
	}
	solace.Logf(*role, "a sale every %s, published direct to %s", *every, shape)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(*every)
	defer tick.Stop()
	for n := 1; ; n++ {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		r, s := *region, *store
		if s == "" {
			// Each store is in one region: 001 to 004 eu, 005 to 008 us,
			// 009 to 012 apac.
			i := rand.Intn(12)
			r = regions[i/4]
			s = fmt.Sprintf("store-%03d", i+1)
		}
		levels := r + "/" + s
		if *extra != "" {
			levels += "/" + *extra
		}
		dept := departments[rand.Intn(len(departments))]
		topic := fmt.Sprintf("ht/retail/%s/%s/sale/v1/S-%d", levels, dept, n)
		msg, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"sale":%d,"amount":"%.2f"}`, n, 2+rand.Float64()*300))
		if err == nil {
			err = pub.Publish(msg, resource.TopicOf(topic))
		}
		if err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else if n%20 == 0 {
			solace.Logf(*role, "%d sales published, the latest to %s", n, topic)
		}
	}
}

// Team subscribes with one subscription and prints the topic of each sale
// it receives: what the broker matched for it.
func Team(args []string) {
	fs := flag.NewFlagSet("ht team", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines, e.g. electronics")
	user := fs.String("user", "svc-ht-teams", "client username to connect as")
	topic := fs.String("topic", "", "topic subscription, with wildcards")
	clientName := fs.String("client-name", "", "client name the broker reports this app by")
	fs.Parse(args)
	if *role == "" || *topic == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop ht team --role <name> --topic <subscription> --client-name <name>")
		os.Exit(2)
	}

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.StartDirectReceiver(*role, svc)
	defer solace.Terminate(rcv)
	solace.Subscribe(*role, *user, rcv, *topic)
	fmt.Println()

	var n atomic.Int64
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		n.Add(1)
		fmt.Printf("[%s] got %s\n", *role, msg.GetDestinationName())
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	// While nothing matches, say so now and then, so silence is not mistaken
	// for a stopped app.
	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	var seen int64
	for {
		select {
		case <-stop:
			solace.Logf(*role, "stopped after %d sales.", n.Load())
			return
		case <-tick.C:
		}
		if now := n.Load(); now == seen {
			solace.Logf(*role, "nothing has matched %s in the last 5 s", *topic)
		} else {
			seen = now
		}
	}
}
