# Saga (630): design

Category 6, Error Handling Patterns, third pattern. Prefix `saga`.

**One idea.** A business transaction across services with their own data is
a chain of local steps joined by events. A failure later in the chain is
undone by compensating steps, also driven by events. The result is
consistent at the end, not at every moment.

**Story.** Orders, payments, inventory (choreography, nobody in charge):
`order created` -> payments takes the money -> `payment taken` -> inventory
reserves -> `stock reserved` (orders confirms) or `stock failed` -> payments
refunds -> `payment refunded` (orders cancels). Topics
`saga/shop/<entity>/<event>/v1/<order>`.

**Broker objects.** `q.saga.orders`, `q.saga.payments`, `q.saga.inventory`,
each subscribed to exactly the events its service reacts to; `cp-saga`,
`acl-saga`, usernames `svc-saga-orders`, `-payments`, `-inventory`.

**Apps.** `saga orders` (an order every 2 s; lists sagas open over 5 s),
`saga payments`, `saga inventory` (SIGUSR1 sold out, SIGUSR2 restocked).
Each acknowledges an event only after publishing its own.

**Play.** apply, build, inventory, payments, orders.

**Cards.**

1. *An item sells out mid-saga.* Payment taken, stock failed, refund
   (compensation), order cancelled. Reset restocks.
2. *Payments crashes mid-saga.* Sagas wait at the payment step, events
   queue on q.saga.payments; reset starts payments and the sagas complete
   late.

**Not shown.** Orchestration (a coordinator service), and a compensation
that itself fails: both named in the guide.
