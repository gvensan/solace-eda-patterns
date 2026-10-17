# Event-Carried State Transfer (110): design

Category 1, Event Generation Patterns, first pattern. Prefix `ecst`.

**One idea.** An event can carry the state that changed, not just the fact
that something changed. A consumer that keeps a copy of that state can work
without calling the owner. The cost is keeping the copy, and the copy being
a little behind.

**Story.** A shop. The customer service publishes the whole customer record
on every change (`ecst/crm/customer/updated/v1/<id>`). Orders carry the
customer id only (`ecst/shop/order/placed/v1/<id>`). Shipping keeps a copy;
invoicing asks the customer service by request-reply for every order, the
usual alternative, so the two can be compared side by side.

**Broker objects.** `q.ecst.shipping` (customer records and orders, one queue
so the copy is current when an order arrives), `q.ecst.invoicing` (orders).
`cp-ecst`, `acl-ecst`, usernames `svc-ecst-crm`, `-orders`, `-shipping`,
`-invoicing`.

**Apps.** `ecst customers` (publishes all 8 records at start, then one change
every 4 s; answers lookups; SIGUSR1 republishes all), `ecst orders`,
`ecst shipping` (copy in memory; holds an order whose customer it has no copy
of), `ecst invoicing` (blocks on each order until the lookup succeeds).

**Play.** apply, build, shipping, invoicing, customers, orders.

**Cards.**

1. *The customer service goes down.* Killed. Invoicing's lookups time out and
   it stops billing; q.ecst.invoicing grows. Shipping carries on from its
   copy. Reset starts the customer service.
2. *Shipping restarts and has lost its copy.* Its copy was in memory: orders
   for unknown customers are held. Reset asks the customer service to
   republish every record, and the held parcels ship.

**Real.** Every event, queue, lookup and timeout. **Not shown.** A durable
copy (a database): the guide names it as the usual fix.
