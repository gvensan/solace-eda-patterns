// Package bridge implements the event bridge scenario.
//
// A retailer's head office and its stores run separate messaging domains:
// two message VPNs, each with its own users, topics and queues. Head office
// publishes price changes in its VPN. A bridge carries them into the stores'
// VPN, where the store's till reads them from its own queue. Neither app
// knows the other VPN exists. Which VPN an app is in comes from
// SOLACE_MSG_VPN, as for every app in the workshop.
package bridge

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

// HQ publishes a price change every --every in its own VPN. --promos
// publishes promotions instead, on a topic the bridge was not set up for.
func HQ(args []string) {
	fs := flag.NewFlagSet("br hq", flag.ExitOnError)
	user := fs.String("user", "svc-br-pricing", "client username to connect as")
	role := fs.String("role", "hq", "label for log lines")
	clientName := fs.String("client-name", "br-hq-pricing", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between messages")
	promos := fs.Bool("promos", false, "publish promotions instead of prices")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	if *promos {
		solace.Logf(*role, "publishing a promotion every %s to br/hq/promo/v1/<sku>", *every)
	} else {
		solace.Logf(*role, "publishing a price change every %s to br/hq/price/changed/v1/<sku>", *every)
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for n := 1; ; n++ {
		sku := fmt.Sprintf("SKU-%02d", 1+rand.Intn(8))
		now := time.Now().UnixMilli()
		topic := "br/hq/price/changed/v1/" + sku
		payload := fmt.Sprintf(`{"n":%d,"sku":"%s","price":"%d.99","at":%d}`, n, sku, 1+rand.Intn(20), now)
		what := fmt.Sprintf("price #%d: %s now %s", n, sku, solace.FieldString(payload, "price"))
		if *promos {
			topic = "br/hq/promo/v1/" + sku
			payload = fmt.Sprintf(`{"n":%d,"sku":"%s","promo":"%d%% off","at":%d}`, n, sku, 10*(1+rand.Intn(3)), now)
			what = fmt.Sprintf("promotion #%d: %s %s", n, sku, solace.FieldString(payload, "promo"))
		}
		if err := solace.PublishConfirmed(svc, pub, topic, payload); err != nil {
			solace.Errf(*role, "%s refused: %s", what, err)
		} else {
			solace.Logf(*role, "%s, published", what)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Store reads its queue in the stores' VPN and prints each price change or
// promotion with how long ago head office published it.
func Store(args []string) {
	fs := flag.NewFlagSet("br store", flag.ExitOnError)
	user := fs.String("user", "svc-br-store", "client username to connect as")
	role := fs.String("role", "store", "label for log lines")
	clientName := fs.String("client-name", "br-store", "client name the broker reports this app by")
	queue := fs.String("queue", "q.br.store", "queue in the stores' VPN")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	fmt.Println()

	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		n, _ := solace.FieldInt(p, "n")
		at, _ := solace.FieldInt(p, "at")
		age := time.Since(time.UnixMilli(int64(at))).Seconds()
		sku := solace.FieldString(p, "sku")
		if strings.Contains(msg.GetDestinationName(), "/promo/") {
			solace.Logf(*role, "promotion #%d: %s %s (sent by head office %.1f s ago)", n, sku, solace.FieldString(p, "promo"), age)
		} else {
			solace.Logf(*role, "price #%d: %s now %s (sent by head office %.1f s ago)", n, sku, solace.FieldString(p, "price"), age)
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
