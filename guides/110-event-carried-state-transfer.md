# Event-Carried State Transfer

A shop's shipping service needs the customer's address for every parcel.
It could ask the customer service each time. Or the customer service could
publish the whole customer record whenever a customer changes, and shipping
could keep its own copy.

That is event-carried state transfer: **the event carries the state that
changed, so a consumer can keep its own copy and never has to ask.** The
customer service owns the data; everyone else keeps a copy from its events.

Press **Play**. It creates two queues and the usernames, starts **Shipping**
and **Invoicing**, then the **Customer Service**, which publishes every
customer record once, and the **Order Service**, an order a second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

To hear the shop's story first, press **Listen: the use case** in the
top bar: a spoken overview with its transcript, about a minute and a half.

## How it differs from its neighbours

- **Request-Reply**: the consumer asks the owner each time it needs the
  data. Invoicing here does exactly that, so you can compare.
- **Event Sourcing**: the events are the only record, and the state is
  rebuilt from them. Here the customer service has its own database; the
  events only spread copies of it.

## Run it

What to do, in the order the page shows it:

1. **Run it**: press **Play**, then read **Follow a message** under the
   diagram. Click a path's name, or any numbered line, to highlight that
   message's whole route. **Expand** at the top right fills the window
   with the diagram.
2. **What each app holds**, under that: compare Customer Service and
   Shipping.
3. **Break it**: open a tile, press **Break it**, watch, press **Reset**.
4. **On the broker**: the queues and clients as the broker reports them.

Start with what the two kinds of event carry. Open the **Customer
Service** log and the **Order Service** log: each prints the event it
publishes.

| Event | Topic | Payload |
| --- | --- | --- |
| A customer changed | `ecst/crm/customer/updated/v1/C-03` | `{"customer":"C-03","version":4,"name":"Cy Diaz","city":"Porto"}` |
| An order was placed | `ecst/shop/order/placed/v1/C-03` | `{"order":"O-10012","customer":"C-03"}` |

Here the customer event carries the whole record, every time, not just the
field that changed, so one missed event cannot leave other fields wrong.
Carrying only the changed fields is also event-carried state transfer; the
point is that the event carries enough state that consumers need not ask. The order names the
customer and nothing more. To ship it, shipping looks C-03 up in its own
copy; invoicing has to ask. The `version` lets a consumer tell a newer
record from an older one.

Open the **Shipping** log: "copy of C-03 updated: Porto (version 4)" for
each customer record, and "O-10012 for C-03: ships to Porto (from my copy,
version 4)" for each order. It never asks anyone. Open the **Invoicing** log:
"O-10012 billed to Cy Diaz in Porto (asked the customer service, 3 ms,
version 4)". Every order costs it a request and a reply. The **Customer
Service** log shows a customer moving every 15 seconds; it publishes the
whole record each time, not just the new city. It keeps its customers in
its own database, so a restart carries on from the last version of each.

**What each app holds**, under the diagram, shows the three side by side:
the customer database (on disk), shipping's copy (in memory), and
invoicing, which keeps no copy and counts the lookups it makes instead. A
row that just changed is green. Watch a customer move: the row changes in
the database, then in shipping's copy.

## On the broker

- **Queues**: q.ecst.shipping takes customer records and orders, in the
  order the broker received them, so shipping's copy is current when an
  order arrives. q.ecst.invoicing takes orders only.
- **Connected clients**: `dataRxMsgCount` is what the broker received from
  each app. Invoicing's climbs with every lookup it sends. Shipping's stays
  at 0: it publishes nothing, it only acknowledges. (`dataTxMsgCount` is
  what the broker sent to it.)

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The customer service goes down.** It is killed. Invoicing's lookups
   time out after 2 seconds each, and it keeps asking about the same order:
   no invoices, and the backlog on q.ecst.invoicing grows. Shipping carries
   on from its copy, which stays full under the diagram, while invoicing's
   failed count climbs. **Reset** starts the customer service, which opens its
   database where it stopped, and invoicing catches up.
2. **Shipping restarts and has lost its copy.** Its copy was in memory.
   Orders for customers it has no copy of are held: "no copy of C-05 yet,
   parcel held". Under the diagram its copy drops to 0 customers and refills
   only as customers change, with the held parcels listed. The events that built the copy were acknowledged and are
   gone from its queue. A customer comes back by itself only when it next
   changes: here about every two minutes, in production perhaps months.
   **Reset** asks the customer service to publish every record again, and
   the held parcels ship. The held orders were acknowledged and kept only
   in shipping's memory, so a second restart before then would lose them.

## Try this

- Press **Ask the customer service to publish every record again** from
  Step through it while everything runs. Invoicing's queue does not
  subscribe to customer records, so only shipping gets them.
- Stop the **Customer Service** with its node, wait, and start it again.
  Invoicing is stuck while it is away. Shipping carries on. On start the
  customer service publishes every record again at its current version, so
  shipping's copy does not change.
- Compare versions. When the **Customer Service** log says "C-03 moved to
  Porto (version 4)", invoicing's next invoice for C-03 says Porto, version
  4, because it asked the owner just then. Shipping's copy says version 4
  only once the record has come through its queue: here a few
  milliseconds later. Between the two, the copy is behind the owner. When
  shipping is slow or has a backlog, that gap is as long as the backlog:
  that is the cost of keeping a copy.
- Shipping checks the version on every record. If a record older than its
  copy ever arrives, it logs "ignored C-03 version 1: my copy already has
  version 4" and keeps its copy.

## How Solace does this

- The events are ordinary guaranteed messages; the pattern is in what they
  carry. A topic per entity (`ecst/crm/customer/updated/v1/C-03`) lets a
  consumer subscribe to all customers, or to one.
- One queue with both subscriptions keeps the two streams in the order the
  broker received them. They come from two services, so there is no shared
  publish order, but the customer service waits for the broker to confirm
  each record, so an order placed after a change is behind it on the queue.
  Two queues would not keep even that: an order could overtake the record
  it needs.
- A copy needs a way to start. Common ones: a bulk load, a "publish
  everything again" request as here, or a durable copy in the consumer's
  own database so a restart keeps it.

Next: [CQRS](120-cqrs.md).
