// Package streaming implements the event streaming scenario.
//
// A bike-share. Every ride start and ride end is an event, and the counter
// keeps which bikes are in use: the bike is added on a start and removed on
// an end. That set is only right if the counter has seen the stream in
// order from where its state started. A queue keeps the stream in order
// and holds the counter's position in it; a snapshot keeps the set across
// a crash. The city's board lists the rides in progress from the same
// events, taken direct, with neither, and joining late it has no history.
//
// Each event also carries the bikes' own total, as a check to compare
// against. A total cannot say which bikes are out or since when, so it
// shows that the board's list is short without being able to fill it.
package streaming

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

const bikes = 40

// Bikes publishes ride starts and ends, four a second, as guaranteed
// messages numbered in order.
func Bikes(args []string) {
	fs := flag.NewFlagSet("es bikes", flag.ExitOnError)
	user := fs.String("user", "svc-es-bikes", "client username to connect as")
	role := fs.String("role", "bikes", "label for log lines")
	rate := fs.Float64("rate", 4, "ride events per second")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "es-bikes")
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	solace.Logf(*role, "%.0f ride events a second across %d bikes, guaranteed, numbered.", *rate, bikes)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	riding := map[int]bool{}
	// Numbers start at 1 on every start of the bikes; the run tells one
	// start's #12 from another's.
	run := time.Now().Format("150405")
	tick := time.NewTicker(time.Duration(float64(time.Second) / *rate))
	defer tick.Stop()
	for seq := 1; ; seq++ {
		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d events.", seq-1)
			return
		case <-tick.C:
		}
		// A start for a bike at rest, or an end for one riding, about half
		// and half once enough are out.
		bike := 1 + rand.Intn(bikes)
		kind := "started"
		if riding[bike] {
			kind = "ended"
			delete(riding, bike)
		} else {
			riding[bike] = true
		}
		id := fmt.Sprintf("B-%02d", bike)
		payload := fmt.Sprintf(`{"run":"%s","seq":%d,"bike":"%s","event":"%s","inUse":%d}`, run, seq, id, kind, len(riding))
		if err := solace.PublishKeyed(svc, pub, fmt.Sprintf("es/bikes/ride/%s/v1/%s", kind, id), payload, ""); err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		}
		if seq%40 == 0 {
			solace.Logf(*role, "#%d published, %d bikes in use", seq, len(riding))
		}
	}
}

// tally is the count of bikes in use, kept as the set of bikes out.
type tally struct {
	out  map[string]bool
	seq  int // last event applied
	odd  int // ends without a start
	role string
	why  string // what an end without a start means for this app
}

func (t *tally) apply(payload string) (note string) {
	bike := solace.FieldString(payload, "bike")
	t.seq, _ = solace.FieldInt(payload, "seq")
	if solace.FieldString(payload, "event") == "started" {
		t.out[bike] = true
		return ""
	}
	if !t.out[bike] {
		t.odd++
		return fmt.Sprintf("  end without a start for %s: %s", bike, t.why)
	}
	delete(t.out, bike)
	return ""
}

func (t *tally) report(published int) {
	fmt.Printf("[%s] in use: %d (at #%d; the bikes say %d)\n", t.role, len(t.out), t.seq, published)
}

