// Package catalog implements the event catalog and event API scenarios.
//
// Both rest on one idea: events described in a document that people and
// tools can read, here an AsyncAPI document in JSON (AsyncAPI is the open
// specification Solace Event Portal exports). The catalog checker compares
// what actually flows with what the document describes. The event API
// consumers are written against one version of an API and expect the
// payload that version promises.
package catalog

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

// ------------------------------------------------------------ AsyncAPI

type schema struct {
	Required []string `json:"required"`
}

type channel struct {
	Description string `json:"description"`
	Publish     struct {
		Message struct {
			Name    string `json:"name"`
			Payload schema `json:"payload"`
		} `json:"message"`
	} `json:"publish"`
}

type document struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Channels map[string]channel `json:"channels"`
}

func load(path string) (document, error) {
	var d document
	b, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(b, &d)
	}
	return d, err
}

// match finds the channel a topic belongs to: a {parameter} level matches
// any one level.
func (d document) match(topic string) (string, channel, bool) {
	levels := strings.Split(topic, "/")
	for name, ch := range d.Channels {
		parts := strings.Split(name, "/")
		if len(parts) != len(levels) {
			continue
		}
		ok := true
		for i, p := range parts {
			if !(strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}")) && p != levels[i] {
				ok = false
				break
			}
		}
		if ok {
			return name, ch, true
		}
	}
	return "", channel{}, false
}

func missing(payload string, ch channel) []string {
	var fields map[string]any
	if json.Unmarshal([]byte(payload), &fields) != nil {
		return []string{"(not JSON)"}
	}
	var out []string
	for _, f := range ch.Publish.Message.Payload.Required {
		if _, ok := fields[f]; !ok {
			out = append(out, f)
		}
	}
	return out
}

// ------------------------------------------------------- event catalog

