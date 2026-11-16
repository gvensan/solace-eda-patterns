// Package guaranteed implements the guaranteed delivery scenario.
//
// A loyalty programme: every purchase at the tills earns points, and the
// points service awards them. Losing an award cheats a customer; awarding it
// twice costs the business. Three things together make sure neither happens:
// the broker confirms each purchase to the tills once it has stored it, the
// points service acknowledges each one only after awarding it, and the points
// service remembers what it has awarded, so a purchase delivered twice is
// recognised and skipped.
package guaranteed

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// purchase is one "points earned" event.
type purchase struct {
	id       string
	seq      int
	customer string
	points   int
}

func (p purchase) payload(run string) string {
	return fmt.Sprintf(`{"purchase":"%s","run":"%s","seq":%d,"customer":"%s","points":%d}`,
		p.id, run, p.seq, p.customer, p.points)
}

// Tills publishes a purchase every half second, guaranteed, without waiting
// for each confirmation: the broker's receipts arrive on their own, carrying
// the purchase they confirm. A purchase the broker refuses goes into an
// outbox. While anything is in the outbox, new purchases queue behind it and
// only the oldest is retried, once a second. Once the broker confirms that
// one, it is taking messages again, so the rest of the outbox is sent at once,
// in order: messages from one publisher reach the queue in the order they
// were sent, so purchases arrive in the order they were made.
func Tills(args []string) {
	fs := flag.NewFlagSet("gd tills", flag.ExitOnError)
	user := fs.String("user", "svc-gd-tills", "client username to connect as")
	role := fs.String("role", "tills", "label for log lines")
	every := fs.Duration("every", 500*time.Millisecond, "time between purchases")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "gd-tills")
	defer svc.Disconnect()

	pub, err := svc.CreatePersistentMessagePublisherBuilder().OnBackPressureWait(1000).Build()
	if err != nil {
		solace.Errf(*role, "could not build the publisher: %s", err)
		os.Exit(1)
	}

	// Ids carry this run, so a restarted till never reuses one: the points
	// service keeps its record across runs, and would skip a reused id.
	run := strconv.FormatInt(time.Now().UnixMilli(), 36)
	topic := resource.TopicOf("gd/loyalty/points/earned/v1/tills")

	var mu sync.Mutex
	var outbox []purchase // refused or waiting, oldest first
	retrying := false     // something was refused: send only the oldest
	inFlight := false     // a retry is waiting for its receipt
	resending := map[string]bool{}
	// Asks the main loop to send the rest of the outbox now.
	flush := make(chan struct{}, 1)

	send := func(p purchase) {
		msg, err := svc.MessageBuilder().WithApplicationMessageID(p.id).BuildWithStringPayload(p.payload(run))
		if err != nil {
			solace.Errf(*role, "could not build %s: %s", p.id, err)
			return
		}
		// The purchase travels as the publish context and comes back on its
		// receipt.
		if err := pub.Publish(msg, topic, nil, p); err != nil {
			solace.Errf(*role, "could not send %s: %s", p.id, err)
		}
	}

	pub.SetMessagePublishReceiptListener(func(r sol.PublishReceipt) {
		p, _ := r.GetUserContext().(purchase)
		mu.Lock()
		defer mu.Unlock()
		if err := r.GetError(); err != nil {
			solace.Errf(*role, "refused %s: %s", p.id, err)
			delete(resending, p.id)
			outbox = append(outbox, p)
			sort.Slice(outbox, func(i, j int) bool { return outbox[i].seq < outbox[j].seq })
			if !retrying {
				solace.Logf(*role, "kept for retry: %s and anything after it waits in the outbox", p.id)
			}
			retrying, inFlight = true, false
			return
		}
		if resending[p.id] {
			delete(resending, p.id)
			solace.Logf(*role, "resent %s: confirmed by the broker", p.id)
			if inFlight {
				// The retry got through: the queue is taking messages again.
				inFlight = false
				select {
				case flush <- struct{}{}:
				default:
				}
			}
			if len(resending) == 0 && len(outbox) == 0 {
				solace.Logf(*role, "outbox empty: every refused purchase is now stored")
			}
			return
		}
		solace.Logf(*role, "confirmed %s (#%d, %d points for %s)", p.id, p.seq, p.points, p.customer)
	})
	if err := pub.Start(); err != nil {
		solace.Errf(*role, "could not start the publisher: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(pub)

	solace.Logf(*role, "a purchase every %s; each is confirmed by the broker once it has stored it.", *every)
	fmt.Println()

	// Send everything in the outbox, oldest first, and go back to sending
	// purchases as they are made, after them.
	flushOutbox := func() {
		mu.Lock()
		if !retrying || inFlight {
			mu.Unlock()
			return
		}
		batch := outbox
		outbox = nil
		retrying = false
		for _, p := range batch {
			resending[p.id] = true
		}
		mu.Unlock()
		if len(batch) > 0 {
			solace.Logf(*role, "the broker is taking purchases again: sending the %d in the outbox, in order", len(batch))
		}
		for _, p := range batch {
			send(p)
		}
	}

	// Retry the oldest purchase in the outbox, unless one is already waiting
	// for its receipt.
	sendOldest := func() {
		mu.Lock()
		if !retrying || inFlight || len(outbox) == 0 {
			mu.Unlock()
			return
		}
		p := outbox[0]
		outbox = outbox[1:]
		inFlight = true
		resending[p.id] = true
		mu.Unlock()
		send(p)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()

	// Retries go once a second; purchases come at their own pace.
	retry := time.NewTicker(time.Second)
	defer retry.Stop()
	made := time.NewTicker(*every)
	defer made.Stop()
	seq := 0
	for {
		select {
		case <-stop:
			mu.Lock()
			if len(outbox) > 0 {
				solace.Logf(*role, "stopping with %d purchase(s) still in the outbox, not sent.", len(outbox))
			}
			mu.Unlock()
			return
		case <-made.C:
			seq++
			p := purchase{id: fmt.Sprintf("P-%s-%04d", run, seq), seq: seq,
				customer: fmt.Sprintf("C-%d", 1+rand.Intn(5)), points: 5 * (1 + rand.Intn(5))}
			mu.Lock()
			if retrying {
				outbox = append(outbox, p)
				solace.Logf(*role, "%s waits in the outbox behind the refused ones (%d waiting)", p.id, len(outbox))
				mu.Unlock()
				continue
			}
			mu.Unlock()
			send(p)
		case <-flush:
			flushOutbox()
		case <-retry.C:
			sendOldest()
		}
	}
}

// store is the points service's record of what it has awarded: one line per
// award, appended and flushed to disk before the award is acknowledged, so
// it survives a crash. It stands in for the database a real service would
// update.
type store struct {
	path     string
	applied  map[string]bool
	balances map[string]int
}

func openStore(role string) *store {
	dir := filepath.Join(solace.Env("COCKPIT_STATE_DIR", "."), "guaranteed-delivery")
	st := &store{path: filepath.Join(dir, "points-awarded.log"), applied: map[string]bool{}, balances: map[string]int{}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		solace.Errf(role, "could not create %s: %s", dir, err)
		os.Exit(1)
	}
	f, err := os.Open(st.path)
	if err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var id, customer string
			var points int
			if _, err := fmt.Sscan(sc.Text(), &id, &customer, &points); err == nil {
				st.applied[id] = true
				st.balances[customer] += points
			}
		}
		f.Close()
	}
	return st
}

