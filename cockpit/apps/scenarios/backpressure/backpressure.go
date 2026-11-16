// Package backpressure implements the backpressure and push scenario.
//
// Cold-chain monitoring in a refrigerated warehouse. Sensors report 40
// temperature readings a second. A reporter writes every reading to a
// compliance database, which slows down at times, and must not lose any. A
// live display on the warehouse wall shows the current temperatures, and only
// cares about the latest.
//
// The broker pushes messages to its consumers, so each needs a way to say
// "not so fast". The reporter reads a queue: it pauses its own intake when too
// many writes are pending, and the broker never sends it more than the
// queue's unacknowledged limit, so the rest waits on the queue. The display
// takes the readings directly: nothing waits for it anywhere, and when it
// falls behind the API's buffer drops readings.
package backpressure

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/metrics"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// slowOnSignals makes SIGUSR1 switch a delay to slow and SIGUSR2 back to
// normal, logging each change. Caught before connecting, so an early signal
// does not take its default action, which is to exit.
func slowOnSignals(role, what string, normal, slow time.Duration) *atomic.Int64 {
	var d atomic.Int64
	d.Store(int64(normal))
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range ch {
			if s == syscall.SIGUSR1 {
				d.Store(int64(slow))
				solace.Logf(role, "%s is slow now: %s each", what, slow)
			} else {
				d.Store(int64(normal))
				solace.Logf(role, "%s is back to normal: %s each", what, normal)
			}
		}
	}()
	return &d
}

// Sensors publish temperature readings as direct messages: no
// acknowledgement, no storage on the way. The queue's subscription is what
// stores a copy for the reporter.
func Sensors(args []string) {
	fs := flag.NewFlagSet("bp sensors", flag.ExitOnError)
	user := fs.String("user", "svc-bp-sensors", "client username to connect as")
	role := fs.String("role", "sensors", "label for log lines")
	rate := fs.Int("rate", 40, "readings per second")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, "bp-sensors")
	defer svc.Disconnect()
	pub := solace.StartDirectPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	solace.Logf(*role, "%d readings a second, published direct.", *rate)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(time.Second / time.Duration(*rate))
	defer tick.Stop()
	for seq := 1; ; seq++ {
		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d readings.", seq-1)
			return
		case <-tick.C:
		}
		zone, sensor := 1+rand.Intn(3), 1+rand.Intn(4)
		payload := fmt.Sprintf(`{"seq":%d,"zone":"z%d","sensor":"s%d","celsius":"%.1f","at":%d}`,
			seq, zone, sensor, 2+rand.Float64()*3, time.Now().UnixMilli())
		msg, err := svc.MessageBuilder().BuildWithStringPayload(payload)
		if err == nil {
			err = pub.Publish(msg, resource.TopicOf(fmt.Sprintf("bp/coldchain/temperature/v1/z%d/s%d", zone, sensor)))
		}
		if err != nil {
			solace.Errf(*role, "publish failed: %s", err)
		}
		if seq%400 == 0 {
			solace.Logf(*role, "%d readings sent", seq)
		}
	}
}

