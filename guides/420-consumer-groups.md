# Consumer Groups

A product catalogue. Every price change goes to two teams: **search**
updates the search index, and **alerts** emails customers watching a
product. Each team needs every price change, and within a team the members
share the work.

That is a consumer group: **several groups each receive every message,
and within a group the members share them.** Each group keeps its own pace
and its own backlog.

**Solace has no consumer group object.** Here each group is its own queue,
`q.cg.search` and `q.cg.alerts`, both subscribed to the same topic, and the
group's members share their queue. Kafka has consumer groups built in; on
Solace, adding a group means adding a queue.

Press **Play**. It creates both queues, starts two search members and one
alerts member, then the **Catalogue**, which publishes four price changes a
second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Competing Consumers** is one group: the members share one queue. Here
  there are several groups, and each gets its own copy of every message.
- **Partitioning** keeps per-key order within one set of consumers. Each
  group's queue here is partitioned too, so a group sees a product's changes
  in order, but the point is that the groups are independent.

## Run it

Open a search member's log and the alerts member's log. Every price change
appears once in the search group, at one of its two members, and once in
the alerts group: "SKU-07 #12 now 41.20 (alerts group)". The two search
members split the products between them; the one alerts member, for now,
has them all.

## On the broker

- **Queues, one per group**: each queue's `spooledMsgCount` grows at the
  same rate. The broker stores a copy of every price change on each.
- **Which member owns which partition**: each queue has two partitions. Search
  1 and Search 2 own one each; Alerts 1 owns both of its queue's.
- **Members of the alerts group**: a member that owns a partition reads
  `active-consumer`; one that owns none reads `inactive`, a standby (see the
  second card).

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The alerts group goes offline.** Every member of it stops. The card shows the
   search group, which carries on as if nothing happened. In the diagram
   `q.cg.alerts` starts counting price changes waiting; `q.cg.search` does
   not. **Reset** starts the alerts member again: it works through
   everything it missed, in order per product, each change "late by" how
   long it waited. The alerts group misses nothing; it is just late.
2. **The alerts group adds more members than it has partitions.** Alerts 2
   starts and, once the broker has given it a partition, Alerts 3 starts
   too: three members, two partitions. Alerts 3 binds without any error and
   receives nothing. The diagram shows it "standby", in a dashed box: the
   broker reports it inactive, because both partitions are taken. It takes
   one over if another member goes away. **Reset** stops Alerts 2 and 3.

## Try this

- Stop **Search 1**. After the 5 second rebalance delay, Search 2 owns both
  of the search queue's partitions and handles every price change for the
  group (it may print a GAP for products it now meets part way: the changes
  in between went to Search 1). The alerts group does not notice.
- Add a third group: add `"billing"` to `cg_groups` in
  `cockpit/scenarios/consumer-groups/tf/queues.tf` and press **Play**.
  `q.cg.billing` appears under **Queues, one per group**, and its
  `spooledMsgCount` grows with every price change, though it has no members
  yet: the group's backlog builds until someone reads it. Remove
  `"billing"` and press **Play** again to delete the queue (Clear broker
  config does not know about it).

## How Solace does this

- A queue per group: the broker stores a copy of each message on every
  queue whose subscription matches, and each queue keeps its own backlog.
- Within a group, the queue's access type decides how members share:
  non-exclusive (any member takes any message), partitioned (per-key order,
  as here) or exclusive (one active member, the rest standing by).
- For direct messages, with nothing stored, Solace has shared subscriptions:
  members that subscribe as `#share/<group>/<topic>` (the Go API's
  `BuildWithShareName`) share the stream. Measured: two members of one share
  group split 150 messages (81 and 69), and the one member of another share
  group received all 150 (`docs/spikes.md`, section 14).

Next: [Shock Absorber](430-shock-absorber.md).
