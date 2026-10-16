# Exclusive Consumer

Some work has to happen one item at a time, in order. A ledger is the classic
case: apply a refund before the payment it refunds and the balance is wrong,
even if only for a moment. So you want exactly one ledger writer.

You also want that writer to survive a crash. Running two copies fixes that,
but now there are two writers. An exclusive queue gives you both: any number
of consumers can bind to it, and the broker delivers to only one of them at
a time. The rest wait on standby.

Press **Play**. It creates the queue, starts **Ledger A**, then **Ledger B**,
then the **Payments Service**, which publishes a posting every half second.

## Watch who is active

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

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

Under **On the broker**, open **Consumers on q.excl.postings**. Both ledgers
are listed. `activityState` is `active-consumer` for A and `inactive` for B.

## Why not just one consumer, or a shared queue?

With one consumer, a crash stops the ledger until someone restarts it.

With a non-exclusive queue and two consumers, both would work at once. That
is the right shape for throughput (see Competing Consumers), but two writers
can apply postings out of order. The exclusive queue keeps one writer and a
spare.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you to
look for, then press **Reset**. **Why this breaks** explains the cause and
where you would meet it in production.

Both cards act on roles, not on a named ledger. Whichever ledger the broker
has made active is the one that fails, and the standby is the one that takes
over. Look at the diagram before you press Break it to see which is which.
After a crash card, the ledgers have swapped roles, so the next card hits the
other ledger.

1. **The active ledger crashes.** The active ledger is killed while it is
   applying postings. The standby turns active within a moment and carries on
   from the posting after the crashed ledger's last. Anything the crashed
   ledger had taken but not acknowledged comes first, marked
   `(redelivered)`: at most 5, and fewer when it was keeping up. Reset starts
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

The second card is the limit of the pattern. The broker fails over when a
session ends. It cannot tell a slow consumer from a stuck one. In production,
pair an exclusive queue with a health check that restarts a consumer that
stops making progress.

## Try this

- Stop the standby ledger, then start it again. It always rejoins as the
  standby while the other is active.
- Stop both ledgers and leave the Payments Service running. Watch
  `lastSpooledMsgId` pull ahead of `highestAckedMsgId` under **Queue**. Start
  either ledger: it becomes active and works through the backlog in order.
- Change `max_delivered_unacked_msgs_per_flow` in
  `cockpit/scenarios/exclusive-consumer/tf/queues.tf`, press **Play**, and run
  the second card again. A stalled ledger holds exactly that many postings.

Next: [Competing Consumers](410-competing-consumers.md).
