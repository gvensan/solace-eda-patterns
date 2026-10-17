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

## How it differs from its neighbours

- **Request-Reply**: the consumer asks the owner each time it needs the
  data. Invoicing here does exactly that, so you can compare.
- **Event Sourcing**: the events are the only record, and the state is
  rebuilt from them. Here the customer service has its own database; the
  events only spread copies of it.

## Run it

Open the **Shipping** log: "copy of C-03 updated: Lyon (version 2)" for each
customer record, and "O-10012 for C-03: ships to Lyon (from my copy, version
2)" for each order. It never asks anyone. Open the **Invoicing** log:
"O-10012 billed to Cy Diaz (asked the customer service, 3 ms)". Every order
costs it a request and a reply. The **Customer Service** log shows a
customer moving every 4 seconds; it publishes the whole record each time,
not just the new city.

## On the broker

- **Queues**: q.ecst.shipping takes customer records and orders, in the
  order they were published, so shipping's copy is current when an order
  arrives. q.ecst.invoicing takes orders only.
- **Connected clients**: invoicing's `dataTxMsgCount` climbs with every
  lookup it sends. Shipping's stays near zero: it only acknowledges.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The customer service goes down.** It is killed. Invoicing's lookups
   time out after 2 seconds each, and it keeps asking about the same order:
   no invoices, and q.ecst.invoicing fills up. Shipping carries on from its
   copy. **Reset** starts the customer service, and invoicing catches up.
2. **Shipping restarts and has lost its copy.** Its copy was in memory.
   Orders for customers it has no copy of are held: "no copy of C-05 yet,
   parcel held". The events that built the copy were acknowledged and are
   gone from its queue. **Reset** asks the customer service to publish
   every record again, and the held parcels ship.

## Try this

- Press **Ask the customer service to publish every record again** from
  Step through it while everything runs. Invoicing's queue does not
  subscribe to customer records, so only shipping gets them.
- Stop the **Customer Service** with its node, wait, and start it again.
  Shipping notices nothing. Invoicing is stuck while it is away.

## How Solace does this

- The events are ordinary guaranteed messages; the pattern is in what they
  carry. A topic per entity (`ecst/crm/customer/updated/v1/C-03`) lets a
  consumer subscribe to all customers, or to one.
- One queue with both subscriptions keeps the two streams in publish order
  for shipping. Two queues would not: an order could overtake the record it
  needs.
- A copy needs a way to start. Common ones: a bulk load, a "publish
  everything again" request as here, or a durable copy in the consumer's
  own database so a restart keeps it.

Next: [CQRS](120-cqrs.md).