func (st *store) award(id, customer string, points int) error {
	f, err := os.OpenFile(st.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s %s %d\n", id, customer, points); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	st.applied[id] = true
	st.balances[customer] += points
	return nil
}

// Points awards the points for each purchase, then acknowledges it.
//
// It is idempotent: a purchase it has already awarded (because the broker
// delivered it again after a crash) is acknowledged and skipped. With
// --no-dedupe it awards it again, to show what that check prevents.
//
// SIGUSR2 asks it to stop itself at the worst moment: right after the next
// award is written, before that purchase is acknowledged. This is not a kill
// from outside; it puts a crash exactly where duplicates come from.
func Points(args []string) {
	fs := flag.NewFlagSet("gd points", flag.ExitOnError)
	user := fs.String("user", "svc-gd-points", "client username to connect as")
	role := fs.String("role", "points", "label for log lines")
	clientName := fs.String("client-name", "gd-points", "client name the broker reports this app by")
	queue := fs.String("queue", "q.gd.awards", "queue of purchases")
	noDedupe := fs.Bool("no-dedupe", false, "award a purchase again even if it was awarded before")
	forget := fs.Bool("forget", false, "delete the record of awards, and exit")
	fs.Parse(args)

	st := openStore(*role)
	if *forget {
		if err := os.Remove(st.path); err != nil && !os.IsNotExist(err) {
			solace.Errf(*role, "could not delete %s: %s", st.path, err)
			os.Exit(1)
		}
		solace.Logf(*role, "deleted the record of %d award(s).", len(st.applied))
		return
	}

	// Caught before connecting, so an early signal does not take SIGUSR2's
	// default action, which is to exit at once.
	var stopAfterNext atomic.Bool
	usr2 := make(chan os.Signal, 1)
	signal.Notify(usr2, syscall.SIGUSR2)
	go func() {
		for range usr2 {
			stopAfterNext.Store(true)
			solace.Logf(*role, "asked to stop right after the next award, before acknowledging it")
		}
	}()

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)

	if *noDedupe {
		solace.Logf(*role, "NOT checking for purchases already awarded (--no-dedupe).")
	}
	solace.Logf(*role, "%d award(s) on record from before. Awarding points from %s.", len(st.applied), *queue)
	fmt.Println()

	var mu sync.Mutex
	last := map[string]int{} // run -> last sequence number seen
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		mu.Lock()
		defer mu.Unlock()
		payload := solace.PayloadOf(msg)
		id, _ := msg.GetApplicationMessageID()
		run := solace.FieldString(payload, "run")
		seq, _ := solace.FieldInt(payload, "seq")
		customer := solace.FieldString(payload, "customer")
		points, _ := solace.FieldInt(payload, "points")
		mark := ""
		if msg.IsRedelivered() {
			mark = "  (redelivered)"
		}

		if st.applied[id] && !*noDedupe {
			fmt.Printf("[%s] %s already awarded, skipped%s\n", *role, id, mark)
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(*role, "could not acknowledge %s: %s", id, err)
			}
			return
		}

		again := st.applied[id]
		if err := st.award(id, customer, points); err != nil {
			solace.Errf(*role, "could not record %s: %s", id, err)
			// Not acknowledged: it holds a slot in the window, and the
			// broker delivers it again after a reconnect or restart.
			return
		}
		note := ""
		if prev, ok := last[run]; ok && seq > prev+1 {
			note = fmt.Sprintf("  GAP: #%d after #%d, a purchase is missing", seq, prev)
		}
		if seq > last[run] {
			last[run] = seq
		}
		verb := "awarded"
		if again {
			verb = "awarded again"
		}
		fmt.Printf("[%s] %s %s: +%d points to %s, balance %d%s%s\n",
			*role, verb, id, points, customer, st.balances[customer], mark, note)

		if stopAfterNext.Load() {
			// The award is on disk; the acknowledgement is never sent.
			solace.Logf(*role, "stopping now: %s is awarded but not acknowledged", id)
			os.Exit(3)
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge %s: %s", id, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	mu.Lock()
	defer mu.Unlock()
	var who []string
	for c, b := range st.balances {
		who = append(who, fmt.Sprintf("%s %d", c, b))
	}
	sort.Strings(who)
	fmt.Println()
	solace.Logf(*role, "stopped. Balances: %s.", strings.Join(who, ", "))
}
