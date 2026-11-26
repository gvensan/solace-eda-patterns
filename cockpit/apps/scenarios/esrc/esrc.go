// Package esrc implements the event sourcing scenario.
//
// A bank's ledger keeps no separate balance record: each balance is that account's events
// added up, held in memory. A teller sends commands ("withdraw 40 from A-2")
// into a queue. The ledger takes one at a time and validates it against the
// balances. An accepted one is recorded as an event: published to the broker,
// whose replay log is the event history here, and confirmed by it. That is
// the only write. Once it is recorded the ledger applies the event to its
// balances, acknowledges the command and takes the next.
//
// At start the ledger has no balances. It asks the broker to replay the
// whole history into a queue of its own and adds the events up, before it
// takes a command. If the history it gets is visibly incomplete, it stops
// rather than decide anything from it. The demo assumes the history is
// complete and durable, as an event store's is; the replay log stands in
// for one.
//
// Each event names its command, so a command redelivered after a crash is
// recognised from the rebuilt history and not recorded twice.
package esrc

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

const (
	eventTopic   = "esrc/bank/account/"
	commandTopic = "esrc/bank/command/"
	// A marker the ledger sends itself to learn the replay is over. The
	// replay log keeps these too, so a marker sent while the replay runs
	// is logged after the replayed events and delivered after them.
	markerTopic = "esrc/bank/marker/"
	accounts    = 4
)

func accountID(i int) string { return fmt.Sprintf("A-%d", i+1) }

