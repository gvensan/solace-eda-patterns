// Package retry implements the discard, pause and retry scenario.
//
// A payment processor takes payment requests from a queue and charges each
// one at the bank, by request-reply. Three things go wrong, and each gets a
// different answer:
//
//   - A payment that can never succeed (it is malformed) is discarded: the
//     processor rejects it, and the broker moves it to the dead message
//     queue, out of the way of the rest.
//   - A payment the bank cannot take right now ("busy") is retried later:
//     the processor republishes it to a retry queue whose messages wait 5
//     seconds before they can be delivered, and acknowledges the original.
//   - When the bank does not answer at all, no payment can succeed, so the
//     processor pauses: it stops taking payments, checks the bank every 2
//     seconds, and resumes when it answers. The payments wait on the queue.
//
// The broker's own delayed redelivery needs local transactions, which the Go
// API does not have, so the delay is a queue's delivery delay
// (docs/spikes.md, section 5).
package retry

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

const (
	paymentTopic = "dpr/pay/payment/requested/v1/"
	retryTopic   = "dpr/pay/payment/retry/v1/"
	bankTopic    = "dpr/bank/charge/v1/"
	maxAttempts  = 3
)

// Payments publishes a payment request every --every. --bad sends malformed
// ones instead (an amount that is not a number), from a different source.
func Payments(args []string) {
	fs := flag.NewFlagSet("dpr payments", flag.ExitOnError)
	user := fs.String("user", "svc-dpr-shop", "client username to connect as")
	role := fs.String("role", "shop", "label for log lines")
	clientName := fs.String("client-name", "dpr-shop", "client name the broker reports this app by")
	every := fs.Duration("every", time.Second, "pause between payments")
	bad := fs.Bool("bad", false, "send malformed payments")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	prefix := "P"
	if *bad {
		prefix = "X"
		solace.Logf(*role, "sending a malformed payment every %s", *every)
	} else {
		solace.Logf(*role, "sending a payment every %s", *every)
	}
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	for n := 1; ; n++ {
		id := fmt.Sprintf("%s-%04d", prefix, n)
		acct := fmt.Sprintf("A-%d", 1+rand.Intn(5))
		amount := strconv.Itoa(5 + rand.Intn(95))
		if *bad {
			amount = `"twelve"`
		}
		payload := fmt.Sprintf(`{"payment":"%s","account":"%s","amount":%s,"attempt":1}`, id, acct, amount)
		if err := solace.PublishConfirmed(svc, pub, paymentTopic+acct, payload); err != nil {
			solace.Errf(*role, "payment %s refused: %s", id, err)
		} else {
			solace.Logf(*role, "payment %s for %s sent", id, acct)
		}
		select {
		case <-stop:
			return
		case <-time.After(*every):
		}
	}
}

