// Package saga implements the saga scenario.
//
// Placing an order takes three services, each with its own data: orders,
// payments and inventory. There is no transaction across them. Instead each
// step is a local change followed by an event, and the next service reacts to
// that event (choreography: nobody is in charge). When a later step fails,
// the earlier steps are undone by compensating actions, also driven by
// events:
//
//	order created  -> payments takes the money   -> payment taken
//	payment taken  -> inventory reserves stock   -> stock reserved | stock failed
//	stock reserved -> orders confirms the order
//	stock failed   -> payments refunds the money -> payment refunded
//	payment refunded -> orders cancels the order
//
// Every service reads its own queue, so a service that is down only delays
// the sagas that need it.
package saga

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"solace-workshop/apps/internal/solace"
	sol "solace.dev/go/messaging/pkg/solace"
	"solace.dev/go/messaging/pkg/solace/message"
)

const (
	created  = "saga/shop/order/created/v1/"
	taken    = "saga/shop/payment/taken/v1/"
	refunded = "saga/shop/payment/refunded/v1/"
	reserved = "saga/shop/stock/reserved/v1/"
	failed   = "saga/shop/stock/failed/v1/"
)

type service struct {
	role string
	svc  sol.MessagingService
	pub  sol.PersistentMessagePublisher
}

func start(role, user, clientName string) service {
	svc := solace.ConnectAs(role, user, clientName)
	return service{role: role, svc: svc, pub: solace.StartPersistentPublisher(role, user, svc)}
}

func (s service) emit(topic, order, payload string) bool {
	if err := solace.PublishConfirmed(s.svc, s.pub, topic+order, payload); err != nil {
		solace.Errf(s.role, "%s: could not publish %s: %s", order, topic, err)
		return false
	}
	return true
}

// consume binds the service's queue and calls handle for each event, with
// the event's type (the topic up to the order id). The event is acknowledged
// only after handle has published whatever it publishes.
func (s service) consume(user, queue string, handle func(kind, order, payload string) bool) sol.PersistentMessageReceiver {
	rcv := solace.BindQueue(s.role, user, s.svc, queue, solace.QueueOpts{ClientAck: true})
	if err := rcv.ReceiveAsync(func(msg message.InboundMessage) {
		t := msg.GetDestinationName()
		i := strings.LastIndexByte(t, '/')
		if handle(t[:i+1], t[i+1:], solace.PayloadOf(msg)) {
			if err := rcv.Ack(msg); err != nil {
				solace.Errf(s.role, "could not acknowledge: %s", err)
			}
		}
	}); err != nil {
		solace.Errf(s.role, "could not register the message handler: %s", err)
		os.Exit(1)
	}
	return rcv
}

// Orders starts a saga every --every by creating an order, and ends each one
// when the outcome arrives: confirmed, or cancelled after a refund. Every 5
// seconds it lists the sagas still open.
func Orders(args []string) {
	fs := flag.NewFlagSet("saga orders", flag.ExitOnError)
	user := fs.String("user", "svc-saga-orders", "client username to connect as")
	role := fs.String("role", "orders", "label for log lines")
	clientName := fs.String("client-name", "saga-orders", "client name the broker reports this app by")
	every := fs.Duration("every", 2*time.Second, "pause between orders")
	fs.Parse(args)

	s := start(*role, *user, *clientName)
	defer s.svc.Disconnect()
	defer solace.Terminate(s.pub)

	var mu sync.Mutex
	open := map[string]time.Time{}
	rcv := s.consume(*user, "q.saga.orders", func(kind, order, _ string) bool {
		mu.Lock()
		began, ok := open[order]
		delete(open, order)
		mu.Unlock()
		took := ""
		if ok {
			took = fmt.Sprintf(" after %.1f s", time.Since(began).Seconds())
		}
		switch kind {
		case reserved:
			solace.Logf(*role, "%s confirmed: paid and stock reserved%s", order, took)
		case refunded:
			solace.Logf(*role, "%s cancelled: out of stock, payment refunded%s", order, took)
		}
		return true
	})
	defer solace.Terminate(rcv)
	solace.Logf(*role, "creating an order every %s", *every)
	fmt.Println()

	stop := make(chan struct{})
	go func() { solace.WaitForStop(); close(stop) }()
	report := time.NewTicker(5 * time.Second)
	defer report.Stop()
	next := time.After(0)
	base := int(time.Now().Unix() % 10000 * 10)
	for n := 1; ; {
		select {
		case <-stop:
			return
		case <-report.C:
			mu.Lock()
			var waiting []string
			for o, t := range open {
				if time.Since(t) > 5*time.Second {
					waiting = append(waiting, fmt.Sprintf("%s (%.0f s)", o, time.Since(t).Seconds()))
				}
			}
			mu.Unlock()
			if len(waiting) > 0 {
				sort.Strings(waiting)
				if len(waiting) > 6 {
					waiting = append(waiting[:6], fmt.Sprintf("and %d more", len(waiting)-6))
				}
				solace.Logf(*role, "waiting on %s", strings.Join(waiting, ", "))
			}
		case <-next:
			order := fmt.Sprintf("O-%06d", base+n)
			amount := 10 + rand.Intn(90)
			mu.Lock()
			open[order] = time.Now()
			mu.Unlock()
			if s.emit(created, order, fmt.Sprintf(`{"order":"%s","amount":%d}`, order, amount)) {
				solace.Logf(*role, "%s created (%d.00): saga started", order, amount)
			}
			n++
			next = time.After(*every)
		}
	}
}

