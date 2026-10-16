// Package exclusive implements the exclusive consumer scenario.
//
// A payments service publishes ledger postings. Each posting has to be
// applied once, in the order it was made, so only one ledger writer may work
// at a time. Two instances run anyway, for availability: both bind the same
// exclusive queue, the broker delivers to whichever bound first, and the
// other waits as a standby until the active one goes away.
//
// The broker decides who is active, not the apps. Each ledger learns its role
// from the Go API's activation and passivation listener, and prints it.
package exclusive

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

// accounts are the ledger accounts postings are made against. The topic ends
// in the account, but every posting lands on the one queue regardless: the
// order that matters here is the order across all of them.
var accounts = []string{"ACC-1001", "ACC-1002", "ACC-1003", "ACC-1004", "ACC-1005"}

// Publish sends one ledger posting at a steady pace, each with a sequence
// number, so a ledger can show that it applies them in order and with no gaps.
func Publish(args []string) {
	fs := flag.NewFlagSet("exclusive publish", flag.ExitOnError)
	user := fs.String("user", "svc-excl-payments", "client username to connect as")
	role := fs.String("role", "payments", "label for log lines")
	every := fs.Duration("every", 500*time.Millisecond, "pause between postings")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "excl-payments")
	defer svc.Disconnect()

	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	solace.Logf(*role, "publishing a ledger posting every %s. Press Stop to end.", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()

	// Sequences restart with each publisher run, so every posting also
	// carries the run it came from and a ledger compares within one run.
	run := time.Now().UnixMilli()
	for seq := 1; ; seq++ {
		account := accounts[rand.Intn(len(accounts))]
		amount := fmt.Sprintf("%d.%02d", 5+rand.Intn(495), rand.Intn(100))
		topic := "excl/ledger/posting/v1/" + account
		payload := fmt.Sprintf(`{"run":%d,"seq":%d,"account":"%s","amount":"%s"}`, run, seq, account, amount)

		if err := solace.PublishKeyed(svc, pub, topic, payload, ""); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		} else {
			solace.Logf(*role, "posting %d  %s  %s", seq, account, amount)
		}

		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d posting(s).", seq)
			return
		case <-time.After(*every):
		}
	}
}

// Ledger binds the exclusive queue and applies postings one at a time.
//
// Several instances can run. Only the one the broker has made active receives
// anything; the rest are bound and idle. On SIGUSR1 the instance stalls: it
// stays connected and keeps its place as the active consumer, but stops
// processing and acknowledging, the way a deadlocked or paused process does.
func Ledger(args []string) {
	fs := flag.NewFlagSet("exclusive ledger", flag.ExitOnError)
	role := fs.String("role", "", "label for log lines, e.g. ledger-a")
	user := fs.String("user", "svc-excl-ledger", "client username to connect as")
	clientName := fs.String("client-name", "", "client name the broker reports this instance by")
	queue := fs.String("queue", "q.excl.postings", "exclusive queue to bind")
	work := fs.Duration("work", 250*time.Millisecond, "time spent applying each posting")
	fs.Parse(args)

	if *role == "" {
		fmt.Fprintln(os.Stderr, "usage: workshop exclusive ledger --role <role> [--client-name name] [--queue q] [--work 250ms]")
		os.Exit(2)
	}

	// Registered before connecting, so a stall request that arrives early is
	// caught rather than taking the default action for SIGUSR1, which is exit.
	stallSignal := make(chan os.Signal, 1)
	signal.Notify(stallSignal, syscall.SIGUSR1)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{
		// Client acknowledgement, so a posting this instance has taken but
		// not finished goes back to the queue, and to the standby, if this
		// instance goes away.
		ClientAck: true,
		OnActivation: func(active bool) {
			if active {
				solace.Logf(*role, "ACTIVE: the broker is delivering %s to this instance", *queue)
			} else {
				solace.Logf(*role, "STANDBY: bound to %s, receiving nothing", *queue)
			}
		},
	})
	defer solace.Terminate(rcv)
	// Every receiver starts passive. The listener reports changes only, so
	// the instance that is not chosen hears nothing further: this line is
	// the last it prints until it is made active.
	solace.Logf(*role, "STANDBY until the broker makes this instance active")

	var mu sync.Mutex
	stalled := false
	go func() {
		<-stallSignal
		mu.Lock()
		stalled = true
		mu.Unlock()
		solace.Logf(*role, "STALLED: still connected and still bound, but no longer processing or acknowledging")
	}()

	applied, redelivered := 0, 0
	lastRun, lastSeq := 0, 0

	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		mu.Lock()
		if stalled {
			mu.Unlock()
			// Hold the delivery thread, as a stuck process would. Nothing
			// is acknowledged from here on.
			select {}
		}
		mu.Unlock()

		time.Sleep(*work)

		payload := solace.PayloadOf(msg)
		run, _ := numberOf(payload, "run")
		seq, _ := numberOf(payload, "seq")
		account := stringOf(payload, "account")
		amount := stringOf(payload, "amount")

		note := ""
		switch {
		case run != lastRun:
			lastRun, lastSeq = run, seq
		case seq == lastSeq+1:
			lastSeq = seq
		case seq <= lastSeq:
			note = fmt.Sprintf("  already applied (last was %d)", lastSeq)
		default:
			note = fmt.Sprintf("  GAP: expected %d", lastSeq+1)
			lastSeq = seq
		}

		applied++
		line := fmt.Sprintf("[%s] applied posting %d  %s  %s", *role, seq, account, amount)
		if msg.IsRedelivered() {
			redelivered++
			line += "  (redelivered)"
		}
		fmt.Println(line + note)

		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge posting %d: %s", seq, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	fmt.Println()
	solace.Logf(*role, "stopped after applying %d posting(s), %d of them redelivered.", applied, redelivered)
}

// numberOf pulls a numeric field out of a posting without a full JSON parse.
func numberOf(payload, field string) (int, bool) {
	key := `"` + field + `":`
	i := strings.Index(payload, key)
	if i < 0 {
		return 0, false
	}
	n, found := 0, false
	for _, c := range payload[i+len(key):] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		found = true
	}
	return n, found
}

// stringOf pulls a string field out of a posting.
func stringOf(payload, field string) string {
	key := `"` + field + `":"`
	i := strings.Index(payload, key)
	if i < 0 {
		return ""
	}
	rest := payload[i+len(key):]
	if j := strings.IndexByte(rest, '"'); j >= 0 {
		return rest[:j]
	}
	return rest
}
