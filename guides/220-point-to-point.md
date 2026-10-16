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

## Watch it work

The Order Service's log shows each job and the broker accepting it. The Label
Printer's log shows the same job numbers, in order.

The sender addresses the queue itself. Its topic is the queue's network
topic, `#P2P/QUE/q.p2p.labels`, which delivers to that queue and nothing
else. Under **On the broker**, open **Queue**: the queue has no topic
subscriptions at all, yet it receives every job.

Many systems do point-to-point with an ordinary topic that exactly one
queue subscribes to. That works too; addressing the queue by name just makes
the "one destination" explicit.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you to
look for, then press **Reset**. **Why this breaks** explains the cause and
where you would meet it in production.

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

Next: [Exclusive Consumer](350-exclusive-consumer.md).
