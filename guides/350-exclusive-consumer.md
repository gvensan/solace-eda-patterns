# Exclusive Consumer

Some work has to happen one item at a time, in order. A ledger is the classic
case: apply a refund before the payment it refunds and the balance is wrong,
even if only for a moment. So you want exactly one ledger writer.

You also want that writer to survive a crash. Running two copies fixes that,
but now there are two writers. An exclusive queue gives you both: several
consumers can bind to it (up to the queue's bind limit), and the broker
delivers to only one of them at a time. The rest wait on standby. That
gives one active delivery flow, not a complete ledger: a real one also
keeps its state where the standby can reach it, and recognises a posting
it has already made.

Press **Play**. It creates the queue, starts **Ledger A**, then **Ledger B**,
then the **Payments Service**, which publishes a posting every half second.

To be walked through this page step by step, press **Guided tour** in the
top bar: it tells you what to do, waits while you do it, and explains what
you see. Its second tab, **Quick overview**, is a short spoken version.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **One consumer, no queue sharing**: a crash stops the ledger until someone
  restarts it. Here a standby is already bound and takes over.
- **Competing Consumers**: a non-exclusive queue with two consumers would
  have both working at once. That is the right shape for throughput, but
  two writers can apply postings out of order. The exclusive queue keeps one
  writer and a spare.

## Run it

In the diagram, Ledger A says **active consumer** and Ledger B says
**standby**. Both are running and both are bound to
`q.excl.postings`. The dashboard asks the broker which one it is delivering
to, so this is the broker's decision, not a guess.

Open each ledger's log from its node:

- Ledger A printed `STANDBY`, then `ACTIVE` a moment later, and is applying
  postings in sequence.
- Ledger B printed `STANDBY` and nothing else. It bound second, so the broker
  kept it in reserve.

Every consumer starts on standby. The Go API tells an app when the broker
makes it active, through the receiver's activation and passivation listener,
and that is the line each ledger prints.

## On the broker

- **Queue**: `accessType` is `exclusive`. `lastSpooledMsgId` minus
  `highestAckedMsgId` is roughly the backlog.
- **Consumers on q.excl.postings**: both ledgers are listed.
  `activityState` is `active-consumer` for the active one and `inactive`
  for the standby; `unackedMsgCount` is what the active one holds.
- **Connected clients**: the two ledgers and the payments service.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

Both cards act on roles, not on a named ledger. Whichever ledger the broker
has made active is the one that fails, and the standby is the one that takes
over. Look at the diagram before you press Break it to see which is which.
After a crash card, the ledgers have swapped roles, so the next card hits the
other ledger.

1. **The active ledger crashes.** The active ledger is killed while it is
   applying postings. The standby turns active within a moment and starts at
   the oldest posting the broker had no acknowledgement for: usually the one
   the crashed ledger was working on, sometimes one or two it had already
   applied whose acknowledgement had not yet left the process. Those come
   first, marked `(redelivered)`: at most 5, and fewer when it was keeping
   up. Compare the crashed ledger's last number with the first here:
   nothing is skipped, and a repeated number is the duplicate a real ledger
   must recognise. Reset starts
   the crashed ledger again, and it joins as the new standby: the broker does
   not hand the queue back.
2. **The active ledger hangs, so nothing is consumed.** The active ledger
   freezes: it stops processing but keeps its connection open. The broker can
   only see the connection, so it still treats the frozen ledger as the
   active consumer. It sends it postings until it holds 5 unacknowledged,
   then waits. The standby stays on standby, nobody consumes, and the queue
   backs up. In the diagram, the frozen ledger shows "active, not acking".
   Reset stops it (it ignores a polite stop, so it is killed after 5
   seconds), and only then does the standby take over, starting with those 5
   postings marked `(redelivered)`.

The second card is the limit of the pattern. The broker fails over when the
active consumer unbinds or its session ends. It cannot tell a slow consumer from a stuck one. In production,
pair an exclusive queue with a health check that restarts a consumer that
stops making progress.

## Try this

- Stop the standby ledger, then start it again. It always rejoins as the
  standby while the other is active.
- Stop both ledgers and leave the Payments Service running. Watch
  `lastSpooledMsgId` pull ahead of `highestAckedMsgId` under **Queue**. Start
  either ledger: it becomes active and works through the backlog in order.
- Change `max_delivered_unacked_msgs_per_flow` in
  `cockpit/scenarios/exclusive-consumer/tf/queues.tf`, press **Play**, then
  stop and start both ledgers so they bind with the new limit, and run the
  second card again. A stalled ledger holds exactly that many postings.

## How Solace does this

- A queue's access type is exclusive or non-exclusive. On an exclusive
  queue the first consumer to bind is active and the rest are standbys;
  the broker delivers to the active one only.
- The Go API tells each receiver when it becomes active or standby, with
  `WithActivationPassivationSupport` (`docs/spikes.md`, section 8).
- When the active consumer unbinds or its session ends, the broker makes a
  standby active and delivers, first and in order, every message the old
  one had not acknowledged, marked redelivered.
- A consumer that stalls but stays connected keeps the queue: it holds up
  to `max_delivered_unacked_msgs_per_flow` messages and nothing moves on.
  Detecting that is the application's job.

Next: [Competing Consumers](410-competing-consumers.md).
