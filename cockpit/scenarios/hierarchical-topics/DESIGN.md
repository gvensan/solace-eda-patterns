# Hierarchical Topics (310): design

Category 3, Consumption Patterns, first pattern. Prefix `ht`. Direct
messaging throughout.

**One idea.** A topic is a path of levels, and each level carries a fact
about the event. Subscribers pick the slice they want with wildcards: `*`
matches exactly one level, `>` matches the rest. The broker does the
routing; nobody filters after the fact. The levels are a contract between
publishers and subscribers.

**Story.** A retail chain. Every sale is published to
`ht/retail/<region>/<store>/<department>/sale/v1/<sale>`, four a second
across three regions, twelve stores and four departments. Teams subscribe:
electronics (`ht/retail/*/*/electronics/sale/v1/>`, one department across
every store), eu (`ht/retail/eu/>`, one region, everything in it), audit
(`ht/retail/>`, everything).

**Broker objects.** No queues. `cp-ht`, `acl-ht`, usernames `svc-ht-stores`,
`svc-ht-teams`.

**Apps.** `ht stores` (one app; `--extra-level` adds a level after the store).
`ht team --role <name> --topic <sub>`, which prints each matched topic and,
when nothing has matched for 5 seconds, says so.

**Play.** apply, build, electronics, eu, audit, stores.

**Cards.**

1. *A subscription miscounts the levels.* A new "eu-sales" team subscribes
   as `ht/retail/eu/*/sale/v1/>` (it forgot the department level) and gets
   nothing: `*` matches exactly one level, and the sale topics have a
   department between the store and `sale`. Reset stops it. `expect:` break:
   eu-sales `has: ["nothing has matched"]`, `lacks: ["got ht/"]`.
2. *The stores add a level.* The stores restart with a new till system that
   puts a `till` level after the store. Electronics, whose subscription
   counts levels with `*`, stops getting anything ("nothing has matched");
   eu and audit, whose `>` takes whatever follows, carry on. Reset restarts
   the stores without the level. `expect:` break: electronics `has:
   ["nothing has matched"]`; audit `has: ["got ht/retail/.*/till"]`. Reset:
   electronics `has: ["got ht/"]`.

**Differs from neighbours.** Publish-Subscribe: the same delivery, without
the levels mattering. Event Filtering (later): selectors on content, where
topics cannot say it.

**Spike.** None: wildcard matching is documented (Wildcard Characters in
Topic Subscriptions).
