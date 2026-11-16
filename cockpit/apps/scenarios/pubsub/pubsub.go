// Package pubsub implements the publish-subscribe scenario.
//
// An online shop publishes an event each time an order is placed. Shipping,
// analytics and email each subscribe to the topics they need, and each gets
// its own copy of every event. The shop does not know who is listening, and
// a new team can start listening without the shop changing anything. The
// events are direct messages: delivered to whoever is subscribed at that
// moment, and kept for no one.
package pubsub

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

var regions = []string{"eu", "us", "apac"}

// Shop publishes an "order placed" event twice a second, numbered, to
// ps/shop/order/placed/v1/<region>/<order>.
func Shop(args []string) {
	fs := flag.NewFlagSet("ps shop", flag.ExitOnError)
	user := fs.String("user", "svc-ps-shop", "client username to connect as")
	role := fs.String("role", "shop", "label for log lines")
	every := fs.Duration("every", 500*time.Millisecond, "time between orders")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "ps-shop")
	defer svc.Disconnect()
	pub := solace.StartDirectPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	solace.Logf(*role, "an order every %s, published direct. The shop does not know who listens.", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(*every)
	defer tick.Stop()
	// Events are numbered from 1 within a run, and carry the run, so a
	// subscriber can count what it missed within one run and never across
	// two.
	run := fmt.Sprintf("%03d", time.Now().Unix()%1000)
	n := 0
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		n++
		region := regions[rand.Intn(len(regions))]
		order := fmt.Sprintf("O-%d", n)
		topic := fmt.Sprintf("ps/shop/order/placed/v1/%s/%s", region, order)
		payload := fmt.Sprintf(`{"run":"%s","event":%d,"order":"%s","region":"%s","total":"%.2f"}`, run, n, order, region, 10+rand.Float64()*190)
		msg, err := svc.MessageBuilder().BuildWithStringPayload(payload)
		if err == nil {
			err = pub.Publish(msg, resource.TopicOf(topic))
		}
		if err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else if n%10 == 0 {
			solace.Logf(*role, "published event #%d (%s)", n, topic)
		}
	}
}

// Subscriber subscribes to one topic and prints each event it receives. It
// remembers the last event number it saw, on disk, so after a restart it can
// say how many were published while it was away.
func Subscriber(args []string) {
	fs := flag.NewFlagSet("ps subscriber", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines, e.g. shipping")
	user := fs.String("user", "", "client username to connect as")
	topic := fs.String("topic", "", "topic subscription")
	clientName := fs.String("client-name", "", "client name the broker reports this app by")
	fs.Parse(args)
	if *role == "" || *user == "" || *topic == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop ps subscriber --role <name> --user <username> --topic <subscription> --client-name <name>")
		os.Exit(2)
	}

	dir := filepath.Join(solace.Env("COCKPIT_STATE_DIR", "."), "publish-subscribe")
	_ = os.MkdirAll(dir, 0o755)
	// "<run> <event>": the last event this subscriber saw, and which run.
	mark := filepath.Join(dir, *role+".last")
	beforeRun, before := "", 0
	if b, err := os.ReadFile(mark); err == nil {
		if f := strings.Fields(string(b)); len(f) == 2 {
			beforeRun = f[0]
			before, _ = strconv.Atoi(f[1])
		}
	}

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.StartDirectReceiver(*role, svc)
	defer solace.Terminate(rcv)
	solace.Subscribe(*role, *user, rcv, *topic)
	fmt.Println()

	first := true
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		payload := solace.PayloadOf(msg)
		n, _ := solace.FieldInt(payload, "event")
		run := solace.FieldString(payload, "run")
		if first {
			first = false
			if run == beforeRun && n > before+1 {
				fmt.Printf("[%s] first event since restarting is #%d; the last one before was #%d: missed #%d to #%d, %d events published while it was away\n",
					*role, n, before, before+1, n-1, n-before-1)
			}
		}
		fmt.Printf("[%s] event #%d: order %s, %s, total %s\n", *role, n,
			solace.FieldString(payload, "order"), solace.FieldString(payload, "region"), solace.FieldString(payload, "total"))
		_ = solace.WriteFileAtomic(mark, []byte(run+" "+strconv.Itoa(n)))
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}
