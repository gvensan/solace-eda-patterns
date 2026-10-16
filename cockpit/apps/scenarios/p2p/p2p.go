// Package p2p implements the point-to-point scenario.
//
// The shop's order service needs a shipping label printed for every order.
// It sends each print job to one named queue, q.p2p.labels, and the label
// printer is the one app that reads it. Nothing else sees the jobs, and if
// the printer is away the queue keeps them until it comes back.
//
// The sender addresses the queue itself, by the queue's network topic
// #P2P/QUE/<queue>, rather than publishing to a topic that something may or
// may not subscribe to. That is the point of the pattern: the sender knows
// exactly where its message goes.
package p2p

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"solace-workshop/apps/internal/solace"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

// Send publishes one label job at a steady pace, each with the next job
// number, to one queue by name.
//
// Each job waits for the broker's acknowledgement, so a job the broker
// refuses (the queue is full, or there is no such queue) is reported against
// its own number, with the broker's error as it is.
func Send(args []string) {
	fs := flag.NewFlagSet("p2p send", flag.ExitOnError)
	user := fs.String("user", "svc-p2p-orders", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	queue := fs.String("queue", "q.p2p.labels", "queue to send each job to")
	every := fs.Duration("every", 500*time.Millisecond, "pause between jobs")
	size := fs.Int("size", 25000, "bytes per job (a label is a small PDF)")
	clientName := fs.String("client-name", "p2p-orders", "client name the broker reports this app by")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	pub := solace.StartPersistentPublisher(*role, *user, svc)
	defer solace.Terminate(pub)

	// A queue's network topic delivers to that queue and nothing else.
	dest := resource.TopicOf("#P2P/QUE/" + *queue)
	solace.Logf(*role, "sending a label job to queue %s every %s. Press Stop to end.", *queue, *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()

	pdf := strings.Repeat("x", *size)
	for n := 1; ; n++ {
		order := fmt.Sprintf("O-%05d", 10000+n)
		payload := fmt.Sprintf(`{"job":%d,"order":"%s","pdf":"%s"}`, n, order, pdf)
		msg, err := svc.MessageBuilder().BuildWithStringPayload(payload)
		if err != nil {
			solace.Errf(*role, "could not build job %d: %s", n, err)
			os.Exit(1)
		}
		if err := pub.PublishAwaitAcknowledgement(msg, dest, 5*time.Second, nil); err != nil {
			solace.Errf(*role, "job %d for order %s refused: %s", n, order, err)
		} else {
			solace.Logf(*role, "sent label job %d for order %s (%d KB)", n, order, *size/1000)
		}

		select {
		case <-stop:
			solace.Logf(*role, "stopping after %d job(s).", n)
			return
		case <-time.After(*every):
		}
	}
}

// Print binds the label queue and prints one label at a time.
//
// It checks that job numbers follow on from each other. A job that never
// arrives shows up as a GAP: a job the broker refused at the sender, for
// example, is gone, because nothing stored it.
func Print(args []string) {
	fs := flag.NewFlagSet("p2p print", flag.ExitOnError)
	user := fs.String("user", "svc-p2p-printer", "client username to connect as")
	role := fs.String("role", "printer", "label for log lines")
	queue := fs.String("queue", "q.p2p.labels", "queue to read jobs from")
	work := fs.Duration("work", 200*time.Millisecond, "time to print one label")
	clientName := fs.String("client-name", "p2p-printer", "client name the broker reports this app by")
	fs.Parse(args)

	svc := solace.ConnectAs(*role, *user, *clientName)
	defer svc.Disconnect()

	rcv := solace.BindQueue(*role, *user, svc, *queue, solace.QueueOpts{ClientAck: true})
	defer solace.Terminate(rcv)

	solace.Logf(*role, "printing labels from %s.", *queue)
	fmt.Println()

	printed, last := 0, 0
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		time.Sleep(*work)
		payload := solace.PayloadOf(msg)
		job, _ := solace.FieldInt(payload, "job")
		order := solace.FieldString(payload, "order")

		note := ""
		switch {
		case job == 1 || last == 0:
			// A new sender run starts again at 1, and a fresh printer has
			// nothing to compare with.
		case job > last+1:
			note = fmt.Sprintf("  GAP: jobs %d to %d never arrived", last+1, job-1)
		case job <= last:
			note = fmt.Sprintf("  (job %d again)", job)
		}
		last = job
		printed++

		line := fmt.Sprintf("[%s] printed label job %d for order %s", *role, job, order)
		if msg.IsRedelivered() {
			line += "  (redelivered)"
		}
		fmt.Println(line + note)
		if err := rcv.Ack(msg); err != nil {
			solace.Errf(*role, "could not acknowledge job %d: %s", job, err)
		}
	}); err != nil {
		solace.Errf(*role, "could not register the message handler: %s", err)
		os.Exit(1)
	}

	solace.WaitForStop()
	fmt.Println()
	solace.Logf(*role, "stopped after printing %d label(s).", printed)
}