// Teller sends a command every --every: pay an amount in or take one out.
// It decides nothing; the ledger does.
func Teller(args []string) {
	fs := flag.NewFlagSet("esrc teller", flag.ExitOnError)
	user := fs.String("user", "svc-esrc-teller", "client username to connect as")
	role := fs.String("role", "teller", "label for log lines")
	clientName := fs.String("client-name", "esrc-teller", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between commands")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	// Command ids start with the time the teller started, so a restarted
	// teller never reuses one the ledger has already recorded.
	start := time.Now().Format("150405")
	solace.Logf(*role, "sending a command every %s", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for n := 1; ; n++ {
		id := fmt.Sprintf("%s-%d", start, n)
		acct := accountID(rand.Intn(accounts))
		amount := 5 * (1 + rand.Intn(20))
		action, words := "deposit", fmt.Sprintf("pay %d.00 into %s", amount, acct)
		if rand.Intn(2) == 0 {
			action, words = "withdraw", fmt.Sprintf("take %d.00 out of %s", amount, acct)
		}
		payload := fmt.Sprintf(`{"command":"%s","account":"%s","action":"%s","amount":%d}`, id, acct, action, amount)
		if err := solace.PublishConfirmed(svc, pub, commandTopic+acct+"/"+action+"/v1", payload); err != nil {
			solace.Errf(*role, "command %s not sent: %s", id, err)
		} else {
			solace.Logf(*role, "command %s: %s", id, words)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// event is one recorded fact, as published and as replayed.
type event struct {
	seq, version, amount, balance int
	command, account, kind        string
}

func parseEvent(p string) event {
	var e event
	e.seq, _ = solace.FieldInt(p, "seq")
	e.version, _ = solace.FieldInt(p, "version")
	e.amount, _ = solace.FieldInt(p, "amount")
	e.balance, _ = solace.FieldInt(p, "balance")
	e.command = solace.FieldString(p, "command")
	e.account = solace.FieldString(p, "account")
	e.kind = solace.FieldString(p, "type")
	return e
}

// Ledger rebuilds the balances from the event history, then takes the
// teller's commands one at a time, recording each accepted one as an event
// and applying it.
func Ledger(args []string) {
	fs := flag.NewFlagSet("esrc ledger", flag.ExitOnError)
	user := fs.String("user", "svc-esrc-ledger", "client username to connect as")
	role := fs.String("role", "ledger", "label for log lines")
	clientName := fs.String("client-name", "esrc-ledger", "client name the broker reports this app by")
	commands := fs.String("commands", "q.esrc.commands", "queue the teller's commands wait in")
	rebuildQ := fs.String("rebuild", "q.esrc.rebuild", "queue the history is replayed into at start")
	stall := fs.Duration("stall", 30*time.Second, "give up on a rebuild that receives nothing for this long")
	// For testing recovery only: exit as if killed once event n is recorded,
	// before its command is acknowledged.
	crashAfter := fs.Int("crash-after-record", 0, "test only: exit right after recording this event")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	var mu sync.Mutex
	balance := map[string]int{}
	version := map[string]int{}
	lastEvent := map[string]int{}
	recorded := map[string]int{} // command id -> the event that recorded it
	seq, applied, rebuiltFrom := 0, 0, 0
	rebuilding := true

	held := solace.NewHolding("Balances, worked out from the events", "account", "balance", "last event")
	// Called with mu held.
	report := func() {
		rows := make([][]string, 0, accounts)
		for i := 0; i < accounts; i++ {
			a := accountID(i)
			ev := "none"
			if lastEvent[a] > 0 {
				ev = fmt.Sprintf("#%d", lastEvent[a])
			}
			rows = append(rows, []string{a, fmt.Sprintf("%d.00", balance[a]), ev})
		}
		var note string
		switch {
		case rebuilding:
			note = fmt.Sprintf("Rebuilding from the event history: %d events added up so far. Commands wait until it is done.", applied)
		case rebuiltFrom == 0:
			note = fmt.Sprintf("The history was empty, so every account started at 0.00. %d events recorded since.", applied)
		default:
			note = fmt.Sprintf("Rebuilt at start from %d events, %d recorded since. There is no separate balance record.", rebuiltFrom, applied-rebuiltFrom)
		}
		held.Set(rows, note)
	}
	// apply adds one event to the balances. Called with mu held; returns
	// what is wrong with it, if anything: events are applied in order, and
	// each carries the balance it left, which must match the sum. The
	// balance is always the sum: the carried one is a check, never a
	// snapshot to fall back on.
	apply := func(e event) string {
		now := balance[e.account] + e.amount
		if e.kind == "withdrawn" {
			now = balance[e.account] - e.amount
		}
		var wrong string
		switch {
		case e.seq != seq+1 && seq == 0:
			wrong = fmt.Sprintf("the history starts at event #%d, not #1: the events before it are missing", e.seq)
		case e.seq != seq+1:
			wrong = fmt.Sprintf("event #%d came after #%d: the history is not complete", e.seq, seq)
		case now != e.balance:
			wrong = fmt.Sprintf("event #%d says %s has %d.00, but the events before it add up to %d.00", e.seq, e.account, e.balance, now)
		}
		balance[e.account] = now
		version[e.account] = e.version
		lastEvent[e.account] = e.seq
		recorded[e.command] = e.seq
		seq = e.seq
		applied++
		report()
		return wrong
	}
	mu.Lock()
	report()
	mu.Unlock()

	// 1. Rebuild: replay the whole history into q.esrc.rebuild, a durable
	// exclusive queue (the broker refuses replay into a temporary one).
	marker := fmt.Sprintf("%s%s/%d", markerTopic, *clientName, time.Now().UnixNano())
	// The broker replays the logged events that match the queue's
	// subscriptions into it, replacing whatever it held. So the queue
	// subscribes to the events for the rebuild only: the ledger owns it,
	// adds the subscription now and removes it once rebuilt, and no live
	// event lands in it in between.
	events := resource.TopicSubscriptionOf(eventTopic + ">")
	prep, err := svc.CreatePersistentMessageReceiverBuilder().
		WithMissingResourcesCreationStrategy(config.PersistentReceiverDoNotCreateMissingResources).
		WithMessageClientAcknowledgement().
		Build(resource.QueueDurableExclusive(*rebuildQ))
	if err == nil {
		err = prep.Start()
	}
	if err == nil {
		// Left behind by a ledger killed mid-rebuild, it is already there.
		if e := prep.AddSubscription(events); e != nil && !strings.Contains(strings.ToLower(e.Error()), "already") {
			err = e
		}
		_ = prep.Terminate(time.Second)
	}
	if err != nil {
		solace.Errf(*role, "could not subscribe %s to the events: %s", *rebuildQ, err)
		os.Exit(1)
	}
	replay, err := svc.CreatePersistentMessageReceiverBuilder().
		WithMissingResourcesCreationStrategy(config.PersistentReceiverDoNotCreateMissingResources).
		WithMessageClientAcknowledgement().
		WithMessageReplay(config.ReplayStrategyAllMessages()).
		Build(resource.QueueDurableExclusive(*rebuildQ))
	if err == nil {
		err = replay.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start the replay of the event history: %s", err)
		os.Exit(1)
	}
	solace.Logf(*role, "asked the broker to replay the event history into %s", *rebuildQ)
	solace.Logf(*role, "no balances yet: rebuilding them from the events, no command taken until done")
	fmt.Println()

	over := make(chan struct{})
	var once sync.Once
	var heard atomic.Int64
	heard.Store(time.Now().UnixNano())
	if err := replay.ReceiveAsync(func(msg message.InboundMessage) {
		heard.Store(time.Now().UnixNano())
		p := solace.PayloadOf(msg)
		// Markers from earlier starts are in the log too: only this
		// start's own marker ends the rebuild.
		if dest := msg.GetDestinationName(); strings.HasPrefix(dest, markerTopic) {
			_ = replay.Ack(msg)
			if dest == marker {
				once.Do(func() { close(over) })
			}
			return
		}
		mu.Lock()
		wrong := apply(parseEvent(p))
		mu.Unlock()
		if wrong != "" {
			// Balances added up from a history with a hole in it are
			// wrong, and any command decided from them would be too.
			solace.Errf(*role, "%s", wrong)
			solace.Errf(*role, "stopping: no command is taken from an incomplete history")
			os.Exit(1)
		}
		_ = replay.Ack(msg)
	}); err != nil {
		solace.Errf(*role, "could not register the replay handler: %s", err)
		os.Exit(1)
	}
	// The marker is logged after every event in the history, so the
	// replay delivers it last: then the rebuild is complete. Sent while
	// the replay is still running, it reaches the queue only from the log
	// (docs/spikes.md, section 23), which is why the log keeps markers.
	if err := solace.PublishConfirmed(svc, pub, marker, `{"marker":true}`); err != nil {
		solace.Errf(*role, "could not send the end-of-replay marker: %s", err)
		os.Exit(1)
	}
	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	// Nothing arriving for a long while, and no marker, means the rebuild
	// cannot finish: say so rather than wait for ever.
	quiet := time.NewTicker(5 * time.Second)
	defer quiet.Stop()
wait:
	for {
		select {
		case <-over:
			break wait
		case <-stop:
			return
		case <-quiet.C:
			if time.Since(time.Unix(0, heard.Load())) > *stall {
				solace.Errf(*role, "rebuild stalled: nothing from %s for %s and no end-of-replay marker", *rebuildQ, *stall)
				solace.Errf(*role, "stopping: no command is taken until the rebuild completes")
				os.Exit(1)
			}
		}
	}
	if err := replay.RemoveSubscription(events); err != nil {
		solace.Errf(*role, "could not remove the events subscription from %s: %s", *rebuildQ, err)
	}
	solace.Terminate(replay)

	mu.Lock()
	rebuilding = false
	rebuiltFrom = applied
	report()
	keys := make([]string, 0, len(balance))
	for k := range balance {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d.00", k, balance[k]))
	}
	summary := strings.Join(parts, ", ")
	n := applied
	mu.Unlock()
	if n == 0 {
		solace.Logf(*role, "rebuilt from 0 events: the history is empty, every account starts at 0.00")
	} else {
		solace.Logf(*role, "rebuilt from %d events: %s", n, summary)
	}

	// 2. Commands, one at a time, only now.
	cmds := solace.BindQueue(*role, *user, svc, *commands, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(cmds)
	solace.Logf(*role, "taking commands from %s, one at a time", *commands)
	fmt.Println()

	for {
		select {
		case <-stop:
			return
		default:
		}
		msg, err := cmds.ReceiveMessage(500 * time.Millisecond)
		if err != nil || msg == nil {
			continue
		}
		p := solace.PayloadOf(msg)
		id := solace.FieldString(p, "command")
		acct := solace.FieldString(p, "account")
		action := solace.FieldString(p, "action")
		amount, _ := solace.FieldInt(p, "amount")

		mu.Lock()
		ev, seen := recorded[id]
		have := balance[acct]
		next, v := seq+1, version[acct]+1
		mu.Unlock()
		// Recorded before a crash, but never acknowledged: the broker sent
		// it again. Its event is in the history, so it is done already.
		if seen {
			solace.Logf(*role, "command %s was already recorded as event #%d before the restart: not recorded twice", id, ev)
			_ = cmds.Ack(msg)
			continue
		}
		if action == "withdraw" && amount > have {
			solace.Logf(*role, "command %s refused: take %d.00 out of %s, balance %d.00. Nothing is recorded", id, amount, acct, have)
			_ = cmds.Ack(msg)
			continue
		}
		kind, after := "deposited", have+amount
		if action == "withdraw" {
			kind, after = "withdrawn", have-amount
		}
		e := event{seq: next, version: v, amount: amount, balance: after, command: id, account: acct, kind: kind}
		payload := fmt.Sprintf(`{"seq":%d,"command":"%s","account":"%s","version":%d,"type":"%s","amount":%d,"balance":%d}`,
			e.seq, e.command, e.account, e.version, e.kind, e.amount, e.balance)
		if err := solace.PublishConfirmed(svc, pub, eventTopic+acct+"/"+kind+"/v1", payload); err != nil {
			// The broker may or may not have it: carrying on could record a
			// second event with this number. A restart rebuilds from the
			// history and finds out; the command, never acknowledged, comes
			// again and is recognised if it was recorded.
			solace.Errf(*role, "event #%d for command %s: outcome unknown: %s", e.seq, id, err)
			solace.Errf(*role, "stopping, so a restart rebuilds from the history and finds out whether it was recorded")
			os.Exit(1)
		}
		if e.seq == *crashAfter {
			solace.Errf(*role, "test: exiting after recording event #%d, before acknowledging command %s", e.seq, id)
			os.Exit(137)
		}
		// Recorded, so now applied: the balance changes only this way.
		mu.Lock()
		_ = apply(e)
		mu.Unlock()
		solace.Logf(*role, "command %s accepted: recorded event #%d, %s %s %d.00, balance %d.00", id, e.seq, acct, kind, amount, after)
		if err := cmds.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge command %s: %s", id, err)
		}
	}
}
