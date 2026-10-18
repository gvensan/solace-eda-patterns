# Strangler (810): design

Category 8, Migration Patterns, first pattern. Prefix `stg`.

**One idea.** Replace an old system one capability at a time by moving the
events for that capability to the new service. With a broker in between,
the move is a subscription change: publishers and the old system are not
touched.

**Story.** A shop publishes order placed, order paid and refund requested
(`stg/shop/order/placed/v1/<id>`, `.../paid/...`,
`stg/shop/refund/requested/v1/<id>`). The old order system reads
`q.stg.legacy`; the new refund service reads `q.stg.refunds`.

**Broker objects.** `q.stg.legacy` (all three, in terraform), `q.stg.refunds`
(none). The refund subscription is moved by the scenario's `route.sh` over
SEMP, the way an operator or pipeline would; Play's `route-start` step puts
the starting routing back, since terraform does not track the subscription
added to q.stg.refunds. `cp-stg`, `acl-stg`, usernames `svc-stg-shop`,
`-legacy`, `-refunds`.

**Apps.** `stg shop`, `stg legacy`, `stg refunds`.

**Play.** apply, route-start, build, legacy, refunds, shop. Step through it
offers "Step 1: route refunds to the new service" and "Step 2: stop routing
refunds to the old system".

**Cards.**

1. *Refunds go to both systems.* Step 1 without step 2: the same refund in
   both logs. Reset: old system only.
2. *The new service fails after the cut-over.* Cut over (old route off
   first), kill the new service: refunds wait on q.stg.refunds, the old
   system gets none. Reset starts the new service (drains), routes back.
