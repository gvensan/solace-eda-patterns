# Competing Consumers

Every order the shop takes needs a confirmation email. Sending one takes a
moment, and at busy times one worker cannot keep up. Instead of a faster
worker, you run several identical ones on one shared queue.

That is competing consumers: **several identical workers share one queue,
and the broker gives each message to exactly one of them.** Add a worker to
go faster; lose one and the others carry on.

Press **Play**. It creates the shared queue `q.cc.emails`, starts **Worker 1**
and **Worker 2**, then **Checkout**, which confirms six orders a second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Point-to-Point** and **Exclusive Consumer**: one reader at a time. Here
  every bound worker reads at once.
- **Consumer Groups**: several groups each get every message, and the
  workers within a group compete. Here there is one group of workers.
- **Shock Absorber**: the point is the queue soaking up a burst. Here the
  point is more workers taking work off the queue.
- **Partitioning**: keeps related messages in order by always sending them
  to the same worker. Competing consumers make no such promise: two emails
  for the same customer can be sent by two workers in either order.

## Watch it work

Open Worker 1's and Worker 2's logs from their nodes. Neither sees every
order: each order number appears in exactly one of them.

Under **On the broker**, open **Workers on q.cc.emails**. `ackedMsgCount`
climbs for both. Open **Queue**: `lastSpooledMsgId` creeps ahead of
`highestAckedMsgId`, because checkout confirms six orders a second and two
workers send about five. A backlog builds.

Now click **Worker 3** in the diagram to start it. Nothing else changes: not
checkout, not the queue, not the other workers. Three workers send about
seven emails a second, and the backlog drains.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you to
look for, then press **Reset**. **Why this breaks** explains the cause and
where you would meet it in production.

Both cards hit one worker (the diagram marks it) and show another worker's
log.

1. **A worker crashes.** The worker is killed while it holds jobs it has not
   finished. Another worker sends those jobs within a moment, marked
   `(redelivered)`, and the rest keep flowing. Reset starts the crashed
   worker again, and it takes a share straight away: on a shared queue there
   is no standby.
2. **A worker hangs, and the others carry on.** The worker freezes but stays
   connected. The other worker keeps sending; only the 5 jobs the frozen one
   holds are stuck. The diagram shows the frozen worker as "active, not
   acking", with no packets reaching it. Compare the Exclusive Consumer's
   hang card, where nothing moves. Reset stops the frozen worker, and its 5
   jobs go to another worker marked `(redelivered)`.

## Try this

- Stop Worker 2 and Worker 3, leaving one worker. Watch the backlog grow
  faster under **Queue**, then start them again and watch it drain.
- Note which worker sends a given customer's emails. Over a minute, the same
  customer is served by different workers, in whatever order they finish.
  Partitioning is the pattern that keeps one customer's messages together.

Next: [Cleanup](999-cleanup.md).
