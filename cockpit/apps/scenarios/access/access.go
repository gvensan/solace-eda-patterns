// Package access implements the access control and authorization scenario.
//
// A bank's payment events. Three apps connect under three usernames, and
// each username's ACL profile says which topics it may publish to and
// subscribe to. The broker enforces it at the moment of publishing or
// subscribing, and says so in the API's error. An app is one role: it
// subscribes to one topic and publishes to another, every second, and
// prints exactly what the broker answers.
package access

import (
	"flag"
	"fmt"
	"os"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// App connects as --user, subscribes to --subscribe (and exits on a refusal,
// printing the broker's error) and, if --publish is given, publishes to it
// once a second, printing any refusal the broker sends back.
func App(args []string) {
	fs := flag.NewFlagSet("ac app", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines")
	user := fs.String("user", "", "client username to connect as")
	clientName := fs.String("client-name", "", "client name the broker reports this app by")
	sub := fs.String("subscribe", "", "topic to subscribe to")
	pubTopic := fs.String("publish", "", "topic to publish to, once a second")
	guaranteed := fs.Bool("guaranteed", false, "publish as guaranteed messages, so the broker's answer comes back on each receipt")
	fs.Parse(args)
	if *role == "" || *user == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop ac app --role <name> --user <username> --client-name <name> [--subscribe <topic>] [--publish <topic>]")
		os.Exit(2)
	}

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	if *sub != "" {
		rcv := solace.StartDirectReceiver(*role, svc)
		defer solace.Terminate(rcv)
		solace.Subscribe(*role, *user, rcv, *sub)
		if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
			fmt.Printf("[%s] received on %s: %s\n", *role, msg.GetDestinationName(), solace.PayloadOf(msg))
		}); err != nil {
			solace.Errf(*role, "could not register the message handler: %s", err)
			os.Exit(1)
		}
	}

	if *pubTopic == "" {
		solace.WaitForStop()
		return
	}
	kind := "direct"
	if *guaranteed {
		kind = "guaranteed"
	}
	solace.Logf(*role, "publishing to %s once a second, %s.", *pubTopic, kind)
	fmt.Println()
	if *guaranteed {
		publishGuaranteed(*role, *user, svc, *pubTopic)
		return
	}
	pub := solace.StartDirectPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for n := 1; ; n++ {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		msg, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"from":"%s","n":%d}`, *role, n))
		if err == nil {
			err = pub.Publish(msg, resource.TopicOf(*pubTopic))
		}
		if err != nil {
			solace.Errf(*role, "publish #%d failed: %s", n, err)
		} else if n%5 == 0 {
			solace.Logf(*role, "published #%d to %s", n, *pubTopic)
		}
	}
}

// publishGuaranteed publishes once a second as guaranteed messages and
// waits for each receipt, which carries the broker's refusal if the topic is
// not allowed. A direct publish to a forbidden topic is dropped by the
// broker without a word to the publisher (docs/spikes.md, section 15).
func publishGuaranteed(role, user string, svc sol.MessagingService, topic string) {
	pub := solace.StartPersistentPublisher(role, user, svc)
	defer solace.Terminate(pub)
	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for n := 1; ; n++ {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		msg, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"from":"%s","n":%d}`, role, n))
		if err == nil {
			err = pub.PublishAwaitAcknowledgement(msg, resource.TopicOf(topic), 5*time.Second, nil)
		}
		if err != nil {
			solace.Errf(role, "publish #%d refused: %s", n, err)
			if why := solace.Explain(err, user, topic); why != "" {
				solace.Errf(role, "%s", why)
			}
		} else if n%5 == 0 {
			solace.Logf(role, "published #%d to %s", n, topic)
		}
	}
}
