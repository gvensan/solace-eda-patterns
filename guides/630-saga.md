# Saga

Placing an order in a shop takes three services, each with its own
database: orders, payments and inventory. In one database this would be
one transaction. Across three, there is none.

A saga is how it is done instead: **each service takes its own step and
publishes an event, and the next service reacts to it. When a step fails,
the steps before it are undone by compensating steps, driven by events
too.** No step is rolled back; each is reversed by a new action.

Press **Play**. It creates a queue per service and the usernames, starts
**Inventory** and **Payments**, then **Orders**, which creates an order
every 2 seconds.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Discard, Pause and Retry**: one consumer deciding what to do with a
  message it cannot process. A saga is about a business outcome across
  several services, and undoing the parts already done.
- **Dead Message Queue**: parks a message nobody could handle. A saga's
  failure is a normal business event ("stock failed") that other services
  act on.

## Run it

Follow one order. **Orders**: "O-001234 created (40.00): saga started".
**Payments**: "O-001234: took 40.00". **Inventory**: "O-001234: stock
reserved". **Orders** again: "O-001234 confirmed: paid and stock reserved
after 0.1 s". Four events, three services, and no service called another.

## On the broker

- **Queues**: one per service, near empty while all three keep up.
- **What payments reacts to**: two subscriptions, "order created" and "stock
  failed". The second is the compensation's trigger. Each service's
  subscriptions are its part of the saga.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **An item sells out mid-saga.** Inventory has nothing left. Each order is
   paid first, then its reservation fails, then payments refunds:
   "refunded 40.00 (compensation: the stock failed)". Orders logs
   "cancelled: out of stock, payment refunded". **Reset** restocks.
2. **Payments crashes mid-saga.** Orders keep being created, and nothing
   comes back: "waiting on O-001240 (8 s), ...". The "order created" events
   wait on q.saga.payments. **Reset** starts payments, and every waiting
   saga completes, late.

## Try this

- Use **Sell the stock out** from Step through it, wait, then **Restock**.
  Count the refunds in the payments log against the cancellations in the
  orders log: they match one for one.
- Stop **Inventory** for 10 seconds. Payments carries on taking money, and
  the sagas wait at the next step instead.

## How Solace does this

- Each service has its own queue subscribed to the events it reacts to. A
  service that is down delays only its step, and its events wait for it.
- Each service acknowledges an event only after publishing its own, so a
  crash between the two repeats the step. Real services make each step
  idempotent for that reason.
- This saga is choreographed: nobody is in charge. The other style,
  orchestration, has one service send each step a command and track the
  saga's state, which is easier to follow when there are many steps.
- A compensation can fail too (the refund is refused). Real systems retry
  compensations until they succeed, and alert a person when they cannot.

Next: [Access Control and Authorization](750-access-control.md).
