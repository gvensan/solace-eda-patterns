// Package reqreply implements the request-reply scenario.
//
// A till needs to know the price of an item before it can ring it up, so it
// asks and waits for the answer: a request, and a reply addressed back to
// the asker. The pricing service answers each request. In the Go API,
// request-reply runs on direct messages only: the request is delivered to
// whoever is subscribed to the request topic at that moment, and if no reply
// comes within the timeout the till is told so.
package reqreply

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// Till asks the price of a random item once a second and logs the reply, or
// the error if none comes within --timeout.
func Till(args []string) {
	fs := flag.NewFlagSet("rr till", flag.ExitOnError)
	user := fs.String("user", "svc-rr-tills", "client username to connect as")
	role := fs.String("role", "till", "label for log lines")
	clientName := fs.String("client-name", "rr-till", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "time between requests")
	timeout := fs.Duration("timeout", 2*time.Second, "how long to wait for a reply")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	// Request-reply builders live on the service's RequestReply() view.
	rr := svc.RequestReply()
	pub, err := rr.CreateRequestReplyMessagePublisherBuilder().Build()
	if err == nil {
		err = pub.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start the requester: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(pub)

	solace.Logf(*role, "asking a price every %s, waiting up to %s for each reply.", *every, *timeout)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(*every)
	defer tick.Stop()
	var mu sync.Mutex
	answered, unanswered := 0, 0
	for n := 1; ; n++ {
		select {
		case <-stop:
			mu.Lock()
			solace.Logf(*role, "stopped: %d requests answered, %d unanswered.", answered, unanswered)
			mu.Unlock()
			return
		case <-tick.C:
		}
		sku := fmt.Sprintf("SKU-%02d", 1+rand.Intn(20))
		msg, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"request":%d,"sku":"%s"}`, n, sku))
		if err != nil {
			solace.Errf(*role, "could not build request #%d: %s", n, err)
			continue
		}
		sent := time.Now()
		err = pub.Publish(msg, func(reply message.InboundMessage, ctx interface{}, err error) {
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				unanswered++
				solace.Errf(*role, "request #%d for %s: %s", n, sku, err)
				return
			}
			answered++
			payload := solace.PayloadOf(reply)
			fmt.Printf("[%s] request #%d: %s costs %s (answered by %s in %d ms)\n", *role, n, sku,
				solace.FieldString(payload, "price"), solace.FieldString(payload, "by"),
				time.Since(sent).Milliseconds())
		}, resource.TopicOf("rr/pricing/price/request/v1/"+sku), *timeout, nil, nil)
		if err != nil {
			solace.Errf(*role, "could not send request #%d: %s", n, err)
		}
	}
}

// Pricing answers price requests. --slow makes each answer take longer
// than the till waits, so the till gives up before the reply arrives.
func Pricing(args []string) {
	fs := flag.NewFlagSet("rr pricing", flag.ExitOnError)
	user := fs.String("user", "svc-rr-pricing", "client username to connect as")
	role := fs.String("role", "pricing", "label for log lines")
	clientName := fs.String("client-name", "rr-pricing", "client name the broker reports this app by")
	work := fs.Duration("work", 20*time.Millisecond, "time to look a price up")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	// Request-reply builders live on the service's RequestReply() view.
	rr := svc.RequestReply()
	rcv, err := rr.CreateRequestReplyMessageReceiverBuilder().
		Build(resource.TopicSubscriptionOf("rr/pricing/price/request/v1/>"))
	if err == nil {
		err = rcv.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start the replier: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(rcv)

	solace.Logf(*role, "answering price requests on rr/pricing/price/request/v1/>, %s each.", *work)
	fmt.Println()

	var mu sync.Mutex
	answered := 0
	if err := rcv.ReceiveAsync(func(req message.InboundMessage, replier sol.Replier) {
		time.Sleep(*work)
		payload := solace.PayloadOf(req)
		n, _ := solace.FieldInt(payload, "request")
		sku := solace.FieldString(payload, "sku")
		price := fmt.Sprintf("%.2f", 1+float64(len(sku)*7+n%13)+0.99)
		reply, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"sku":"%s","price":"%s","by":"%s"}`, sku, price, *role))
		if err == nil {
			err = replier.Reply(reply)
		}
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			solace.Errf(*role, "could not reply to request #%d: %s", n, err)
			return
		}
		answered++
		fmt.Printf("[%s] request #%d: %s costs %s, replied\n", *role, n, sku, price)
	}); err != nil {
		solace.Errf(*role, "could not register the request handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	mu.Lock()
	defer mu.Unlock()
	fmt.Println()
	solace.Logf(*role, "stopped after answering %d requests.", answered)
}
