# Request-Reply (230): design

Category 2, Communication Patterns, third pattern. Prefix `rr`. Direct
messaging only: the Go API supports request-reply for direct messages and
nothing else (Request-Reply Messaging in the Solace Go API).

**One idea.** Sometimes an app needs an answer, not just to tell the world
something. It publishes a request and waits for a reply addressed back to
it, with a timeout. On Solace that is two direct messages: the request goes
to whoever is subscribed to the request topic now, and the reply comes back
on a topic the API made for the asker. No reply within the timeout is an
answer too, and the asker is told.

**Story.** A supermarket till needs the price of an item before it can ring
it up. It asks `rr/pricing/price/request/v1/<sku>` and waits up to 2
seconds. The pricing service answers every request in about 20 ms.

**Broker objects.** No queues. `cp-rr`, `acl-rr`, usernames `svc-rr-tills`,
`svc-rr-pricing`.

**Apps.** `rr till` (one app, a request a second, logs each reply with who
answered and how long it took, or the API's error). `rr pricing` (one app;
`--work` sets how long an answer takes; `--role pricing-2` for a second
instance).

**Play.** apply, build, pricing, till.

**Cards.**

1. *The pricing service goes down.* Pricing is killed (SIGKILL). The till's
   requests get no reply and, after 2 seconds each, the API's timeout error.
   The till keeps asking; nothing is queued for pricing, because a direct
   request is delivered only to a subscriber that is there. Reset starts
   pricing and the next request is answered. `expect:` break: till
   `has: ["request #\d+ for SKU-\d+: .*imeout"]` (exact text from the run).
   Reset: till `has: ["answered by pricing in \d+ ms"]`.
2. *The pricing service is slow.* Pricing restarts with `--work 3s`, longer
   than the till waits. Every request times out at the till, and a second
   later pricing logs that it replied: the reply arrives after the asker
   gave up, and the API discards it. Reset restarts pricing at normal speed.
   `expect:` break: till `has: ["imeout"]`, pricing `has: ["replied"]`.
   Reset: till `has: ["answered by pricing"]`, `lacks: ["imeout"]` after
   `settle:`.

**Try this.** Start `pricing-2` beside pricing: both subscribe to the
request topic, so both answer every request. What the till sees is to
measure (spike below) and the guide says what was found.

**Differs from neighbours.** Publish-Subscribe: no answer expected.
Point-to-Point: one receiver, with a queue; here the asker needs a reply,
and nothing is queued.

**Spike.** With two repliers on one request topic, does the till's reply
handler run twice, or does the API take the first reply and drop the
second? Also the exact timeout error text. Run after the current
regression finishes.
