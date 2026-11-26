# Point-to-Point (220): design

Category 2, Communication Patterns, second pattern. Prefix `p2p`.
Guaranteed messaging to one durable queue.

**One idea.** Each message goes to one destination, and one receiver takes
it from there. The queue keeps a message until the receiver acknowledges
it, so the receiver can be away. Publish-Subscribe is the opposite: the
publisher names a topic and does not know who receives.

**Story.** The shop's order service sends a 25 KB label job every half
second, which lands on the label printer's queue, `q.p2p.labels`. The printer is its
one reader and prints the jobs in order.

**Addressing.** Topic to queue, the form Solace recommends (changed
2026-11-26; it was queue by name before). The sender publishes to
`p2p/label/requested` and the queue has that one subscription, so
the sender never names the queue. Addressing the queue by its network
topic, `#P2P/QUE/q.p2p.labels`, is described as the other way and offered
as a Try this (`p2p send --queue q.p2p.labels`); it is not a card.

**Broker objects.** Exclusive queue `q.p2p.labels`, 1 MB (about 40 jobs, so
an outage fills it in about 20 s), at most 5 unacknowledged per flow.
`cp-p2p` with `reject_msg_to_sender_on_no_subscription_match_enabled`, so a
job on a topic no queue subscribes to is refused, not dropped. `acl-p2p`,
usernames `svc-p2p-orders` and `svc-p2p-printer`.

**Apps.** `p2p send` (`PublishAwaitAcknowledgement`, logs each job the broker
accepted or the error it returned; `--topic` sets the topic, `--queue`
sends to a queue by name instead). `p2p print`
(client acknowledgement after printing; reports a `GAP` when job numbers
skip, which is how refused jobs show).

**Play.** apply, build, printer, sender. The printer starts first; the queue
would keep the first job anyway.

**Cards.**

1. *The printer goes offline.* The printer is stopped (SIGTERM, through
   `signal_consumer.sh ... active TERM`). The sender carries on; the queue
   fills, then the broker refuses jobs with `Spool Over Quota`. Reset starts
   the printer: every stored job in order, then a `GAP` for the refused
   ones.
2. *The sender gets the topic wrong.* The sender restarts with
   `p2p/lable/requested`. Every job is refused with `No Subscription Match`,
   because of the client profile setting.

**Assumption stated, not built.** Delivery is at least once: a job printed
but not acknowledged before a crash is delivered again. The guide's How
Solace does this says so; no card forces it.
