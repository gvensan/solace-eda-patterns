// Package streaming implements the event streaming scenario.
//
// A bike-share. Every ride start and ride end is an event, and the counter
// keeps the number of bikes in use: plus one on a start, minus one on an
// end. That number is only right if the counter has seen the stream in
// order from where its count started. A queue keeps the stream in order
// and holds the counter's position in it; a snapshot keeps the count
// across a crash. The city's board takes the same events direct, with
// neither, and joining late it has no history.
package streaming

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
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
		payload := fmt.Sprintf(`{"seq":%d,"bike":"%s","event":"%s","inUse":%d}`, seq, id, kind, len(riding))
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
// snapshot of the bikes out written before each acknowledgement, so a
// restart carries on with the right count from the right position.
func Counter(args []string) {
	fs := flag.NewFlagSet("es counter", flag.ExitOnError)
	user := fs.String("user", "svc-es-counter", "client username to connect as")
	role := fs.String("role", "counter", "label for log lines")
	clientName := fs.String("client-name", "es-counter", "client name the broker reports this app by")
	queue := fs.String("queue", "q.es.rides", "the stream's queue")
	noSnapshot := fs.Bool("no-snapshot", false, "keep the count in memory only")
	forget := fs.Bool("forget", false, "delete the snapshot, and exit")
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
	if !*noSnapshot {
		if b, err := os.ReadFile(snap); err == nil {
			for _, bike := range strings.Fields(string(b)) {
				t.out[bike] = true
			}
			solace.Logf(*role, "restored %d rides in use from the snapshot.", len(t.out))
		}
	}

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
	published, first := 0, true
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
		note := t.apply(payload)
		published, _ = solace.FieldInt(payload, "inUse")
		if note != "" {
			fmt.Printf("[%s] #%d%s\n", *role, t.seq, note)
		}
		if !*noSnapshot {
			bikesOut := make([]string, 0, len(t.out))
			for b := range t.out {
				bikesOut = append(bikesOut, b)
			}
			sort.Strings(bikesOut)
			if err := os.WriteFile(snap, []byte(strings.Join(bikesOut, " ")), 0o644); err != nil {
				solace.Errf(*role, "could not write the snapshot: %s", err)
				mu.Unlock()
				return // not acknowledged: it will come back
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

// Board shows the count on the city's screen. It takes the events direct:
// no queue, no position, no history, so started mid-stream it is wrong
// from the first end it sees for a bike it never saw start.
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
	solace.Logf(*role, "counting from now, direct: no history.")
	fmt.Println()

	t := &tally{out: map[string]bool{}, role: *role, why: "joined late, no history"}
	var mu sync.Mutex
	published := 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		payload := solace.PayloadOf(msg)
		mu.Lock()
		defer mu.Unlock()
		note := t.apply(payload)
		published, _ = solace.FieldInt(payload, "inUse")
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
			solace.Logf(*role, "stopped: %d in use on the board, %d ends without a start.", len(t.out), t.odd)
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
