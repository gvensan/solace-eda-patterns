# Publish-Subscribe (210): design

Category 2, Communication Patterns, first pattern. Prefix `ps`. Direct
messaging throughout.

**One idea.** A publisher sends an event to a topic once; every subscriber
to that topic gets its own copy. The publisher does not know who listens,
and a new listener needs no change at the publisher. Direct messages are
kept for no one: a subscriber that is away misses what was published.

**Story.** An online shop publishes an event each time an order is placed,
to `ps/shop/order/placed/v1/<region>/<order>`. Shipping and email subscribe
to every order (`ps/shop/order/placed/v1/>`); analytics subscribes to the
EU region only (`ps/shop/order/placed/v1/eu/>`). Two orders a second.

**Broker objects.** No queues. `cp-ps`, `acl-ps`, usernames `svc-ps-shop`,
`svc-ps-shipping`, `svc-ps-analytics`, `svc-ps-email`.

**Apps.** `ps shop` (one app). `ps subscriber --role <name> --topic <sub>`
for the three subscribers, plus a second shipping instance for card 2. A
subscriber keeps the last event number it saw in a file, so after a restart
it can say how many it missed.

**Play.** apply, build, shipping, analytics, email, shop.

**Cards.**

1. *Shipping goes away for a while.* Shipping is killed (SIGKILL); Reset
   starts it again. Analytics and email carry on. On restart shipping says
   "missed #n to #m, k events published while it was away": a direct
   message is delivered to whoever is subscribed at that moment, and kept
   for no one. (Point-to-Point and Guaranteed Delivery are where a queue
   keeps them.) `expect:` break: email `has: ["event #"]`; reset: shipping
   `has: ["missed #\d+ to #\d+"]`.
2. *A second shipping instance is started to share the load.* Both shipping
   instances receive every order: pub/sub copies to every subscriber, so
   two subscribers mean two shipments. Reset stops the second. `expect:`
   break: shipping-2 `has: ["event #"]` and shipping `has: ["event #"]`
   with the same numbers (checked by a shared pattern on the same event
   number is not possible in smoke; both receiving is enough). Points the
   guide at Competing Consumers and shared subscriptions.

**Differs from neighbours.** Point-to-Point: one receiver, a queue that
keeps messages. Hierarchical Topics: the same mechanism, with the topic
levels as the contract. Event Streaming: the stream kept in order with a
position.

**Spike.** None: direct pub/sub on topics is documented and already used
by the core workshop.