// Reporter writes every reading to the compliance database, then
// acknowledges it. Writes queue up inside it; above --high pending it pauses
// the receiver, below --low it resumes. SIGUSR1 makes the database slow,
// SIGUSR2 normal again.
func Reporter(args []string) {
	fs := flag.NewFlagSet("bp reporter", flag.ExitOnError)
	user := fs.String("user", "svc-bp-reporter", "client username to connect as")
	role := fs.String("role", "reporter", "label for log lines")
	clientName := fs.String("client-name", "bp-reporter", "client name the broker reports this app by")
	queue := fs.String("queue", "q.bp.readings", "queue of readings")
	high := fs.Int("high", 50, "pause when this many writes are pending")
	low := fs.Int("low", 10, "resume when pending writes fall to this")
	fs.Parse(args)

	writeTime := slowOnSignals(*role, "the database", 10*time.Millisecond, 100*time.Millisecond)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "writing every reading from %s; pausing at %d pending writes, resuming at %d.", *queue, *high, *low)
	fmt.Println()

	writes := make(chan message.InboundMessage, 10000)
	var mu sync.Mutex
	pending, written, lastSeq := 0, 0, 0
	paused, behind := false, false
	var pauseDone chan struct{} // closed once the latest Pause has returned

	go func() {
		for msg := range writes {
			time.Sleep(time.Duration(writeTime.Load()))
			payload := solace.PayloadOf(msg)
			seq, _ := solace.FieldInt(payload, "seq")
			at, _ := solace.FieldInt(payload, "at")
			age := time.Since(time.UnixMilli(int64(at)))
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(*role, "could not acknowledge reading #%d: %s", seq, err)
			}
			mu.Lock()
			pending--
			written++
			note := ""
			if lastSeq > 0 && seq > lastSeq+1 {
				note = fmt.Sprintf("  GAP: #%d after #%d", seq, lastSeq)
			}
			if seq > lastSeq {
				lastSeq = seq
			}
			if note != "" {
				fmt.Printf("[%s] wrote reading #%d%s\n", *role, seq, note)
			}
			if paused && pending <= *low {
				paused = false
				// Resume only after the Pause has happened, or a late Pause
				// would leave the receiver paused for good.
				done := pauseDone
				mu.Unlock()
				<-done
				err := rcv.Resume()
				mu.Lock()
				if err != nil {
					solace.Errf(*role, "could not resume: %s", err)
				}
				solace.Logf(*role, "resumed with %d writes pending; the queue sends the readings that waited", pending)
			}
			// After a pause, the backlog is gone once the readings being
			// written are fresh again.
			if behind && !paused && age < time.Second {
				behind = false
				solace.Logf(*role, "caught up: writing readings less than a second old")
			}
			if written%200 == 0 {
				solace.Logf(*role, "%d readings written, the latest %.1f s old, %d pending", written, age.Seconds(), pending)
			}
			mu.Unlock()
		}
	}()

	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		mu.Lock()
		pending++
		if !paused && pending >= *high {
			paused, behind = true, true
			// Pause from outside the handler: the API delivers no more to it.
			done := make(chan struct{})
			pauseDone = done
			go func() {
				defer close(done)
				if err := rcv.Pause(); err != nil {
					solace.Errf(*role, "could not pause: %s", err)
				}
			}()
			solace.Logf(*role, "paused with %d writes pending: taking no more readings until the database catches up", pending)
		}
		mu.Unlock()
		writes <- msg
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	mu.Lock()
	defer mu.Unlock()
	fmt.Println()
	solace.Logf(*role, "stopped after writing %d readings.", written)
}

// Display shows the latest temperatures. It takes readings directly, with
// the API buffering at most --buffer of them; when it falls behind, the API
// drops the oldest (or with --drop latest, the newest). SIGUSR1 makes
// rendering slow, SIGUSR2 normal again.
func Display(args []string) {
	fs := flag.NewFlagSet("bp display", flag.ExitOnError)
	user := fs.String("user", "svc-bp-display", "client username to connect as")
	role := fs.String("role", "display", "label for log lines")
	clientName := fs.String("client-name", "bp-display", "client name the broker reports this app by")
	buffer := fs.Uint("buffer", 20, "readings the API holds for the display before it drops some")
	drop := fs.String("drop", "oldest", "which readings to drop when the buffer is full: oldest or latest")
	fs.Parse(args)

	renderTime := slowOnSignals(*role, "rendering", 10*time.Millisecond, 250*time.Millisecond)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	b := svc.CreateDirectMessageReceiverBuilder().
		WithSubscriptions(resource.TopicSubscriptionOf("bp/coldchain/temperature/v1/>"))
	which := "oldest first"
	if *drop == "latest" {
		b = b.OnBackPressureDropLatest(*buffer)
		which = "newest first"
	} else {
		b = b.OnBackPressureDropOldest(*buffer)
	}
	rcv, err := b.Build()
	if err == nil {
		err = rcv.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start the receiver: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(rcv)
	solace.Logf(*role, "showing readings as they arrive, direct; the API holds %d and drops the %s when full.", *buffer, *drop)
	fmt.Println()

	var mu sync.Mutex
	var lastSeq int
	var lastAge time.Duration
	var lastText string
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(time.Duration(renderTime.Load()))
		payload := solace.PayloadOf(msg)
		seq, _ := solace.FieldInt(payload, "seq")
		at, _ := solace.FieldInt(payload, "at")
		mu.Lock()
		lastSeq, lastAge = seq, time.Since(time.UnixMilli(int64(at)))
		lastText = fmt.Sprintf("%s %s %s °C", solace.FieldString(payload, "zone"),
			solace.FieldString(payload, "sensor"), solace.FieldString(payload, "celsius"))
		mu.Unlock()
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	var dropped uint64
	for {
		select {
		case <-stop:
			solace.Logf(*role, "stopped; %d readings dropped in all.", dropped)
			return
		case <-tick.C:
		}
		total := svc.Metrics().GetValue(metrics.ReceivedMessagesBackpressureDiscarded)
		mu.Lock()
		if total > dropped {
			solace.Logf(*role, "dropped %d readings in the last 2 s (%s), %d in all", total-dropped, which, total)
		}
		dropped = total
		if lastSeq > 0 {
			solace.Logf(*role, "showing reading #%d (%s), %.1f s old", lastSeq, lastText, lastAge.Seconds())
		}
		mu.Unlock()
	}
}
