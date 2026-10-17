# Shock Absorber

Concert ticket sales. When tickets go on sale, 1,500 purchase requests
arrive in a couple of seconds; the rest of the time they trickle in. Two
booking workers reserve seats at a steady 20 a second each.

The queue between them is the shock absorber: **it takes a burst at the
speed the producer sends it, and the consumers drain it at theirs.** The
producer is never slowed down or refused, and the consumers never see more
than they can handle. The queue has limits, in space and in time, and the
cards show both.

Press **Play**. It creates `q.sa.purchases` and its dead message queue,
starts **Booking 1** and **Booking 2**, then the **On-Sale Page**, which
sends a trickle and a burst of 1,500 every minute.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Competing Consumers**: adds workers to go faster. A shock absorber
  keeps the same workers and lets the queue hold the difference.
- **Backpressure and Push**: the consumer pacing its own intake. Here nobody
  slows down; the queue takes the surge.
- **Point-to-Point**: a queue holding messages while the receiver is away.
  Here the receivers are there, and slower than the burst.

## Run it

Wait for the first burst, or press **Fire a burst now** under Step through
it. The **On-Sale Page** log says "BURST: requests R-41 to R-1540 handed
to the API in 7 ms; the broker never slowed the on-sale page down", and
three seconds later "every request confirmed by the broker: 1500 stored,
none refused". In the diagram, `q.sa.purchases` jumps to about 1,500 waiting and
then falls at 40 a second as the workers book. Each worker's log says every
5 seconds how many it has booked and how long the latest waited; near the
end of a burst that is 30 seconds or more.

## On the broker

- **Queues**: `msgSpoolUsage` rises with a burst and drains; `maxMsgSpoolUsage`
  is the absorber's size. `maxMsgSpoolUsageExceededDiscardedMsgCount` counts
  what a burst too big for it cost, and `maxTtlExpiredToDmqMsgCount` what
  waited too long.
- **The workers on q.sa.purchases**: each holds up to 20 unacknowledged;
  `ackedMsgCount` climbs at 20 a second each while there is a backlog.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The burst is bigger than the queue.** A second on-sale page releases
   6,000 tickets at once. The first few thousand are stored; then every
   receipt says "refused: Spool Over Quota", and the page sums it up: "the
   broker refused N request(s): those customers got no booking". The
   workers carry on with what was stored. **Reset** clears the backlog.
2. **Requests go stale while they wait.** A second on-sale page releases
   1,500 tickets with a 20-second seat hold: each request carries a
   20-second time to live. The workers need about 40 seconds for the burst,
   so the requests at the back are still waiting when their hold lapses.
   The broker moves each to `q.sa.purchases.dmq`: dots hop across,
   labelled "expired", and the DMQ's count climbs. **Reset** clears what is
   left.

## Try this

- Start a third worker: copy the `booking-2` action in
  `cockpit/scenarios/shock-absorber/scenario.yaml` as `booking-3` with
  client name `sa-booking-3`, start it, and fire a burst. The backlog
  drains in two thirds of the time: the absorber and more workers are not
  either-or.
- Change `max_msg_spool_usage` in `cockpit/scenarios/shock-absorber/tf/queues.tf`
  to 5 and press **Play**: the first card's burst of 6,000 now fits, and the
  workers take two and a half minutes to book it. Size is a trade between
  what you can take and how long the last customer waits.

## How Solace does this

- A queue stores guaranteed messages on the broker until a consumer takes
  them, so a producer publishes at its own pace. Its size
  (`max_msg_spool_usage`, in MB) and the broker's message spool are the
  limits; past them the broker refuses messages to the publisher with
  `Spool Over Quota` (the default `reject_msg_to_sender_on_discard_behavior`).
- A publisher can give a message a time to live; a queue honours it only
  with `respect_ttl_enabled`, and can cap it with `max_ttl`. On broker
  10.26.0 an expired message goes to the queue's dead message queue if it
  has one, and is counted on the queue it left (`docs/spikes.md`, sections
  11 and 16).
- A burst arrives faster than it leaves, which is the whole point. What
  the queue cannot do is make a late answer early: that is what the time
  to live is for.

Next: [Partitioning](440-partitioning.md).
