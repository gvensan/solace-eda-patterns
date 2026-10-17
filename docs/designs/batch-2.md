# Batch 2 design briefs

Status: for review. No scenario code is written until these are approved.
Date: 2026-10-16

Four wave 1 scenarios: Guaranteed Delivery (330), Backpressure and Push
(340), Consumer Groups (420) and Shock Absorber (430). Each brief gives the
one idea, how it differs from the scenarios already in the repo and from the
rest of this batch, the story, the broker objects and apps, two failure
modes with their `expect:` checks, what to spike first, and what the guide
will say about how Solace does it.

Scenarios already in the repo, for reference: Point-to-Point (220),
Exclusive Consumer (350), Competing Consumers (410), Partitioning (440), Dead
Letter Queue (610). They use one online shop (labels, ledger, emails, order
status, order intake). Each brief here uses a new domain.

Conventions carried over from batch 1: prefixes on every object and topic,
role-based cards where instances are interchangeable and a named app where
there is only one, a crash is a SIGKILL, every indicator explains itself, the
page never shows work that is not happening, every card has an `expect:`
block, and Go API facts are taken from the v1.10.0 source, not assumed.

---

## Guaranteed Delivery (330)

Category 3, Consumption Patterns, third pattern. Prefix `gd`.

### The one idea

Once the broker has confirmed a message, it is not lost and not skipped,
even if a consumer fails part way through. The publisher knows which
messages the broker accepted, and because delivery is at least once, the
consumer must be safe to receive the same message twice.

Three parts, each shown on the page: the publisher's confirmation (the
broker's acknowledgement of each message), the consumer's acknowledgement
(the message is deleted only after it), and an idempotent consumer (a
redelivered message is recognised and not applied again).

### How it differs from its neighbours

- **Dead Message Queue**: what happens when a message can never be processed.
  Guaranteed Delivery is about a message that can be, not being lost or
  doubled on the way.
- **Exclusive Consumer, Competing Consumers**: who receives. Here there is one
  consumer; the lesson is the contract between publisher, broker and
  consumer.
- **Point-to-Point**: showed a queue holding messages and refusing them when
  full. Guaranteed Delivery shows the acknowledgements on both sides that
  make "held" mean "will be delivered".
- **Backpressure** (this batch): the consumer controlling its intake.
  **Shock Absorber** (this batch): the queue soaking up a burst. Neither is
  about loss or duplicates.

### Story

A loyalty programme. Every purchase earns points, and the points service
awards them. Losing an award cheats a customer; awarding it twice costs the
business. Both are visible in the customer's balance.

### Broker objects

- Queue `q.gd.awards`, exclusive, subscription `gd/loyalty/points/earned/v1/>`,
  `max_delivered_unacked_msgs_per_flow` 5.
- Client profile `cp-gd`, ACL profile `acl-gd`, usernames `svc-gd-tills`,
  `svc-gd-points`.

### Apps and roles

- **Tills** (`gd tills`, one app): publishes one "points earned" event per
  purchase, two a second, guaranteed, each with an application message id
  (`purchase id`) and a running sequence number. It publishes asynchronously
  and tracks confirmations with the publish receipt listener (the context
  carries the purchase id), so its log shows "confirmed P-1234" per message.
  Anything not yet confirmed stays in an outbox and is resent if the broker
  refuses it.
