# Partitioning

Each order moves through **created**, **paid**, **packed**, **shipped** and
**delivered**, and fulfilment must see one order's updates in that order:
packing an order that has not been paid for is a real mistake. Many orders
are in flight at once, so one consumer is too slow and two share the work.

That is partitioning: **messages with the same key always go to the same
consumer, in order, while messages with different keys are handled in
parallel.** Here the key is the order id.

Press **Play**. It creates the partitioned queue `q.part.orders` (four
partitions), starts **Fulfilment 1** and **Fulfilment 2**, then the **Order
Service**, which moves eight orders along at a time.

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

Open both consumers' logs from their nodes. Every line says the order and its
key. An order id appears in only one of the two logs, and its statuses run
created, paid, packed, shipped, delivered without a gap. The two consumers
work on different orders at the same time.

Under **On the broker**, open **Partitions of q.part.orders**. The broker
keeps each partition as an internal queue of its own (`#pq/...`). The key
picks a partition, and each partition belongs to one consumer. The four
partitions fill unevenly, because keys are hashed onto partitions rather
than dealt out in turn.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you to
look for, then press **Reset**. **Why this breaks** explains the cause and
where you would meet it in production.

1. **A consumer crashes, and its orders move.** One consumer is killed (the
   diagram marks it). The card shows the other one: after the rebalance
   delay of 5 seconds it starts receiving orders it has never seen, part way
   through, "picked up O-... at paid". Those are the crashed consumer's
   partitions. Every order still moves one step at a time. Reset starts the
   crashed consumer and the partitions are shared out again.
2. **The publisher forgets the partition key.** The Order Service restarts
   without setting the key. Within seconds the consumers print `SPLIT`: one
   order's updates now go to both, so each sees gaps. Nothing errors and
   every consumer stays busy; the damage is order lost downstream. Reset
   restarts the publisher with the key.

## Try this

- Stop Fulfilment 2 and watch the queue's `partitionRebalanceStatus` under
  **Queue** while the broker moves its partitions, then start it again.
- Change `partition_count` in `cockpit/scenarios/partitioning/tf/queues.tf`
  to 2, press **Cleanup**, then **Play**. With two partitions, a third
  consumer would get nothing: a partition belongs to one consumer at a time,
  so there is no point running more consumers than partitions.

Next: [Dead Letter Queue](610-dead-letter-queue.md).
