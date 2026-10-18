// Package governance implements the intermediated and disintermediated
// governance scenarios.
//
// Both are about who decides what an application may publish and consume.
// Intermediated: a central team grants each request, and the broker enforces
// the grant. Disintermediated: each team owns a part of the topic space and
// acts there without asking, inside guardrails the broker enforces. The apps
// here are ordinary publishers and consumers; what differs is how their
// usernames' permissions came about.
package governance

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// Sales publishes the sales team's events, orders and payments, as direct
// messages. Who may subscribe to them is the central team's decision.
func Sales(args []string) {
	fs := flag.NewFlagSet("ig sales", flag.ExitOnError)
	user := fs.String("user", "svc-ig-sales", "client username to connect as")
	role := fs.String("role", "sales", "label for log lines")
	clientName := fs.String("client-name", "ig-sales", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between events")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartDirectPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	solace.Logf(*role, "publishing orders on ig/sales/order/... and payments on ig/sales/payment/...")
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	base := int(time.Now().Unix()%10000) * 10
	for n := 1; ; n++ {
		id := fmt.Sprintf("O-%05d", base+n)
		topic := "ig/sales/order/created/v1/" + id
		body := fmt.Sprintf(`{"order":"%s","items":%d}`, id, 1+rand.Intn(5))
		if n%2 == 0 {
			topic = "ig/sales/payment/taken/v1/" + id
			body = fmt.Sprintf(`{"order":"%s","card":"**** %04d","amount":%d}`, id, rand.Intn(10000), 10+rand.Intn(90))
		}
		msg, err := svc.MessageBuilder().BuildWithStringPayload(body)
		if err == nil {
			err = pub.Publish(msg, resource.TopicOf(topic))
		}
		if err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else if n%10 == 0 {
			solace.Logf(*role, "published %d events", n)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Team is one team's app in the disintermediated scenario. It publishes
// under --publish (guaranteed, so a refusal comes back on the receipt),
// and/or keeps its own queue, which it creates itself if it does not exist:
// self-service, no ticket.
func Team(args []string) {
	fs := flag.NewFlagSet("dg team", flag.ExitOnError)
	team := fs.String("team", "", "the team this app belongs to")
	role := fs.String("role", "", "label for log lines (default the team)")
	clientName := fs.String("client-name", "", "client name the broker reports this app by")
	publish := fs.String("publish", "", "topic prefix to publish under, once a second")
	queue := fs.String("queue", "", "a queue this app creates for itself and reads")
	subscribe := fs.String("subscribe", "", "topic subscription for that queue")
	fs.Parse(args)
	if *team == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop dg team --team <team> --client-name <name> [--publish <prefix>] [--queue <name> --subscribe <topic>]")
		os.Exit(2)
	}
	if *role == "" {
		*role = *team
	}
	user := "svc-dg-" + *team

	svc := solace.ConnectAs(*role, user, *clientName)
	defer svc.Disconnect()

	if *queue != "" {
		rcv, err := svc.CreatePersistentMessageReceiverBuilder().
			WithMissingResourcesCreationStrategy(config.PersistentReceiverCreateOnStartMissingResources).
			WithMessageClientAcknowledgement().
			Build(resource.QueueDurableExclusive(*queue))
		if err == nil {
			err = rcv.Start()
		}
		if err != nil {
			solace.Errf(*role, "could not create or bind queue %s:", *queue)
			solace.Errf(*role, "%s", err)
			os.Exit(1)
		}
		defer solace.Terminate(rcv)
		if *subscribe != "" {
			if err := rcv.AddSubscription(resource.TopicSubscriptionOf(*subscribe)); err != nil {
				solace.Errf(*role, "could not subscribe queue %s to %s:", *queue, *subscribe)
				solace.Errf(*role, "%s", err)
				os.Exit(1)
			}
		}
		solace.Logf(*role, "queue %s is ours (created by this team, no request needed), subscribed to %s", *queue, *subscribe)
		if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
			p := solace.PayloadOf(msg)
			solace.Logf(*role, "received %s from %s", solace.FieldString(p, "order"), msg.GetDestinationName())
			_ = rcv.Ack(msg)
		}); err != nil {
			solace.Errf(*role, "could not register the message handler: %s", err)
			os.Exit(1)
		}
	}

	if *publish == "" {
		fmt.Println()
		solace.WaitForStop()
		return
	}
	pub := solace.StartPersistentPublisher(*role, user, svc)
	defer solace.Terminate(pub)
	prefix := strings.TrimSuffix(*publish, "/")
	solace.Logf(*role, "publishing under %s/", prefix)
	fmt.Println()
	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	base := int(time.Now().Unix()%10000) * 10
	for n := 1; ; n++ {
		id := fmt.Sprintf("O-%05d", base+n)
		topic := prefix + "/" + id
		msg, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"order":"%s","team":"%s"}`, id, *team))
		if err == nil {
			err = pub.PublishAwaitAcknowledgement(msg, resource.TopicOf(topic), 5*time.Second, nil)
		}
		if err != nil {
			solace.Errf(*role, "%s refused: %s", topic, err)
		} else {
			solace.Logf(*role, "published %s", topic)
		}
		select {
		case <-stop:
			return
		case <-time.After(time.Second):
		}
	}
}
