# Partitioning

Each order moves through **created**, **paid**, **packed**, **shipped** and
**delivered**, and fulfilment must see one order's updates in that order:
packing an order that has not been paid for is a real mistake. Many orders
are in flight at once, so one consumer is too slow and three share the work.

That is partitioning: **messages with the same key always go to the same
partition, and each partition has one consumer at a time, so they are
handled in order, while messages with different keys are handled in
parallel.** Here the key is the order id. The consumer that owns a
partition can change when consumers join or leave, and several orders
share each partition.

Press **Play**. It creates the partitioned queue `q.part.orders` (six
partitions), starts **Fulfilment 1**, **2** and **3**, then the **Order
Service**, which moves six orders along at a time, one in each partition.
Every update carries a sequence number, `#1` to `#5`, so a consumer can tell
when one is missing.

Click the **Order Service** to change its **Pace**. **Clear** (six orders,
two updates a second) is the easiest to follow: each lane carries one colour
at a time. **Busy** (ten orders, five a second) is closer to real
traffic: several keys share most partitions. Changing it restarts the order
service, so orders in progress at that moment never finish.

To be walked through this page step by step, press **Guided tour** in the
top bar: it tells you what to do, waits while you do it, and explains what
you see. Its second tab, **Quick overview**, is a short spoken version.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Competing Consumers**: several consumers share a queue, but any message
  can go to any of them, so two updates for one order can be handled at once
  by two consumers, in either order. Partitioning keeps each key with one
  consumer.
- **Exclusive Consumer**: keeps everything in order by having one consumer at
  a time. Partitioning keeps order per key and still uses every consumer.

## Watch it work

The diagram draws the queue as six lanes, `P0` to `P5`, one per partition.
Each lane names the consumer the broker gave it to, with a line to that
consumer. Each consumer says which partitions it owns: two each.

Each coloured dot is one update a consumer handled, drawn a moment later
along the path it took: from the order service, through its partition, to
its consumer, where it is consumed. The letter and colour stand for the
order while it is in flight; a finished order's letter goes to the next
order in the same partition. Follow one letter: it always takes the same
lane to the same consumer. Under each consumer, the last few updates it
handled show as `A3`, `C1` and so on, the order's letter and sequence
number. They come in at the left of the line and leave from the right end,
oldest first. `A1` to the left of `A5` is one order delivered and the next
one starting.

Open a consumer's log from its node. Every line names the order, its
sequence number, its key and `msg`, the broker's number for that message.

## On the broker

Open **Partitions of q.part.orders**. The broker keeps each partition as an
internal queue of its own (`#pq/...`), and `partitionClientName` is the
consumer it belongs to: the same owner the lanes show. `spooledMsgCount` is
how many messages have gone into it. The counts differ, because keys are
hashed onto partitions rather than dealt out in turn.

`lastSpooledMsgId` is the newest message's number. Compare it with the
`msg` numbers in a consumer's log: that is how the dashboard places each
dot in its lane. A consumer cannot ask which partition a message came from,
so the dashboard matches message numbers against what the broker lists in
each partition. The few it misses cross the queue as faint dashed dots.

**Queue** shows `partitionRebalanceStatus`: `ready` normally, `holddown`
while the broker waits after a consumer comes or goes, and `rebalancing`
while it moves partitions. The lanes show the same as "waiting to
rebalance" and "moving partitions".

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **A consumer crashes, and its partitions move.** One consumer is killed
   (the diagram marks it). Its two lanes say "no owner", and their held
   counts creep up, because for 5 seconds (the rebalance delay) nothing reads
   them. Then each of the other two consumers takes one, and the lane lines
   swing over. The card shows one of them, usually picking an order up part
   way, "picked up O-... at #3"; sometimes the lane it took happens to start
   a new order. Every order still moves one number at a time. Reset
   starts the crashed consumer, and 5 seconds later two lanes move back to it.
2. **A fourth consumer joins.** Fulfilment 4 starts. For 5 seconds it owns
   nothing ("no partition: idle"), then the broker takes one lane from one of
   the others. Six partitions over four consumers is two, two, one and one.
   Reset stops it, and its lane goes back.
3. **The publisher forgets the partition key.** The order service restarts
   without setting the key. Follow one letter: its dots now go down
   different lanes to different consumers. The consumers print `GAP`, for
   example "O-... #1 then #3 (#2 went to another consumer)", and their rows
   flag it in red. Nothing errors and every consumer stays busy; the damage
   is order lost downstream. Reset restarts the publisher with the key.

## Try this

- Start **Fulfilment 4**, then stop **Fulfilment 1** from its node. Watch the
  lanes: the broker hands out only the partitions it must, and keeps every
  consumer within one partition of the others.
- Change `partition_count` in `cockpit/scenarios/partitioning/tf/queues.tf`
  to 3, press **Cleanup**, then **Play**. Three partitions over three
  consumers is one each; start Fulfilment 4 and it sits idle, because a
  partition belongs to one consumer at a time. The diagram still draws six
  lanes and marks the missing three as not on the broker. Set it back to 6
  when you are done.

Next: [Event Bridge](510-event-bridge.md).
