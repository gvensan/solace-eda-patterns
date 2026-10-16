// Package compete implements the competing consumers scenario.
//
// Every order the shop takes needs a confirmation email. Sending one takes
// a moment, so one sender cannot keep up at busy times. The jobs go on one
// shared queue, and several identical workers take from it: the broker gives
// each job to exactly one of them, and adding a worker adds capacity.
package compete

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

var names = []string{"ana", "ben", "chen", "dara", "eli", "fran", "gus", "hana"}

// Checkout publishes one "order confirmed" event per order, at a steady pace
// a little faster than two workers can send emails, so the queue always has
// work waiting and adding a third worker visibly helps.
func Checkout(args []string) {
	fs := flag.NewFlagSet("compete checkout", flag.ExitOnError)
	user := fs.String("user", "svc-cc-checkout", "client username to connect as")
	role := fs.String("role", "checkout", "label for log lines")
	rate := fs.Float64("rate", 6, "orders per second")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "cc-checkout")
	defer svc.Disconnect()

	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	every := time.Duration(float64(time.Second) / *rate)
	solace.Logf(*role, "confirming %.0f orders a second. Press Stop to end.", *rate)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()

	for n := 1; ; n++ {
		order := fmt.Sprintf("O-%05d", 20000+n)
		email := names[rand.Intn(len(names))] + "@example.com"
		topic := "cc/shop/order/confirmed/v1/" + order
		payload := fmt.Sprintf(`{"order":"%s","email":"%s"}`, order, email)
		if err := solace.PublishKeyed(svc, pub, topic, payload, ""); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else if n%10 == 0 {
			// One line in ten: at six a second, a line per order would bury
			// the workers' logs, which are the interesting ones.
			solace.Logf(*role, "%d orders confirmed so far (latest %s)", n, order)
		}

		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d order(s).", n)
			return
		case <-time.After(every):
		}
	}
}

// Worker sends confirmation emails, one at a time, from the shared queue.
//
// Every worker runs the same code and binds the same queue. None of them
// knows about the others: the broker decides which worker gets each job.
func Worker(args []string) {
	fs := flag.NewFlagSet("compete worker", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines, e.g. worker-1")
	user := fs.String("user", "svc-cc-worker", "client username to connect as")
	clientName := fs.String("client-name", "", "client name the broker reports this worker by")
	queue := fs.String("queue", "q.cc.emails", "shared queue to take jobs from")
	work := fs.Duration("work", 400*time.Millisecond, "time to send one email")
	fs.Parse(args)

	if *role == "" || *clientName == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop compete worker --role <role> --client-name <name> [--work 400ms]")
		os.Exit(2)
	}

	stalled := solace.StallOnSignal(*role)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	// Shared: every worker binds the same queue and the broker spreads the
	// jobs across them. Client acknowledgement, so a job a worker took but
	// had not finished goes back to the queue if the worker goes away.
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true, Shared: true})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "sending emails, about %s each.", *work)
	fmt.Println()

	var mu sync.Mutex
	sent, redelivered := 0, 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		if stalled() {
			solace.Freeze()
		}
		time.Sleep(*work)

		payload := solace.PayloadOf(msg)
		mu.Lock()
		sent++
		n := sent
		again := msg.IsRedelivered()
		if again {
			redelivered++
		}
		mu.Unlock()

		line := fmt.Sprintf("[%s] #%d sent confirmation for %s to %s", *role, n,
			solace.FieldString(payload, "order"), solace.FieldString(payload, "email"))
		if again {
			line += "  (redelivered)"
		}
		fmt.Println(line)
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	mu.Lock()
	defer mu.Unlock()
	fmt.Println()
	solace.Logf(*role, "stopped after sending %d email(s), %d of them redelivered.", sent, redelivered)
}