// Bank answers charge requests. --busy-every N answers every Nth with
// "busy, try later".
func Bank(args []string) {
	fs := flag.NewFlagSet("dpr bank", flag.ExitOnError)
	user := fs.String("user", "svc-dpr-bank", "client username to connect as")
	role := fs.String("role", "bank", "label for log lines")
	clientName := fs.String("client-name", "dpr-bank", "client name the broker reports this app by")
	busyEvery := fs.Int("busy-every", 0, "answer every Nth charge with busy (0: never)")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	rcv, err := svc.RequestReply().CreateRequestReplyMessageReceiverBuilder().
		Build(resource.TopicSubscriptionOf(bankTopic + ">"))
	if err == nil {
		err = rcv.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start answering: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(rcv)
	if *busyEvery > 0 {
		solace.Logf(*role, "answering charges, and every %d is answered busy", *busyEvery)
	} else {
		solace.Logf(*role, "answering charges")
	}
	fmt.Println()

	var n atomic.Int64
	if err := rcv.ReceiveAsync(func(req message.InboundMessage, replier sol.Replier) {
		p := solace.PayloadOf(req)
		id := solace.FieldString(p, "payment")
		answer := "charged"
		if *busyEvery > 0 && n.Add(1)%int64(*busyEvery) == 0 {
			answer = "busy"
		}
		reply, err := svc.MessageBuilder().BuildWithStringPayload(fmt.Sprintf(`{"payment":"%s","answer":"%s"}`, id, answer))
		if err == nil {
			err = replier.Reply(reply)
		}
		if err != nil {
			solace.Errf(*role, "could not answer for %s: %s", id, err)
			return
		}
		solace.Logf(*role, "%s: %s", id, answer)
	}); err != nil {
		solace.Errf(*role, "could not register the handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
}

// Processor charges each payment at the bank, and discards, retries or
// pauses as the package comment describes.
func Processor(args []string) {
	fs := flag.NewFlagSet("dpr processor", flag.ExitOnError)
	user := fs.String("user", "svc-dpr-processor", "client username to connect as")
	role := fs.String("role", "processor", "label for log lines")
	clientName := fs.String("client-name", "dpr-processor", "client name the broker reports this app by")
	queue := fs.String("queue", "q.dpr.payments", "payment queue")
	retryQueue := fs.String("retry-queue", "q.dpr.retry", "retry queue, with a delivery delay")
	timeout := fs.Duration("timeout", time.Second, "how long to wait for the bank")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()
	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)
	ask, err := svc.RequestReply().CreateRequestReplyMessagePublisherBuilder().Build()
	if err == nil {
		err = ask.Start()
	}
	if err != nil {
		solace.Errf(*role, "could not start asking the bank: %s", err)
		os.Exit(1)
	}
	defer solace.Terminate(ask)

	opts := solace.QueueOpts{ClientAck: true, Outcomes: []config.MessageSettlementOutcome{config.PersistentReceiverRejectedOutcome}}
	main := solace.BindQueue(*role, *user, svc, *queue, opts)
	defer solace.Terminate(main)
	retries := solace.BindQueue(*role, *user, svc, *retryQueue, opts)
	defer solace.Terminate(retries)
	fmt.Println()

	charge := func(p string) (string, error) {
		q, err := svc.MessageBuilder().BuildWithStringPayload(p)
		if err != nil {
			return "", err
		}
		acct := solace.FieldString(p, "account")
		reply, err := ask.PublishAwaitResponse(q, resource.TopicOf(bankTopic+acct), *timeout, nil)
		if err != nil {
			return "", err
		}
		return solace.FieldString(solace.PayloadOf(reply), "answer"), nil
	}

	stopping := make(chan struct{})
	var paused atomic.Bool
	handle := func(rcv sol.PersistentMessageReceiver) func(message.InboundMessage) {
		return func(msg message.InboundMessage) {
			p := solace.PayloadOf(msg)
			id := solace.FieldString(p, "payment")
			attempt, _ := solace.FieldInt(p, "attempt")
			if _, ok := solace.FieldInt(p, "amount"); !ok {
				solace.Errf(*role, "%s: amount is not a number, can never succeed: discarded to the dead message queue", id)
				if err := rcv.Settle(msg, config.PersistentReceiverRejectedOutcome); err != nil {
					solace.Errf(*role, "could not reject %s: %s", id, err)
				}
				return
			}
			for {
				answer, err := charge(p)
				if err == nil && answer == "charged" {
					if paused.CompareAndSwap(true, false) {
						solace.Logf(*role, "the bank answers again: resuming payments")
						_ = main.Resume()
						_ = retries.Resume()
					}
					solace.Logf(*role, "%s charged (attempt %d)", id, attempt)
					break
				}
				if err == nil && answer == "busy" {
					if attempt >= maxAttempts {
						solace.Errf(*role, "%s: bank busy on attempt %d of %d, giving up: discarded to the dead message queue", id, attempt, maxAttempts)
						if err := rcv.Settle(msg, config.PersistentReceiverRejectedOutcome); err != nil {
							solace.Errf(*role, "could not reject %s: %s", id, err)
						}
						return
					}
					next := strings.Replace(p, fmt.Sprintf(`"attempt":%d`, attempt), fmt.Sprintf(`"attempt":%d`, attempt+1), 1)
					if err := solace.PublishConfirmed(svc, pub, retryTopic+solace.FieldString(p, "account"), next); err != nil {
						solace.Errf(*role, "%s: could not put it on the retry queue: %s", id, err)
						return
					}
					solace.Logf(*role, "%s: bank busy, retry %d of %d in 5 s", id, attempt, maxAttempts-1)
					break
				}
				// No answer: the bank is down, not this payment. Stop taking
				// payments until it is back, and keep this one.
				if paused.CompareAndSwap(false, true) {
					solace.Errf(*role, "%s: no answer from the bank: %s", id, err)
					solace.Errf(*role, "pausing payments, checking the bank every 2 s")
					_ = main.Pause()
					_ = retries.Pause()
				} else {
					solace.Errf(*role, "%s: still no answer from the bank", id)
				}
				select {
				case <-stopping:
					return
				case <-time.After(2 * time.Second):
				}
			}
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(*role, "could not acknowledge %s: %s", id, err)
			}
		}
	}
	if err := main.ReceiveAsync(handle(main)); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	if err := retries.ReceiveAsync(handle(retries)); err != nil {
		solace.Errf(*role, "could not register the retry handler: %s", err)
		os.Exit(1)
	}
	solace.WaitForStop()
	close(stopping)
}
