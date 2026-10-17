# Dead Message Queue

Now and then the shop receives an order that can never be processed: this
one has no delivery address. Trying it again and again wastes the order
intake's time, and on a queue read in order it holds up every order behind
it. Dropping it loses the order.

A dead message queue (DMQ) is the middle way: **after a set number of
attempts, a message that keeps failing is moved aside to a queue of its own,
kept for someone to look at, and the messages behind it keep flowing.** A
message that waits too long and expires goes there too. The pattern guide,
and other brokers, call it a dead letter queue; Solace's name is dead
message queue.

This page describes **broker 10.26.0**, the version this workshop pins.
Where 10.8.1, the core workshop's version, behaves differently, it says so.

Press **Play**. It creates `q.dlq.orders` and its dead message queue
`q.dlq.orders.dmq`, starts **Operations** watching the DMQ, starts the
**Order Intake**, then the **Shop**, which places a good order every half
second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Discard, Pause and Retry** decides what to do about a failure: drop it,
  slow down, or try again later. A dead message queue is where a message
  goes once retrying is over.
- **Guaranteed Delivery** makes sure a message is not lost on the way. A dead message queue makes sure it is not lost when it cannot be processed.

## Watch it work

Each queue box in the broker shows how many orders it holds now: `waiting`
on `q.dlq.orders`, `parked` on the DMQ. When the broker moves an order to
the DMQ, a dot hops from one box to the other with the reason beside it,
`failed 3 attempts` or `expired`, then runs on to Operations. The reason
comes from the broker's own counters on `q.dlq.orders`, not from the order:
a parked message does not say why it was parked.

Under **On the broker**, open **Retry settings on q.dlq.orders**: two
redeliveries (`maxRedeliveryCount`, so three attempts in all), the dead
message queue it uses, whether it respects the publisher's DMQ-eligible
flag (`No`, the 10.26.0 default), and whether it respects a message's time
to live (`Yes`; the broker's default is no). **Queues** counts what moved
to the DMQ, by reason: `maxRedeliveryExceededToDmqMsgCount` and
`maxTtlExpiredToDmqMsgCount`.

Click **Place a bad order** in **Step through it**, or use the first card.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you to
look for, then press **Reset**. **Why this breaks** explains the cause and
where you would meet it in production.

1. **An order that can never succeed.** The shop places one order with no
   address. In the intake's log it fails on attempt 1, 2 and 3 while good
   orders are accepted around it. After the third, a dot hops to the DMQ,
   labelled `failed 3 attempts`, Operations shows it parked, and **Parked
   orders on q.dlq.orders.dmq** lists it. Nothing needs
   resetting: the order is kept for someone to deal with.
2. **The sender says not to keep it (broker 10.26.0).** The queue is set to
   respect the DMQ-eligible flag, and the shop places a bad order marked not
   eligible. It fails three times and is dropped: no dot hops, and
   Operations never sees it.
   Reset turns the setting back off and places the same kind of order again.
   This time it is parked, even though it says not eligible: on 10.26.0,
   with the setting off, the queue parks everything it removes.

3. **Orders expire while the intake is down.** The intake stops, and the
   shop places three orders (`H-...`) that are only valid for 5 seconds:
   each is sent with a time to live. The shop's ordinary orders pile up on
   `q.dlq.orders`, unharmed, since they have none. The three do not: about 5
   seconds later each hops to the DMQ, labelled `expired`, and Operations
   shows it parked, "was valid for 5s". Reset starts the intake, which
   works through the waiting orders.

### On broker 10.8.1

There is no `respect_dmq_eligible_enabled` on 10.8.1 (terraform would refuse
the attribute). The publisher's flag always decides: an order marked not
eligible is dropped, and one left unset, or marked eligible, is parked. So
on 10.8.1 the second card's order would be dropped both times. The
measurements behind this are in `docs/spikes.md`, section 6.

## Try this

- Place a bad order with **Place a bad order**, then restart **Operations**
  from its node. The parked order is shown again, marked (shown before): the
  watcher reads without removing, so it stays on the DMQ.
- Change `max_redelivery_count` in
  `cockpit/scenarios/dead-letter-queue/tf/queues.tf` to 1 and press **Play**.
  A bad order is now parked after two attempts. Do not try 0: on Solace it
  means **redeliver forever**. Measured here, one failing message was
  redelivered almost 10,000 times in 4 seconds.
- Set `respect_ttl_enabled` in the same file to `false`, press **Play**,
  then use the third card. The orders' time to live is ignored: they wait
  with the others and the intake accepts them when it is back, long after
  they stopped being valid.
- Remove the `dead_msg_queue` line and press **Play**. The queue falls back to
  the default, `#DEAD_MSG_QUEUE`, which this broker does not have, so a bad
  order is dropped after its last attempt. **Queues** counts it in
  `maxRedeliveryExceededToDmqFailedMsgCount`.

Next: [Access Control and Authorization](750-access-control.md).
