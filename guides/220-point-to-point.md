# Point-to-Point

The shop's order service needs a shipping label printed for every order. It
does not broadcast "an order needs a label" for whoever is interested. It
sends a print job to one place, the label printer's queue, and only the
printer gets it.

That is point-to-point: **the sender names one destination, and one
receiver takes each message from it.** If the receiver is away, the queue
keeps the message until it comes back.

Press **Play**. It creates the queue `q.p2p.labels`, starts the **Label
Printer**, then the **Order Service**, which sends a 25 KB label job every
half second.

To be walked through this page step by step, press **Guided tour** in the
top bar: it tells you what to do, waits while you do it, and explains what
you see. Its second tab, **Quick overview**, is a short spoken version.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Publish-Subscribe**: the sender publishes to a topic and every
  subscriber gets a copy. Here the sender names one queue and nobody else
  sees the job.
- **Competing Consumers**: several readers share one queue to go faster.
  Here there is one reader.
- **Exclusive Consumer**: still one reader at a time, but with a standby
  waiting to take over. Here there is no standby.

## Run it

The Order Service's log shows each job and the broker accepting it. The Label
Printer's log shows the same job numbers, in order.

The sender addresses the queue itself. Its topic is the queue's network
topic, `#P2P/QUE/q.p2p.labels`, which delivers to that queue and nothing
else. In Solace Broker Manager, open **Queues**, then q.p2p.labels: its
**Subscriptions** tab is empty. The queue has no topic subscriptions at all,
yet it receives every job.

Many systems do point-to-point with an ordinary topic that exactly one
queue subscribes to. That works too; addressing the queue by name just makes
the "one destination" explicit.

## On the broker

- **Queue**: `msgSpoolUsage` is what waits for the printer, near zero while
  it keeps up, against its 1 MB limit (`maxMsgSpoolUsage`).
  `maxMsgSpoolUsageExceededDiscardedMsgCount` counts jobs refused because
  the queue was full.
- **Reader of q.p2p.labels**: the printer's connection to the queue, and the
  jobs it has acknowledged.
- **Client profile**: `rejectMsgToSenderOnNoSubscriptionMatchEnabled` is the
  setting the second card relies on.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The printer goes offline.** The queue's reader stops. The Order Service
   carries on without an error, because the queue holds the jobs. After about
   20 seconds the queue reaches its 1 MB limit and the broker starts refusing
   jobs back to the sender with `Spool Over Quota`. Reset starts the printer:
   it prints every job that waited, in order, then reports a `GAP` for the
   refused ones, which were never stored.
2. **The sender names the wrong queue.** The Order Service restarts with the
   queue name misspelt. Every job is refused with `No Subscription Match`:
   there is no such queue. Without the client profile setting that makes the
   broker say so, those jobs would vanish silently.

## Try this

- Stop the printer for ten seconds only, then start it again. Nothing is
  refused and nothing is missing: the queue absorbed the outage.
- Under **On the broker**, watch `maxMsgSpoolUsageExceededDiscardedMsgCount`
  on **Queue** while the first card is in effect. It counts every refused job.
- Raise `max_msg_spool_usage` in
  `cockpit/scenarios/point-to-point/tf/queues.tf` to 5 and press **Play**. The
  same outage now lasts about two minutes before anything is refused.

## How Solace does this

- A durable queue keeps each guaranteed message until its receiver
  acknowledges it. The sender's publish is confirmed once the broker has
  stored the job, so the sender knows it is safe without knowing whether the
  printer is there.
- The sender can address the queue by name, with its network topic
  `#P2P/QUE/<queue>`, or publish to a topic the queue subscribes to.
- Delivery is at least once. If the printer crashed after printing a job but
  before acknowledging it, the broker would send that job again, so a real
  receiver must tolerate a repeat (here, it would print twice).
- A full queue refuses new messages back to the sender, which is told; a
  message that matches no queue or subscriber is dropped silently unless the
  client profile says to refuse it.

Next: [Request-Reply](230-request-reply.md).
