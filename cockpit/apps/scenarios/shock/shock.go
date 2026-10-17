// Package shock implements the shock absorber scenario.
//
// Concert ticket sales. When tickets go on sale, hundreds of purchase
// requests arrive in a few seconds; otherwise they trickle in. Two booking
// workers reserve seats at a steady pace. The queue between them is the
// shock absorber: it takes the burst at the speed the on-sale page sends
// it, and the workers drain it at theirs. Nobody is slowed down or refused,
// as long as the queue is big enough for the burst and the requests are
// still worth handling by the time they come out.
package shock

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// seatChoices stands in for what a real purchase request carries (the
// seats asked for, in order of preference, and the customer's details). It
// gives each request a realistic size, about 400 bytes, which is what the
// queue's size in MB is measured against.
var seatChoices = strings.Repeat("A12,A13,A14,B12,B13,B14,C12,C13,", 10)

// OnSale publishes a trickle of purchase requests and, every --every-burst
// and on SIGUSR1, a burst of --burst requests as fast as it can. With --once
// it sends one burst and exits: a release bigger than usual, or one whose
// requests carry a time to live (a seat hold that lapses). Each request is
// numbered and carries the time it was made.
func OnSale(args []string) {
	fs := flag.NewFlagSet("sa onsale", flag.ExitOnError)
	user := fs.String("user", "svc-sa-onsale", "client username to connect as")
	role := fs.String("role", "onsale", "label for log lines")
	clientName := fs.String("client-name", "sa-onsale", "client name the broker reports this app by")
	every := fs.Duration("every", 500*time.Millisecond, "time between requests outside a burst")
	burst := fs.Int("burst", 1500, "requests in a burst")
	everyBurst := fs.Duration("every-burst", 60*time.Second, "time between bursts; 0 means only on signal")
	once := fs.Bool("once", false, "send one burst, wait for its receipts, and exit")
	ttl := fs.Duration("ttl", 0, "time to live on each request; 0 means none")
	prefix := fs.String("prefix", "R", "letter in front of each request's number, so two pages never share one")
	fs.Parse(args)

	// Caught before connecting, so an early signal does not take SIGUSR1's
	// default action, which is to exit.
	bursts := make(chan os.Signal, 1)
	signal.Notify(bursts, syscall.SIGUSR1)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	// Receipts arrive on their own; refusals are the only ones worth a line.
	pub, err := svc.CreatePersistentMessagePublisherBuilder().OnBackPressureWait(10000).Build()
	if err != nil {
		solace.Errf(*role, "could not build the publisher: %s", err)
		os.Exit(1)
	}
	var mu sync.Mutex
	refused, confirmed := 0, 0
	pub.SetMessagePublishReceiptListener(func(r sol.PublishReceipt) {
		mu.Lock()
		defer mu.Unlock()
		if err := r.GetError(); err != nil {
			refused++
			if refused <= 5 || refused%100 == 0 {
				solace.Errf(*role, "request %v refused: %s", r.GetUserContext(), err)
			}
			return
		}
		confirmed++
	})
	if err := pub.Start(); err != nil {
		solace.Errf(*role, "could not start the publisher: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(pub)

	hold := ""
	if *ttl > 0 {
		hold = fmt.Sprintf(", each valid for %s (time to live)", *ttl)
	}
	if *once {
		solace.Logf(*role, "one burst of %d requests%s.", *burst, hold)
	} else if *everyBurst > 0 {
		solace.Logf(*role, "a request every %s, and a burst of %d every %s or on signal%s.", *every, *burst, *everyBurst, hold)
	} else {
		solace.Logf(*role, "a request every %s, and a burst of %d on signal%s.", *every, *burst, hold)
	}
	fmt.Println()

	send := func(n int) {
		b := svc.MessageBuilder()
		if *ttl > 0 {
			b = b.WithProperty(config.MessagePropertyPersistentTimeToLive, ttl.Milliseconds())
		}
		id := fmt.Sprintf("%s-%d", *prefix, n)
		msg, err := b.BuildWithStringPayload(fmt.Sprintf(`{"request":"%s","at":%d,"customer":"C-%06d","seats":"%s"}`,
			id, time.Now().UnixMilli(), n%1000000, seatChoices))
		if err == nil {
			err = pub.Publish(msg, resource.TopicOf("sa/tickets/purchase/requested/v1/"+id), nil, id)
		}
		if err != nil {
			solace.Errf(*role, "could not send request %s: %s", id, err)
		}
	}

	n := 0
	sendBurst := func() {
		start := time.Now()
		first := n + 1
		for i := 0; i < *burst; i++ {
			n++
			send(n)
		}
		solace.Logf(*role, "BURST: requests %s-%d to %s-%d handed to the API in %d ms; the broker never slowed the on-sale page down",
			*prefix, first, *prefix, n, time.Since(start).Milliseconds())
	}
	// Once the receipts are in, say how the broker took the burst.
	outcome := func() {
		mu.Lock()
		defer mu.Unlock()
		if refused > 0 {
			solace.Errf(*role, "the broker refused %d request(s): those customers got no booking", refused)
		} else {
			solace.Logf(*role, "every request confirmed by the broker: %d stored, none refused", confirmed)
		}
	}

	if *once {
		sendBurst()
		time.Sleep(3 * time.Second)
		outcome()
		return
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(*every)
	defer tick.Stop()
	var periodic <-chan time.Time
	if *everyBurst > 0 {
		t := time.NewTicker(*everyBurst)
		defer t.Stop()
		periodic = t.C
	}
	for {
		select {
		case <-stop:
			mu.Lock()
			solace.Logf(*role, "stopping: %d confirmed, %d refused.", confirmed, refused)
			mu.Unlock()
			return
		case <-tick.C:
			n++
			send(n)
		case <-bursts:
			sendBurst()
			time.AfterFunc(3*time.Second, outcome)
		case <-periodic:
			sendBurst()
			time.AfterFunc(3*time.Second, outcome)
		}
	}
}

// Booking reserves a seat for each request at a steady --rate, and says
// how long each request waited on the queue when that is more than a few
// seconds. With --drain it takes everything as fast as it can, without
// booking, until the queue has been empty for 2 seconds, and exits: the
// way to clear a backlog a card left behind.
func Booking(args []string) {
	fs := flag.NewFlagSet("sa booking", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines, e.g. booking-1")
	user := fs.String("user", "svc-sa-booking", "client username to connect as")
	clientName := fs.String("client-name", "", "client name the broker reports this worker by")
	queue := fs.String("queue", "q.sa.purchases", "queue of purchase requests")
	rate := fs.Float64("rate", 20, "bookings per second")
	drain := fs.Bool("drain", false, "take everything without booking, until the queue is empty, then exit")
	fs.Parse(args)
	if *role == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop sa booking --role <name> --client-name <name>")
		os.Exit(2)
	}

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true, Shared: true})
	defer solace.Terminate(rcv)
	if *drain {
		solace.Logf(*role, "clearing %s: taking every request without booking it.", *queue)
	} else {
		solace.Logf(*role, "booking about %.0f a second from %s.", *rate, *queue)
	}
	fmt.Println()

	work := time.Duration(float64(time.Second) / *rate)
	if *drain {
		work = 0
	}
	var mu sync.Mutex
	booked, lastSeen := 0, 0
	last := ""
	var waited time.Duration
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(work)
		payload := solace.PayloadOf(msg)
		id := solace.FieldString(payload, "request")
		at, _ := solace.FieldInt(payload, "at")
		w := time.Since(time.UnixMilli(int64(at)))
		mu.Lock()
		booked++
		waited = w
		if w > 5*time.Second && booked%20 == 0 {
			fmt.Printf("[%s] booked request %s, which waited %.0f s\n", *role, id, w.Seconds())
		}
		last = id
		mu.Unlock()
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge %s: %s", id, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	if *drain {
		// The backlog is gone once the requests coming out are fresh (the
		// on-sale page's trickle never stops, so the queue is never quiet),
		// or once nothing has come for 2 seconds.
		start := time.Now()
		quiet := 0
		for quiet < 4 {
			time.Sleep(500 * time.Millisecond)
			mu.Lock()
			if booked == lastSeen {
				quiet++
			} else {
				quiet = 0
			}
			fresh := booked > 0 && booked > lastSeen && waited < time.Second
			lastSeen = booked
			mu.Unlock()
			if fresh {
				break
			}
		}
		mu.Lock()
		solace.Logf(*role, "cleared %d request(s) in %.0f s; the queue is empty.", booked, time.Since(start).Seconds())
		mu.Unlock()
		return
	}
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			mu.Lock()
			solace.Logf(*role, "stopped after %d bookings.", booked)
			mu.Unlock()
			return
		case <-tick.C:
			mu.Lock()
			if booked > 0 {
				solace.Logf(*role, "%d booked; the latest, %s, waited %.1f s", booked, last, waited.Seconds())
			}
			mu.Unlock()
		}
	}
}
