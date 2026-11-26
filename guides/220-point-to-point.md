# Point-to-Point

The shop's order service needs a shipping label printed for every order. It
does not broadcast "an order needs a label" for whoever is interested. Each
print job goes to one place, the label printer's queue, and only the printer
gets it.

That is point-to-point: **each message goes to one destination, and one
receiver takes it from there.** If the receiver is away, the queue keeps the
message until it comes back.

Press **Play**. It creates the queue `q.p2p.labels` and subscribes it to one
topic, starts the **Label Printer**, then the **Order Service**, which
publishes a 25 KB label job to that topic every half second.

To be walked through this page step by step, press **Guided tour** in the
top bar: it tells you what to do, waits while you do it, and explains what
you see. Its second tab, **Quick overview**, is a short spoken version.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Publish-Subscribe**: the sender publishes to a topic and every
  subscriber gets a copy. Here the sender also publishes to a topic, but
  the broker is set up so that one queue takes it and one receiver reads
  that queue. Nobody else sees the job.
- **Competing Consumers**: several readers share one queue to go faster.
  Here there is one reader.
- **Exclusive Consumer**: still one reader at a time, but with a standby
  waiting to take over. Here there is no standby.

## Run it

The Order Service's log shows each job and the broker accepting it. The Label
Printer's log shows the same job numbers, in order.

The sender does not know the queue. It publishes to the topic
`p2p/label/requested`, and the queue subscribes to that topic. In
Solace Broker Manager, open **Queues**, then q.p2p.labels: its
**Subscriptions** tab lists that one topic. **What the queue subscribes to**,
under On the broker, shows the same.

This is the form Solace recommends, because the sender is tied to a topic
and not to a queue. The queue can be renamed, or a second queue added for
another kind of consumer, without touching the sender.

You can also send to a queue by its name. Every queue has a network topic,
`#P2P/QUE/<queue>`, which delivers to that queue and nothing else, with no
subscription needed. That works, but the sender is then tied to that one
queue. Try this, below, shows it.

## On the broker

- **Queue**: `msgSpoolUsage` is what waits for the printer, near zero while
  it keeps up, against its 1 MB limit (`maxMsgSpoolUsage`).
  `maxMsgSpoolUsageExceededDiscardedMsgCount` counts jobs refused because
  the queue was full.
- **What the queue subscribes to**: the one topic that brings jobs to the
  queue.
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
2. **The sender gets the topic wrong.** The Order Service restarts with its
   topic misspelt. Every job is refused with `No Subscription Match`: no
   queue subscribes to that topic. Without the client profile setting that
   makes the broker say so, those jobs would vanish silently.

## Try this

- Send to the queue by name. Stop the **Order Service** node, then in a
  terminal at the repository's root run
  `bash cockpit/apps/run.sh p2p send --queue q.p2p.labels`. The Label
  Printer's log shows the jobs arriving: they reach the same queue without
  using its subscription. Stop it with Ctrl+C, then start the Order Service
  again. A sender like this only works with a queue of exactly that name.
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
- The recommended form is a topic that one queue subscribes to: the sender
  publishes to the topic and never names the queue. A sender can also
  address a queue by name, with its network topic `#P2P/QUE/<queue>`.
  Solace's own wording: "it's best to map a topic to a queue for more
  flexibility"
  ([Creating Your First Queue](https://docs.solace.com/Cloud/ggs_queue.htm)).
- Nothing in the message makes it point-to-point. It is point-to-point
  because one queue subscribes to the topic and one receiver reads the
  queue. A second queue subscribed to the same topic would get a copy of
  every job.
- Delivery is at least once. If the printer crashed after printing a job but
  before acknowledging it, the broker would send that job again, so a real
  receiver must tolerate a repeat (here, it would print twice).
- A full queue refuses new messages back to the sender, which is told; a
  message that matches no queue or subscriber is dropped silently unless the
  client profile says to refuse it.

Next: [Request-Reply](230-request-reply.md).
