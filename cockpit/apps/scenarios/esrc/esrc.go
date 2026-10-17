// Package esrc implements the event sourcing scenario.
//
// A bank's ledger keeps no balances table. Every deposit and withdrawal is an
// event, and the balances are whatever those events add up to. The ledger
// publishes each event to the broker, which keeps a copy in its replay log,
// and applies it when it comes back on its own queue. When the ledger starts,
// it asks the broker to replay every logged event into its queue, and
// rebuilds the balances from them before it takes the next transaction.
//
// Each event also records the balance it left behind, which the ledger
// checks as it applies it: if events are missing, the totals stop matching.
package esrc

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

const (
	accountTopic   = "esrc/bank/account/"
	statementTopic = "esrc/bank/statement/v1/"
	accounts       = 4
)

func accountID(i int) string { return fmt.Sprintf("A-%d", i+1) }

// Ledger rebuilds the balances from the replay log, then makes a
// transaction every --every, publishing it as an event.
func Ledger(args []string) {
	fs := flag.NewFlagSet("esrc ledger", flag.ExitOnError)
	user := fs.String("user", "svc-esrc-ledger", "client username to connect as")
	role := fs.String("role", "ledger", "label for log lines")
	clientName := fs.String("client-name", "esrc-ledger", "client name the broker reports this app by")
	queue := fs.String("queue", "q.esrc.ledger", "exclusive queue the replay is delivered to")
	every := fs.Duration("every", time.Second, "pause between transactions")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	rcv, err := svc.CreatePersistentMessageReceiverBuilder().
		WithMissingResourcesCreationStrategy(config.PersistentReceiverDoNotCreateMissingResources).
		WithMessageClientAcknowledgement().
		WithMessageReplay(config.ReplayStrategyAllMessages()).
		Build(resource.QueueDurableExclusive(*queue))
	if err == nil {
		err = rcv.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not bind to %s with replay: %s", *queue, err)
		os.Exit(1)
	}
	defer solace.Terminate(rcv)
	solace.Logf(*role, "bound to %s and asked the broker to replay every logged event", *queue)
	solace.Logf(*role, "no balances yet: rebuilding them from the events")
	fmt.Println()

	var mu sync.Mutex
	balance := map[string]int{}
	seq, first, applied, mismatches := 0, 0, 0, 0
	last := time.Now()
	rebuilding := true
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		p := solace.PayloadOf(msg)
		n, _ := solace.FieldInt(p, "seq")
		acct := solace.FieldString(p, "account")
		kind := solace.FieldString(p, "type")
		amount, _ := solace.FieldInt(p, "amount")
		recorded, _ := solace.FieldInt(p, "balance")
		mu.Lock()
		if kind == "withdrawn" {
			balance[acct] -= amount
		} else {
			balance[acct] += amount
		}
		if n > seq {
			seq = n
		}
		if first == 0 || n < first {
			first = n
		}
		applied++
		last = time.Now()
		now := balance[acct]
		quiet := rebuilding
		if now != recorded {
			mismatches++
			balance[acct] = recorded
		}
		mu.Unlock()
		if now != recorded {
			solace.Errf(*role, "event #%d says %s has %d.00, the events I have add up to %d.00: earlier events are missing", n, acct, recorded, now)
		} else if !quiet {
			solace.Logf(*role, "event #%d applied: %s %s %d.00, balance %d.00", n, acct, kind, amount, now)
		}
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge event #%d: %s", n, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	// The replay arrives first. Once nothing has come for a moment, it is
	// over: the ledger is the only publisher, and it has not started yet.
	for {
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		done := time.Since(last) > 1500*time.Millisecond
		mu.Unlock()
		if done {
			break
		}
	}
	mu.Lock()
	rebuilding = false
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
	n, bad, from := applied, mismatches, first
	mu.Unlock()
	if n == 0 {
		solace.Logf(*role, "rebuilt from 0 events: no transactions in the log, every account starts at 0.00")
	} else if bad == 0 {
		solace.Logf(*role, "rebuilt from %d events: %s", n, summary)
		solace.Logf(*role, "every event's recorded balance matched the events before it")
	} else {
		solace.Errf(*role, "rebuilt from %d events, and %d of them did not match the events before them: %s", n, bad, summary)
	}
	if from > 1 {
		solace.Errf(*role, "the log starts at event #%d: events #1 to #%d are no longer in it", from, from-1)
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for {
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
		acct := accountID(rand.Intn(accounts))
		amount := 5 * (1 + rand.Intn(20))
		mu.Lock()
		have := balance[acct]
		seq++
		n := seq
		mu.Unlock()
		kind, after := "deposited", have+amount
		if rand.Intn(2) == 0 {
			if amount > have {
				solace.Logf(*role, "withdrawal of %d.00 from %s refused: balance %d.00", amount, acct, have)
				mu.Lock()
				seq--
				mu.Unlock()
				continue
			}
			kind, after = "withdrawn", have-amount
		}
		payload := fmt.Sprintf(`{"seq":%d,"account":"%s","type":"%s","amount":%d,"balance":%d}`, n, acct, kind, amount, after)
		if err := solace.PublishConfirmed(svc, pub, accountTopic+acct+"/"+kind+"/v1", payload); err != nil {
			solace.Errf(*role, "event #%d not stored: %s", n, err)
		}
	}
}

// Statements publishes --count monthly statements of --size bytes each, on
// topics the replay log keeps, then exits. They are large, and they push the
// oldest events out of a small replay log.
func Statements(args []string) {
	fs := flag.NewFlagSet("esrc statements", flag.ExitOnError)
	user := fs.String("user", "svc-esrc-ledger", "client username to connect as")
	role := fs.String("role", "statements", "label for log lines")
	clientName := fs.String("client-name", "esrc-statements", "client name the broker reports this app by")
	count := fs.Int("count", 45, "statements to publish")
	size := fs.Int("size", 30000, "bytes per statement")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	pdf := strings.Repeat("x", *size)
	for i := 1; i <= *count; i++ {
		acct := accountID(i % accounts)
		if err := solace.PublishConfirmed(svc, pub, statementTopic+acct, fmt.Sprintf(`{"account":"%s","pdf":"%s"}`, acct, pdf)); err != nil {
			solace.Errf(*role, "statement %d not stored: %s", i, err)
		}
	}
	solace.Logf(*role, "published %d monthly statements of %d KB each (%d KB in all)", *count, *size/1000, *count**size/1000)
}