- **Points service** (`gd points`, one app, client name `gd-points`): binds
  the queue with client acknowledgement, applies the award to a small balance
  store, then acknowledges. It keeps the ids it has applied in a file under
  `COCKPIT_STATE_DIR`, so it remembers them across a crash; a message whose
  id it has already applied is acknowledged and skipped ("already awarded,
  skipped"). It checks the sequence for gaps.

Both are single apps, so the cards name them.

### Play

apply, build, points, tills.

### Failure modes

1. **The points service crashes after awarding, before acknowledging.** A
   failure-only action sends the points service a signal that makes it exit
   immediately after applying the next award and before acknowledging it
   (the narrow window that causes duplicates). The service is then started
   again (reset, or a trigger step).
   - Shows: the broker redelivers that message, marked redelivered; the
     idempotent consumer recognises the id and skips it. The balance is
     right. Behaviour, not an error: the point is what does not go wrong.
   - `expect:` break: points `has: ["exited with code"]`. Reset (restart):
     points `has: ["\\(redelivered\\).*already awarded, skipped"]`,
     `lacks: ["GAP"]`.
   - Try this: restart the points service with `--no-dedupe`, repeat, and see
     the same award applied twice ("awarded P-... again").

2. **The broker refuses messages for a while.** A failure-only action turns
   off `ingress_enabled` on the queue over SEMP (the queue stops accepting
   messages; nothing is restarted). Reset turns it back on.
   - Shows: the tills' publish receipts carry the broker's refusal (expected
     text to confirm in the spike, likely a queue shutdown error); the tills
     keep the refused purchases in the outbox and resend them once the queue
     accepts again. The points service sees no gap: every purchase arrives.
   - `expect:` break: tills `has: ["refused P-", "kept for retry"]`. Reset:
     tills `has: ["resent P-"]`; points `has: ["awarded"]`,
     `lacks: ["GAP"]`, with `settle:` a few seconds.

### Spike first

- Publish to a queue with `ingress_enabled` false: the receipt error text,
  and whether it comes through the receipt listener with its context.
- `PublishReceipt` with a context from `Publish(..., context)`: confirm the
  context round-trips, and whether receipts arrive in order.
- Application message id (`WithApplicationMessageID`,
  `GetApplicationMessageID`): confirm it survives the broker and a
  redelivery.
- SIGKILL between apply and ack: confirm the message comes back marked
  redelivered on restart of an exclusive-queue consumer.

Run with the spike tool against 10.26.0, as in batch 1, and record in
`docs/spikes.md`.

### How Solace does this (for the guide)

- The publisher chooses per message: direct (best effort, no
  acknowledgement) or persistent, also called guaranteed (the broker stores
  it before confirming). The same topic can carry both.
- The broker confirms a persistent message once it has stored it. The
  consumer's acknowledgement is what removes it from the queue.
- Delivery is at least once. After a failure a message can arrive again,
  flagged as redelivered, so the consumer's idempotency is the
  application's job. The guide will check Solace's documentation for the
  exact wording on duplicates before stating it.
- Other brokers: Kafka consumers commit offsets rather than acknowledging
  messages one by one, and Kafka has an idempotent producer and
  transactions; RabbitMQ has publisher confirms and consumer acks, close to
  Solace. The guide will keep the comparison to what the workshop shows.

---

## Backpressure and Push (340)

Category 3, Consumption Patterns, fourth pattern. Prefix `bp`.

### The one idea

The broker pushes messages to consumers, so a consumer needs a way to say
"not so fast". With guaranteed messages, the broker never sends a consumer
more than it can hold unacknowledged, and the consumer can pause its own
intake; what it cannot take waits on the queue, not in its memory. With
direct messages there is no such brake: the consumer's own buffer fills and
it must choose what to drop.

### How it differs from its neighbours

- **Shock Absorber** (this batch): the queue soaks up a producer's burst.
  Backpressure is the consumer controlling how fast it takes from the queue.
- **Competing Consumers**: adds consumers to go faster. Backpressure keeps
  one consumer safe when it cannot.
- **Competing Consumers' and Exclusive Consumer's hang cards**: a frozen
  consumer hits the unacknowledged limit by accident. Here a slow consumer
  uses the limit, and pause and resume, on purpose.
- **Guaranteed Delivery** (this batch): about not losing messages; here the
  question is pacing.

### Story

Cold-chain monitoring in a refrigerated warehouse. Temperature sensors report
40 readings a second. A **reporter** writes every reading to a compliance
database, which slows down at times, and must not lose any. A **live
display** on the warehouse wall shows the current temperatures; it only cares
about the latest reading.

### Broker objects

- Queue `q.bp.readings`, exclusive (one reporter), subscription
  `bp/coldchain/temperature/v1/>`, `max_delivered_unacked_msgs_per_flow` 20.
- No queue for the display: it takes direct messages.
- Client profile `cp-bp`, ACL profile `acl-bp`, usernames `svc-bp-sensors`,
  `svc-bp-reporter`, `svc-bp-display`.

### Apps and roles

- **Sensors** (`bp sensors`): publishes 40 readings a second to
  `bp/coldchain/temperature/v1/<zone>/<sensor>`. Direct publishing, so both
  consumers can take the same stream: the queue's subscription turns them
  into guaranteed messages on `q.bp.readings` (to confirm in the spike), and
  the display receives them directly.
- **Reporter** (`bp reporter`, one app, client name `bp-reporter`): binds the
  queue with client acknowledgement and hands each reading to a simulated
  database writer with its own queue of pending writes. Above a high-water
  mark of pending writes it calls `Pause()` on the receiver and logs it;
  below a low-water mark it calls `Resume()`. It acknowledges a reading only
  once it is written.
- **Display** (`bp display`, one app): a direct receiver built with
  `OnBackPressureDropOldest(20)`, rendering one reading at a time. It reports
  the API's `ReceivedMessagesBackpressureDiscarded` metric as it changes and
  how old the reading it shows is. A `--drop latest` flag builds it with
  `OnBackPressureDropLatest(20)` instead.

### Play

apply, build, reporter, display, sensors.

### Failure modes

1. **The database slows down.** A signal makes the reporter's writes ten times
   slower. Reset (the same signal) restores them.
   - Shows: pending writes reach the high-water mark and the reporter pauses
     ("paused with 200 writes pending"). The broker stops sending once 20
     readings are unacknowledged (On the broker: `unackedMsgCount` 20 on the
     reporter's flow), and the backlog grows on the queue (`msgSpoolUsage`),
     not in the reporter. After Reset it resumes, catches up, and no reading
     is missing.
   - `expect:` break: reporter `has: ["paused with \\d+ writes pending"]`.
     Reset: reporter `has: ["resumed", "caught up"]`, `lacks: ["GAP"]`.

2. **The live display falls behind.** A signal makes the display render ten
   times slower. Reset restores it.
   - Shows: there is no queue and no broker-side brake for direct messages;
     the API's buffer of 20 fills, and with drop-oldest the API discards the
     oldest readings to make room. The display reports
     "dropped 312 readings (oldest first)" and keeps showing readings less
     than a second old: on a status display that is the right trade.
   - `expect:` break: display `has: ["dropped \\d+ readings"]`. Reset with
     `settle:` a few seconds: display `lacks: ["dropped"]`.
   - Try this: restart the display with `--drop latest`. Under the same
     slowdown it keeps rendering old readings and drops the new ones, so the
     reading on screen grows stale ("reading is 14s old").

Both apps are single, so the cards name them.

### Spike first

- `Pause()` and `Resume()` on a persistent receiver: confirm the handler stops
  and that the broker stops at the flow's unacknowledged limit (the flow's
  `unackedMsgCount` on the queue's `txFlows`).
- Direct publishing to a topic a queue subscribes to: confirm the queue
  stores them as guaranteed messages, and what the reporter's messages look
  like.
- `OnBackPressureDropOldest` and `OnBackPressureDropLatest` on a direct
  receiver with a slow handler: confirm the
  `ReceivedMessagesBackpressureDiscarded` metric counts the drops, and which
  readings survive.
- What the broker does with a direct consumer that falls far behind beyond
  the API's buffer (TCP backs up): look for egress discard counters on the
  client in the monitor tree, so the guide can say what happens and not
  guess.

### How Solace does this (for the guide)

- Solace pushes messages to consumers rather than consumers pulling them.
  For guaranteed messages, flow control between broker and consumer is a
  window of unacknowledged messages. The Go API has no window-size setting,
  so the scenario uses the queue's `max_delivered_unacked_msgs_per_flow`,
  which the broker enforces per flow, and the receiver's `Pause()` and
  `Resume()`.
- For direct messages there is no per-message flow control. The Go API
  buffers incoming messages and, when the buffer is full, drops the oldest
  (`OnBackPressureDropOldest`) or the latest (`OnBackPressureDropLatest`).
- The publisher has its own backpressure setting
  (`OnBackPressureWait` or `OnBackPressureReject` on its buffer), for when
  the broker or network cannot take messages as fast as it publishes; the
  guide mentions it, the scenario does not exercise it.
- Other brokers: Kafka consumers pull, so a slow consumer simply falls behind
  on the log; RabbitMQ's prefetch count plays the role of the
  unacknowledged limit.

---

## Consumer Groups (420)

Category 4, Consumer Scalability Patterns, second pattern. Prefix `cg`.

### The one idea

Several independent groups each receive every message, and within each group
the members share the work. Each group keeps its own pace and its own
backlog; one group falling behind or going away does not affect another.

**Solace has no first-class consumer group.** This scenario models each group
as its own queue, all subscribed to the same topic, with the group's members
sharing that queue. The guide says so on its first page.

### How it differs from its neighbours

- **Competing Consumers**: one group. Consumer Groups is several groups, each
  getting its own copy of every message.
- **Partitioning**: order per key within one set of consumers. Here each
  group's queue is partitioned so a group keeps per-key order, but the lesson
  is the groups being independent.
- **Point-to-Point**: one receiver per message. Here each group gets the
  message, and one member of each group handles it.
- **Shock Absorber** (this batch): one queue soaking up a burst. Here several
  queues each absorb at their own pace.

### Story

A product catalogue. Every price change goes to two teams: **search**, which
updates the search index (two members), and **alerts**, which emails
customers watching a product (one member at Play). Each team needs every
price change, and within a team the work is shared.

### Broker objects

- Topic `cg/catalog/price/changed/v1/<sku>`, partition key the SKU.
- Queue `q.cg.search`: non-exclusive, `partition_count` 2, subscription
  `cg/catalog/price/changed/v1/>`. Two members, two partitions.
- Queue `q.cg.alerts`: non-exclusive, `partition_count` 2, same subscription.
  One member at Play, a second available.
- Short rebalance delay and handoff, as in Partitioning.
- Client profile `cp-cg`, ACL profile `acl-cg`, usernames `svc-cg-catalog`,
  `svc-cg-search`, `svc-cg-alerts`.

Partition counts follow the measured fact that consumers beyond a queue's
partition count sit idle: each group's partition count equals the members it
is meant to run (2), and the second card shows what a member beyond that
does.

### Apps and roles

- **Catalog** (`cg catalog`): four price changes a second across 20 SKUs,
  keyed by SKU, each with a sequence per SKU and a timestamp.
- **Group member** (`cg member --group search|alerts`), run as search-1,
  search-2, alerts-1, alerts-2 and alerts-3, each with `--client-name
  cg-<group>-<n>`. Prints each change it handles with its group, and how late
  it is ("late by 23s") when it is more than a few seconds behind.

Members within a group are interchangeable, so cards use roles with a
`queue:` per group.

### Play

apply, build, search-1, search-2, alerts-1, catalog.

### Failure modes

1. **A whole group goes offline.** The alerts group's members are stopped.
   Reset starts them again.
   - Shows: search carries on as if nothing happened; `q.cg.alerts` holds every
     price change for the alerts group (`msgSpoolUsage` grows on that queue
     only). After Reset alerts catches up on every change it missed, late but
     complete, in order per SKU.
   - `expect:` break: a search member `has: ["changed"]` (unaffected). Reset:
     an alerts member `has: ["late by \\d+s"]`, `lacks: ["SPLIT", "OUT OF
     ORDER"]`.
   - Roles: `{role: active, queue: q.cg.search}` for the search log and
     `{role: active, queue: q.cg.alerts}` for alerts.

2. **A group adds more members than it has partitions.** Alerts-2 and alerts-3
   are started, so the alerts group has three members on a queue with two
   partitions. Reset stops alerts-3.
   - Shows: alerts-3 binds without any error and receives nothing; the two
     partitions are owned by the other two. Nothing fails; adding the member
     simply does not help.
   - `expect:` break: alerts-3 `has: ["bound to queue"]`,
     `lacks: ["changed"]` over the window; alerts-2 `has: ["changed"]`.
     Reset: nothing to check beyond alerts-3 stopping (`has: ["stopped"]`).
   - Needs a shared change: the broker reports the idle member's flow as
     `active-consumer`, so the diagram would say "active consumer" with
     packets flowing to an app that gets nothing. The dashboard needs an
     "active, idle" reading (active flow, nothing acknowledged and nothing
     held for a few seconds) with no packets, under the rule that the page
     never shows work that is not happening.

### Spike first

- Two partitioned queues on the same topic: each receives every message
  (expected; quick to confirm).
- A third consumer on a two-partition queue: its `txFlows` entry, to confirm
  `activityState` and that `ackedMsgCount` and `unackedMsgCount` stay 0, which
  is what the "idle" reading would use.
- Direct shared subscriptions (`#share/<name>/<topic>`, the Go API's
  `BuildWithShareName`): confirm they share direct messages among members,
  for a Try this item and the guide.

### How Solace does this (for the guide)

- Kafka has consumer groups as a built-in concept: one stored log, and each
  group keeps its own position in it. Solace has no consumer group object.
  The equivalent is a queue per group: the broker stores a copy of each
  message per queue, every queue keeps its own backlog, and adding a group
  means creating a queue, rather than starting a consumer with a new group
  id.
- Within a group the queue's access type decides how members share:
  non-exclusive (any member takes any message), partitioned (per-key order,
  as here) or exclusive (one active member, the rest on standby).
- For direct messages, Solace has shared subscriptions
  (`#share/<name>/<topic>`), which share a stream among members with no queue
  and no persistence.

---

## Shock Absorber (430)

Category 4, Consumer Scalability Patterns, third pattern. Prefix `sa`.

### The one idea

A queue between a bursty producer and steady consumers turns a sudden surge
into a backlog that drains at the consumers' pace. The producer is never
slowed down or refused, and the consumers never see more than they can
handle. The queue has limits, in space and in time, and the cards show both.

### How it differs from its neighbours

- **Competing Consumers**: more workers to go faster. A shock absorber keeps
  the same workers and lets the queue hold the difference.
- **Point-to-Point**: a queue holding messages while a receiver is away. A
  shock absorber is about a receiver that is there but slower than a burst.
- **Backpressure** (this batch): the consumer pacing its own intake. A shock
  absorber is the queue taking the producer's surge so nobody needs to slow
  down.
- **Consumer Groups** (this batch): several queues for several groups. Here
  one queue absorbs.

### Story

Concert ticket sales. At 10:00 tickets go on sale and 1,500 purchase requests
arrive in a few seconds; otherwise requests trickle in. Two booking workers
reserve seats at a steady 20 a second each.

### Broker objects

- Queue `q.sa.purchases`, non-exclusive, subscription
  `sa/tickets/purchase/requested/v1/>`, `max_msg_spool_usage` sized to hold a
  burst with room to spare (to set from the spike), `respect_ttl_enabled`
  true.
- Client profile `cp-sa`, ACL profile `acl-sa`, usernames `svc-sa-onsale`,
  `svc-sa-booking`.

### Apps and roles

- **On-sale** (`sa onsale`): two requests a second, and every 60 seconds a
  burst of 1,500 as fast as it can publish. Each request carries a sequence
  number and its publish time. Its log shows the burst going out, and that
  it was never refused or slowed.
- **Booking worker** (`sa booking`), booking-1 and booking-2, client names
  `sa-booking-1` and `-2`: reserves a seat per request at about 20 a second,
  and prints how long the request waited ("waited 34s").

### Play

apply, build, booking-1, booking-2, onsale.

### Failure modes

1. **The burst is bigger than the queue.** A failure-only action triggers a
   much bigger burst than the queue was sized for.
   - Shows: the queue fills during the burst and the broker refuses the rest
     back to the on-sale app with `Spool Over Quota`; those requests are lost
     unless the publisher keeps them. The bookings carry on with what was
     accepted. The absorber's size is a decision, sized from the worst burst
     you expect.
   - `expect:` break: on-sale `has: ["refused: .*Spool Over Quota"]`. Reset
     (bursts return to normal size): on-sale `lacks: ["refused"]` after
     `settle:`.
   - Point-to-Point showed the same refusal from a receiver being away; here
     it is a burst outgrowing a queue that is being drained. The guide says
     so.

2. **Requests go stale in the queue.** A failure-only action sets a short
   time to live on purchase requests (for example 20 seconds: a seat hold
   that expires) and triggers a burst.
   - Shows: the workers cannot reach the back of the burst within 20 seconds,
     so the broker discards the oldest waiting requests as they expire. The
     workers never see them and report a gap in the sequence; the queue's
     monitor counts the expiries (counter name to confirm in the spike).
     Absorbing has a cost in time: a request that waits too long may no
     longer be worth handling, and a TTL makes that explicit.
   - `expect:` break: a booking worker `has: ["GAP: requests"]`. Reset: TTL
     removed, `lacks: ["GAP"]` after `settle:`.
   - Roles: the gap shows on whichever worker reaches past it, so the check
     reads `{role: active}` and the guide says either worker may report it.
     The spike confirms whether one or both do; if it is not deterministic,
     the expectation moves to the queue's expiry counter, read through a
     small reporting step that prints it.

### Spike first

- A burst of 1,500 to 20,000 persistent messages from one Go publisher: how
  fast it goes out (the Standard edition's ingress limit is 10,000 a second),
  and the spool size per message, to size `max_msg_spool_usage`.
- TTL expiry while queued with `respect_ttl_enabled`: that messages are
  discarded at expiry, the monitor counter that counts them, and that on
  10.26.0 they go to a DMQ only if one is configured.
- Which worker sees the gap after expiry, and whether that is stable.

### How Solace does this (for the guide)

- A queue is the absorber: persistent messages are stored on the broker until
  a consumer takes them, so producers publish at their own pace. Its size
  limit (`max_msg_spool_usage`) and the message spool overall bound how big a
  burst it takes; past that the broker refuses messages back to publishers
  (with the default `reject_msg_to_sender_on_discard_behavior`).
- Time to live is set per message by the publisher, and honoured by a queue
  with `respect_ttl_enabled`; the queue can also cap it (`max_ttl`).
- Other brokers: a Kafka topic absorbs bursts by retaining them on the log by
  time or size, and consumers read at their own pace; there is no refusal to
  the producer while the log has space, and old data ages out by retention,
  not per-message TTL.

---

## Shared changes this batch needs

- An "active, idle" flow reading for Consumer Groups (a consumer the broker
  reports active that has acknowledged and holds nothing), with no packets
  and its own hover text and guide row.
- A small per-app file store under `COCKPIT_STATE_DIR` for the Guaranteed
  Delivery points service's applied ids, removed by Cleanup.
- A toggle signal (SIGUSR2) for "slow down" and "crash after apply" in the Go
  helpers, alongside the existing freeze on SIGUSR1.
- Possibly a one-shot reporting action that prints a queue counter into a
  log, if a card's outcome is visible only in the monitor tree.

## Order of building

Guaranteed Delivery, Backpressure, Consumer Groups, Shock Absorber: the first
two are single-consumer and settle the new Go helpers; Consumer Groups needs
the "idle" reading; Shock Absorber depends most on spike numbers.
