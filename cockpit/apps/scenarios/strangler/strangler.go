// Package strangler implements the strangler fig scenario.
//
// A shop's old order system handles every order event: placed, paid and
// refund requested. The team is replacing it one piece at a time, starting
// with refunds. The shop keeps publishing exactly as before. What moves is a
// queue subscription on the broker: refund events stop going to the old
// system's queue and start going to the new refund service's queue. Neither
// the shop nor the old system changes.
package strangler

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

// Shop publishes an order event a second: placed, paid, and every third
// order a refund request.
func Shop(args []string) {
	fs := flag.NewFlagSet("stg shop", flag.ExitOnError)
	user := fs.String("user", "svc-stg-shop", "client username to connect as")
	role := fs.String("role", "shop", "label for log lines")
	clientName := fs.String("client-name", "stg-shop", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between events")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	solace.Logf(*role, "publishing order events; it does not know who handles them")
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	base := int(time.Now().Unix()%10000) * 10
	refund := 0
	for n := 0; ; n++ {
		order := fmt.Sprintf("O-%05d", base+n/2)
		topic, what := "stg/shop/order/placed/v1/"+order, order+" placed"
		if n%2 == 1 {
			topic, what = "stg/shop/order/paid/v1/"+order, order+" paid"
		}
		if n%6 == 5 {
			refund++
			id := fmt.Sprintf("R-%05d", base+refund)
			topic, what = "stg/shop/refund/requested/v1/"+id, "refund "+id+" requested"
		}
		if err := solace.PublishConfirmed(svc, pub, topic, fmt.Sprintf(`{"event":"%s"}`, what)); err != nil {
			solace.Errf(*role, "%s refused: %s", what, err)
		} else {
			solace.Logf(*role, "%s", what)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

func handle(role, user, clientName, queue, system string) {
	svc := solace.ConnectAs(role, user, clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(role, user, svc, queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		t := msg.GetDestinationName()
		id := t[strings.LastIndexByte(t, '/')+1:]
		switch {
		case strings.Contains(t, "/refund/"):
			solace.Logf(role, "refunded %s (%s)", id, system)
		case strings.Contains(t, "/paid/"):
			solace.Logf(role, "%s: payment recorded (%s)", id, system)
		default:
			solace.Logf(role, "%s: order recorded (%s)", id, system)
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}

// Legacy is the old order system. It handles whatever its queue receives.
func Legacy(args []string) {
	fs := flag.NewFlagSet("stg legacy", flag.ExitOnError)
	user := fs.String("user", "svc-stg-legacy", "client username to connect as")
	role := fs.String("role", "legacy", "label for log lines")
	clientName := fs.String("client-name", "stg-legacy", "client name the broker reports this app by")
	queue := fs.String("queue", "q.stg.legacy", "queue to read")
	fs.Parse(args)
	handle(*role, *user, *clientName, *queue, "old order system")
}

// Refunds is the new refund service. It handles whatever its queue receives,
// which is nothing until refunds are routed to it.
func Refunds(args []string) {
	fs := flag.NewFlagSet("stg refunds", flag.ExitOnError)
	user := fs.String("user", "svc-stg-refunds", "client username to connect as")
	role := fs.String("role", "refunds", "label for log lines")
	clientName := fs.String("client-name", "stg-refunds", "client name the broker reports this app by")
	queue := fs.String("queue", "q.stg.refunds", "queue to read")
	fs.Parse(args)
	handle(*role, *user, *clientName, *queue, "new refund service")
}