// Payments takes the money when an order is created, and gives it back when
// inventory says the stock failed: the compensating action.
func Payments(args []string) {
	fs := flag.NewFlagSet("saga payments", flag.ExitOnError)
	user := fs.String("user", "svc-saga-payments", "client username to connect as")
	role := fs.String("role", "payments", "label for log lines")
	clientName := fs.String("client-name", "saga-payments", "client name the broker reports this app by")
	fs.Parse(args)

	s := start(*role, *user, *clientName)
	defer s.svc.Disconnect()
	defer solace.Terminate(s.pub)
	rcv := s.consume(*user, "q.saga.payments", func(kind, order, p string) bool {
		amount, _ := solace.FieldInt(p, "amount")
		switch kind {
		case created:
			if !s.emit(taken, order, p) {
				return false
			}
			solace.Logf(*role, "%s: took %d.00", order, amount)
		case failed:
			if !s.emit(refunded, order, p) {
				return false
			}
			solace.Logf(*role, "%s: refunded %d.00 (compensation: the stock failed)", order, amount)
		}
		return true
	})
	defer solace.Terminate(rcv)
	fmt.Println()
	solace.WaitForStop()
}

// Inventory reserves stock for each paid order, or reports that it cannot.
// SIGUSR1 sells the stock out; SIGUSR2 restocks.
func Inventory(args []string) {
	fs := flag.NewFlagSet("saga inventory", flag.ExitOnError)
	user := fs.String("user", "svc-saga-inventory", "client username to connect as")
	role := fs.String("role", "inventory", "label for log lines")
	clientName := fs.String("client-name", "saga-inventory", "client name the broker reports this app by")
	fs.Parse(args)

	s := start(*role, *user, *clientName)
	defer s.svc.Disconnect()
	defer solace.Terminate(s.pub)

	var mu sync.Mutex
	soldOut := false
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for v := range sig {
			out := v == syscall.SIGUSR1
			mu.Lock()
			soldOut = out
			mu.Unlock()
			if out {
				solace.Logf(*role, "the last item has sold: out of stock")
			} else {
				solace.Logf(*role, "restocked")
			}
		}
	}()

	rcv := s.consume(*user, "q.saga.inventory", func(_, order, p string) bool {
		mu.Lock()
		out := soldOut
		mu.Unlock()
		if out {
			if !s.emit(failed, order, p) {
				return false
			}
			solace.Logf(*role, "%s: out of stock, reported stock failed", order)
			return true
		}
		if !s.emit(reserved, order, p) {
			return false
		}
		solace.Logf(*role, "%s: stock reserved", order)
		return true
	})
	defer solace.Terminate(rcv)
	fmt.Println()
	solace.WaitForStop()
}