// Counter reads the stream from its queue and keeps the count, with a
// snapshot of the bikes out and the last event applied, written before each
// acknowledgement, so a restart carries on with the right count from the
// right position. A crash after the snapshot but before the acknowledgement
// brings that event back; the snapshot already has it, so it is skipped.
func Counter(args []string) {
	fs := flag.NewFlagSet("es counter", flag.ExitOnError)
	user := fs.String("user", "svc-es-counter", "client username to connect as")
	role := fs.String("role", "counter", "label for log lines")
	clientName := fs.String("client-name", "es-counter", "client name the broker reports this app by")
	queue := fs.String("queue", "q.es.rides", "the stream's queue")
	noSnapshot := fs.Bool("no-snapshot", false, "keep the count in memory only")
	forget := fs.Bool("forget", false, "delete the snapshot, and exit")
	// For testing recovery only: exit as if killed once the snapshot for
	// event n is written, before n is acknowledged.
	crashAfter := fs.Int("crash-after-snapshot", 0, "test only: exit right after the snapshot for this event")
	fs.Parse(args)

	dir := filepath.Join(solace.Env("COCKPIT_STATE_DIR", "."), "event-streaming")
	_ = os.MkdirAll(dir, 0o755)
	snap := filepath.Join(dir, "counter.snapshot")
	if *forget {
		if err := os.Remove(snap); err != nil && !os.IsNotExist(err) {
			solace.Errf(*role, "could not delete %s: %s", snap, err)
			os.Exit(1)
		}
		solace.Logf(*role, "deleted the snapshot.")
		return
	}

	t := &tally{out: map[string]bool{}, role: *role,
		why: "the count is wrong: this app did not see the start"}
	// The snapshot is "at <run> <seq>" and the bikes out: the state, and
	// the position in the stream it was taken at.
	snapRun, snapSeq := "", 0
	if !*noSnapshot {
		if b, err := os.ReadFile(snap); err == nil {
			f := strings.Fields(string(b))
			if len(f) >= 3 && f[0] == "at" {
				snapRun = f[1]
				snapSeq, _ = strconv.Atoi(f[2])
				f = f[3:]
			}
			for _, bike := range f {
				t.out[bike] = true
			}
			t.seq = snapSeq
			solace.Logf(*role, "restored %d rides in use from the snapshot, taken at #%d.", len(t.out), snapSeq)
		}
	}

	published := 0
	held := solace.NewHolding("Bikes in use, built from the stream", "bike")
	// Called with the tally not changing under it.
	report := func() {
		ids := make([]string, 0, len(t.out))
		for b := range t.out {
			ids = append(ids, b)
		}
		sort.Strings(ids)
		rows := make([][]string, len(ids))
		for i, b := range ids {
			rows[i] = []string{b}
		}
		note := "No ride events applied yet."
		if t.seq > 0 {
			note = fmt.Sprintf("%d bikes in use, at event #%d.", len(ids), t.seq)
			if published > 0 {
				note += fmt.Sprintf(" The bikes say %d.", published)
			}
		}
		held.Set(rows, note)
	}
	report()

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)
	if *noSnapshot {
		solace.Logf(*role, "counting from %s with NO snapshot: a restart starts the count from zero.", *queue)
	} else {
		solace.Logf(*role, "counting from %s, snapshot before each acknowledgement.", *queue)
	}
	fmt.Println()

	var mu sync.Mutex
	first := true
	run := snapRun
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		payload := solace.PayloadOf(msg)
		mu.Lock()
		if first {
			first = false
			seq, _ := solace.FieldInt(payload, "seq")
			if msg.IsRedelivered() {
				solace.Logf(*role, "resumed at #%d, delivered again after the last stop", seq)
			} else {
				solace.Logf(*role, "resumed at #%d", seq)
			}
		}
		if seq, _ := solace.FieldInt(payload, "seq"); solace.FieldString(payload, "run") == snapRun && seq <= snapSeq {
			mu.Unlock()
			solace.Logf(*role, "#%d is already in the snapshot: skipped", seq)
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(*role, "could not acknowledge: %s", err)
			}
			return
		}
		// The bikes number a new stream from #1 with every bike at rest; a
		// count from the old stream does not carry over to it.
		if r := solace.FieldString(payload, "run"); run != "" && r != run {
			solace.Logf(*role, "the bikes started a new stream (run %s): counting from zero", r)
			t.out = map[string]bool{}
		}
		note := t.apply(payload)
		run = solace.FieldString(payload, "run")
		published, _ = solace.FieldInt(payload, "inUse")
		report()
		if note != "" {
			fmt.Printf("[%s] #%d%s\n", *role, t.seq, note)
		}
		if !*noSnapshot {
			bikesOut := make([]string, 0, len(t.out))
			for b := range t.out {
				bikesOut = append(bikesOut, b)
			}
			sort.Strings(bikesOut)
			if err := solace.WriteFileAtomic(snap, []byte(fmt.Sprintf("at %s %d %s", run, t.seq, strings.Join(bikesOut, " ")))); err != nil {
				solace.Errf(*role, "could not write the snapshot: %s", err)
				mu.Unlock()
				return // not acknowledged: it will come back
			}
			if t.seq == *crashAfter {
				solace.Errf(*role, "test: exiting after the snapshot for #%d, before acknowledging it", t.seq)
				os.Exit(137)
			}
		}
		mu.Unlock()
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge: %s", err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			mu.Lock()
			solace.Logf(*role, "stopped at #%d with %d in use.", t.seq, len(t.out))
			mu.Unlock()
			return
		case <-tick.C:
			mu.Lock()
			if t.seq > 0 {
				t.report(published)
			}
			mu.Unlock()
		}
	}
}