// Shop publishes the shop's events. --cancellations adds an event type the
// catalog does not have; --rename sends the payment's amount as "total".
func Shop(args []string) {
	fs := flag.NewFlagSet("ec shop", flag.ExitOnError)
	team := fs.String("team", "orders", "orders or payments")
	user := fs.String("user", "", "client username (default svc-ec-<team>)")
	clientName := fs.String("client-name", "", "client name (default ec-<team>)")
	cancellations := fs.Bool("cancellations", false, "also publish order cancelled events")
	rename := fs.Bool("rename", false, "send the payment's amount as total")
	every := fs.Duration("every", time.Second, "pause between events")
	fs.Parse(args)
	if *user == "" {
		*user = "svc-ec-" + *team
	}
	if *clientName == "" {
		*clientName = "ec-" + *team
	}
	role := *team
	svc := solace.ConnectAs(role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(role, *user, svc)
	defer solace.Terminate(pub)
	switch {
	case *cancellations:
		solace.Logf(role, "new release: also publishing order cancelled events")
	case *rename:
		solace.Logf(role, "new release: the payment's amount is now called total")
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	base := int(time.Now().Unix()%10000) * 10
	for n := 1; ; n++ {
		order := fmt.Sprintf("O-%05d", base+n)
		amount := 10 + rand.Intn(90)
		var topic, payload string
		switch {
		case *team == "payments" && *rename:
			topic, payload = "ec/shop/payment/taken/v1/"+order, fmt.Sprintf(`{"order":"%s","total":%d}`, order, amount)
		case *team == "payments":
			topic, payload = "ec/shop/payment/taken/v1/"+order, fmt.Sprintf(`{"order":"%s","amount":%d}`, order, amount)
		case *cancellations && n%3 == 0:
			topic, payload = "ec/shop/order/cancelled/v1/"+order, fmt.Sprintf(`{"order":"%s","reason":"customer"}`, order)
		case n%2 == 0:
			topic, payload = "ec/shop/order/shipped/v1/"+order, fmt.Sprintf(`{"order":"%s","carrier":"DPD"}`, order)
		default:
			topic, payload = "ec/shop/order/created/v1/"+order, fmt.Sprintf(`{"order":"%s","amount":%d}`, order, amount)
		}
		if err := solace.PublishConfirmed(svc, pub, topic, payload); err != nil {
			solace.Errf(role, "%s refused: %s", topic, err)
		} else {
			solace.Logf(role, "published %s", topic)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Checker subscribes to everything under --topic and compares each event
// with the catalog. It reports each kind of finding once a minute, not once
// per event, and a summary every 10 seconds.
func Checker(args []string) {
	fs := flag.NewFlagSet("ec checker", flag.ExitOnError)
	user := fs.String("user", "svc-ec-checker", "client username to connect as")
	role := fs.String("role", "checker", "label for log lines")
	clientName := fs.String("client-name", "ec-checker", "client name the broker reports this app by")
	catalogPath := fs.String("catalog", "catalog.json", "AsyncAPI document")
	topic := fs.String("topic", "ec/>", "what to watch")
	fs.Parse(args)

	doc, err := load(*catalogPath)
	if err != nil {
		solace.Errf(*role, "could not read the catalog: %s", err)
		os.Exit(1)
	}
	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.StartDirectReceiver(*role, svc)
	defer solace.Terminate(rcv)
	solace.Subscribe(*role, *user, rcv, *topic)
	solace.Logf(*role, "catalog %q %s: %d event types documented", doc.Info.Title, doc.Info.Version, len(doc.Channels))
	fmt.Println()

	var mu sync.Mutex
	seen := map[string]bool{}
	said := map[string]time.Time{}
	say := func(key, line string, bad bool) {
		if time.Since(said[key]) < time.Minute {
			return
		}
		said[key] = time.Now()
		if bad {
			solace.Errf(*role, "%s", line)
		} else {
			solace.Logf(*role, "%s", line)
		}
	}
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		t := msg.GetDestinationName()
		mu.Lock()
		defer mu.Unlock()
		name, ch, ok := doc.match(t)
		if !ok {
			kind := t[:strings.LastIndexByte(t, '/')] + "/..."
			seen["UNDOCUMENTED "+kind] = true
			say(kind, fmt.Sprintf("undocumented: %s is not in the catalog", t), true)
			return
		}
		seen[name] = true
		if m := missing(solace.PayloadOf(msg), ch); len(m) > 0 {
			say("payload "+name, fmt.Sprintf("does not match the catalog: %s is missing %s", t, strings.Join(m, ", ")), true)
			return
		}
		say("ok "+name, fmt.Sprintf("in the catalog: %s (%s)", t, ch.Publish.Message.Name), false)
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
		documented, undocumented := 0, []string{}
		for k := range seen {
			if strings.HasPrefix(k, "UNDOCUMENTED ") {
				undocumented = append(undocumented, strings.TrimPrefix(k, "UNDOCUMENTED "))
			} else {
				documented++
			}
		}
		seen = map[string]bool{}
		mu.Unlock()
		sort.Strings(undocumented)
		if len(undocumented) == 0 {
			solace.Logf(*role, "last 10 s: %d event types seen, all in the catalog", documented)
		} else {
			solace.Errf(*role, "last 10 s: %d documented event types, %d not in the catalog (%s)", documented, len(undocumented), strings.Join(undocumented, ", "))
		}
	}
}

// Finance books each payment, reading the amount field the catalog
// promises.
func Finance(args []string) {
	fs := flag.NewFlagSet("ec finance", flag.ExitOnError)
	user := fs.String("user", "svc-ec-finance", "client username to connect as")
	role := fs.String("role", "finance", "label for log lines")
	clientName := fs.String("client-name", "ec-finance", "client name the broker reports this app by")
	queue := fs.String("queue", "q.ec.finance", "queue to read")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		order := solace.FieldString(p, "order")
		if amount, ok := solace.FieldInt(p, "amount"); ok {
			solace.Logf(*role, "%s: booked %d.00", order, amount)
		} else {
			solace.Errf(*role, "%s: payment has no amount field, cannot book it", order)
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

// ----------------------------------------------------------- event APIs

// Shipping implements the shipping event API. --versions says which
// versions it publishes; --break sends v1 events in the v2 shape.
func Shipping(args []string) {
	fs := flag.NewFlagSet("ea shipping", flag.ExitOnError)
	user := fs.String("user", "svc-ea-shipping", "client username to connect as")
	role := fs.String("role", "shipping", "label for log lines")
	clientName := fs.String("client-name", "ea-shipping", "client name the broker reports this app by")
	versions := fs.String("versions", "v1,v2", "versions to publish")
	broken := fs.Bool("break", false, "publish v1 events in the v2 payload shape")
	every := fs.Duration("every", 1500*time.Millisecond, "pause between parcels")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	v1, v2 := strings.Contains(*versions, "v1"), strings.Contains(*versions, "v2")
	switch {
	case *broken:
		solace.Logf(*role, "new release: v1 events now carry the weight as {value, unit}, under the same v1 topic")
	case v1 && v2:
		solace.Logf(*role, "publishing every dispatched parcel on API v1 and v2")
	case v2:
		solace.Logf(*role, "v1 retired: publishing on API v2 only")
	default:
		solace.Logf(*role, "publishing every dispatched parcel on API v1")
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	base := int(time.Now().Unix()%10000) * 10
	for n := 1; ; n++ {
		parcel := fmt.Sprintf("P-%05d", base+n)
		grams := 200 + rand.Intn(4800)
		newShape := fmt.Sprintf(`{"parcel":"%s","weight":{"value":%d,"unit":"g"}}`, parcel, grams)
		oldShape := fmt.Sprintf(`{"parcel":"%s","weightKg":%.1f}`, parcel, float64(grams)/1000)
		if v1 {
			body := oldShape
			if *broken {
				body = newShape
			}
			if err := solace.PublishConfirmed(svc, pub, "ea/shipping/parcel/dispatched/v1/"+parcel, body); err != nil {
				solace.Errf(*role, "%s v1 refused: %s", parcel, err)
			}
		}
		if v2 {
			if err := solace.PublishConfirmed(svc, pub, "ea/shipping/parcel/dispatched/v2/"+parcel, newShape); err != nil {
				solace.Errf(*role, "%s v2 refused: %s", parcel, err)
			}
		}
		solace.Logf(*role, "%s dispatched, published on %s", parcel, *versions)
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Consumer is written against one version of the shipping API and reads the
// field that version promises. It says when nothing has arrived for a while,
// because a retired topic is silent, not an error.
func Consumer(args []string) {
	fs := flag.NewFlagSet("ea consumer", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines")
	user := fs.String("user", "", "client username to connect as")
	clientName := fs.String("client-name", "", "client name the broker reports this app by")
	queue := fs.String("queue", "", "queue to read")
	version := fs.String("version", "v1", "API version this consumer was written for")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	solace.Logf(*role, "written against shipping API %s", *version)
	fmt.Println()

	var mu sync.Mutex
	last := time.Now()
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		parcel := solace.FieldString(p, "parcel")
		mu.Lock()
		last = time.Now()
		mu.Unlock()
		var body map[string]any
		_ = json.Unmarshal([]byte(p), &body)
		if *version == "v1" {
			if kg, ok := body["weightKg"].(float64); ok {
				solace.Logf(*role, "%s: label printed, %.1f kg", parcel, kg)
			} else {
				solace.Errf(*role, "%s: payload has no weightKg, which API v1 promises: cannot print the label", parcel)
			}
		} else {
			weight, _ := body["weight"].(map[string]any)
			if g, ok := weight["value"].(float64); ok {
				solace.Logf(*role, "%s: tracking updated, %.0f g", parcel, g)
			} else {
				solace.Errf(*role, "%s: payload has no weight.value, which API v2 promises", parcel)
			}
		}
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
		case <-time.After(5 * time.Second):
		}
		mu.Lock()
		idle := time.Since(last)
		mu.Unlock()
		if idle > 8*time.Second {
			solace.Logf(*role, "nothing received on API %s for %.0f s", *version, idle.Seconds())
		}
	}
}
