// Package filter implements the event filtering scenario.
//
// A factory's sensors publish readings on topics that name the site, the
// line and the kind of sensor:
//
//	ef/plant/<site>/<line>/<kind>/reading/v1
//
// Each reading also has a severity from 1 to 5, as a message property and in
// the payload, but not in the topic. A consumer that filters on what is in
// the topic does it with a subscription, and the broker sends it nothing
// else. A consumer that filters on something only in the message uses a
// selector, which the broker evaluates for each message on the queue, and
// what it does not match stays there.
package filter

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

var (
	sites = []string{"berlin", "lyon"}
	lines = []string{"line1", "line2"}
	kinds = []string{"temp", "vibration"}
)

// Sensors publishes --rate readings a second from random sensors. About one
// reading in twelve has severity 4 or 5.
func Sensors(args []string) {
	fs := flag.NewFlagSet("ef sensors", flag.ExitOnError)
	user := fs.String("user", "svc-ef-sensors", "client username to connect as")
	role := fs.String("role", "sensors", "label for log lines")
	clientName := fs.String("client-name", "ef-sensors", "client name the broker reports this app by")
	rate := fs.Int("rate", 5, "readings a second")
	pad := fs.Int("pad", 0, "extra bytes of waveform in each reading")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	wave := strings.Repeat("~", *pad)
	if *pad > 0 {
		solace.Logf(*role, "high resolution: %d readings a second, %d KB each, to ef/plant/<site>/<line>/<kind>/reading/v1", *rate, *pad/1000)
	} else {
		solace.Logf(*role, "publishing %d readings a second to ef/plant/<site>/<line>/<kind>/reading/v1", *rate)
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(time.Second / time.Duration(*rate))
	defer tick.Stop()
	refused := 0
	for n := 1; ; n++ {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		site, line, kind := sites[rand.Intn(2)], lines[rand.Intn(2)], kinds[rand.Intn(2)]
		sev := 1 + rand.Intn(3)
		if rand.Intn(12) == 0 {
			sev = 4 + rand.Intn(2)
		}
		topic := fmt.Sprintf("ef/plant/%s/%s/%s/reading/v1", site, line, kind)
		payload := fmt.Sprintf(`{"reading":%d,"site":"%s","line":"%s","kind":"%s","severity":%d,"value":%d,"wave":"%s"}`,
			n, site, line, kind, sev, 20+rand.Intn(60), wave)
		msg, err := svc.MessageBuilder().WithProperty("severity", int32(sev)).BuildWithStringPayload(payload)
		if err == nil {
			err = pub.PublishAwaitAcknowledgement(msg, resource.TopicOf(topic), 5*time.Second, nil)
		}
		if err != nil {
			refused++
			if refused <= 3 || refused%25 == 0 {
				solace.Errf(*role, "reading #%d (%s, severity %d) refused: %s", n, topic, sev, err)
			}
			continue
		}
		if n%(5**rate) == 0 {
			solace.Logf(*role, "published reading #%d", n)
		}
	}
}

// Watch binds a queue, optionally with a selector, and prints each message
// it is given. Every 10 seconds it says how many it got and what they had in
// common, so a filter that lets the wrong thing through shows.
func Watch(args []string) {
	fs := flag.NewFlagSet("ef watch", flag.ExitOnError)
	user := fs.String("user", "", "client username to connect as")
	role := fs.String("role", "", "label for log lines")
	clientName := fs.String("client-name", "", "client name the broker reports this app by")
	queue := fs.String("queue", "", "queue to bind")
	selector := fs.String("selector", "", "message selector, evaluated by the broker for each message on the queue")
	fs.Parse(args)
	if *user == "" || *role == "" || *clientName == "" || *queue == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop ef watch --user <u> --role <r> --client-name <c> --queue <q> [--selector <s>]")
		os.Exit(2)
	}

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	b := svc.CreatePersistentMessageReceiverBuilder().
		WithMissingResourcesCreationStrategy(config.PersistentReceiverDoNotCreateMissingResources).
		WithMessageClientAcknowledgement()
	if *selector != "" {
		b = b.WithMessageSelector(*selector)
	}
	rcv, err := b.Build(resource.QueueDurableExclusive(*queue))
	if err == nil {
		err = rcv.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not bind to queue %s: %s", *queue, err)
		os.Exit(1)
	}
	defer solace.Terminate(rcv)
	if *selector != "" {
		solace.Logf(*role, "bound to queue %s with selector: %s", *queue, *selector)
	} else {
		solace.Logf(*role, "bound to queue %s, no selector", *queue)
	}
	fmt.Println()

	var mu sync.Mutex
	got, seen := 0, map[string]bool{}
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		n, _ := solace.FieldInt(p, "reading")
		sev, _ := solace.FieldInt(p, "severity")
		t := msg.GetDestinationName()
		parts := strings.Split(t, "/")
		label := t
		if len(parts) >= 5 {
			label = parts[2] + " " + parts[3] + " " + parts[4]
		}
		mu.Lock()
		got++
		seen[label] = true
		mu.Unlock()
		solace.Logf(*role, "reading #%d: %s, severity %d", n, label, sev)
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for {
		select {
		case <-stop:
			return
		case <-time.After(10 * time.Second):
		}
		mu.Lock()
		kinds := make([]string, 0, len(seen))
		for k := range seen {
			kinds = append(kinds, k)
		}
		solace.Logf(*role, "last 10 s: %d readings, from %d sensor groups", got, len(kinds))
		got, seen = 0, map[string]bool{}
		mu.Unlock()
	}
}