// Board lists the rides in progress on the city's screen: which bikes are
// out and for how long. It takes the events direct: no queue, no position,
// no history, so started mid-stream its list misses every bike that was
// already out, and the bikes' total in each event says how many but not
// which.
func Board(args []string) {
	fs := flag.NewFlagSet("es board", flag.ExitOnError)
	user := fs.String("user", "svc-es-board", "client username to connect as")
	role := fs.String("role", "board", "label for log lines")
	clientName := fs.String("client-name", "es-board", "client name the broker reports this app by")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.StartDirectReceiver(*role, svc)
	defer solace.Terminate(rcv)
	solace.Subscribe(*role, *user, rcv, "es/bikes/ride/>")
	solace.Logf(*role, "listing rides from now, direct: no history.")
	fmt.Println()

	t := &tally{out: map[string]bool{}, role: *role, why: "joined late, no history"}
	var mu sync.Mutex
	published := 0
	since := map[string]time.Time{}
	held := solace.NewHolding("Rides in progress, as the board lists them", "bike", "riding since")
	// Called with mu held.
	report := func() {
		ids := make([]string, 0, len(since))
		for b := range since {
			ids = append(ids, b)
		}
		sort.Strings(ids)
		rows := make([][]string, len(ids))
		for i, b := range ids {
			rows[i] = []string{b, since[b].Format("15:04:05")}
		}
		note := "No ride events received yet."
		if t.seq > 0 {
			note = fmt.Sprintf("%d rides listed, at event #%d. The bikes say %d are out: ", len(ids), t.seq, published)
			if missing := published - len(ids); missing > 0 {
				note += fmt.Sprintf("%d the board cannot name.", missing)
			} else {
				note += "all of them listed."
			}
		}
		held.Set(rows, note)
	}
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		payload := solace.PayloadOf(msg)
		mu.Lock()
		defer mu.Unlock()
		note := t.apply(payload)
		if bike := solace.FieldString(payload, "bike"); t.out[bike] {
			since[bike] = time.Now()
		} else {
			delete(since, bike)
		}
		published, _ = solace.FieldInt(payload, "inUse")
		report()
		if note != "" {
			fmt.Printf("[%s] #%d%s\n", *role, t.seq, note)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			mu.Lock()
			solace.Logf(*role, "stopped: %d rides listed on the board, %d ends without a start.", len(t.out), t.odd)
			mu.Unlock()
			return
		case <-tick.C:
			mu.Lock()
			if t.seq > 0 {
				longest := ""
				for bike, at := range since {
					if longest == "" || at.Before(since[longest]) || (at.Equal(since[longest]) && bike < longest) {
						longest = bike
					}
				}
				line := fmt.Sprintf("%d rides listed", len(since))
				if longest != "" {
					line += fmt.Sprintf(", longest %s for %.0f s", longest, time.Since(since[longest]).Seconds())
				}
				// The total says how many rides the list is missing, never
				// which bikes or since when.
				gap := "all of them listed"
				if missing := published - len(since); missing > 0 {
					gap = fmt.Sprintf("%d the board cannot name", missing)
				}
				fmt.Printf("[%s] %s (at #%d; the bikes say %d are out: %s)\n", *role, line, t.seq, published, gap)
			}
			mu.Unlock()
		}
	}
}
