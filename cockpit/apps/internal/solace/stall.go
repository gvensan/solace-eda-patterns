package solace

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

// StallOnSignal arranges for SIGUSR1 to freeze the app, and returns a
// function that reports whether it has been frozen.
//
// A frozen app stays connected and stays bound: it simply stops processing
// and acknowledging, the way a deadlocked or paused process does. The broker
// cannot tell that from a slow consumer, which is what the "hangs" failure
// cards show. Failure modes send the signal through scripts/signal_consumer.sh.
//
// Call it before connecting, so a signal that arrives early is caught rather
// than taking SIGUSR1's default action, which is to exit.
func StallOnSignal(role string) func() bool {
	var stalled atomic.Bool
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		<-ch
		stalled.Store(true)
		Logf(role, "STALLED: still connected and still bound, but no longer processing or acknowledging")
	}()
	return stalled.Load
}

// Freeze holds the calling goroutine for good, as a stuck process would.
// A message handler calls it once the app is stalled, so nothing it holds
// is ever acknowledged.
func Freeze() {
	select {}
}
