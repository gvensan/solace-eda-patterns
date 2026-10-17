# Hierarchical Topics

A retail chain publishes every sale on a topic that says where it happened
and what was sold, one fact per level:

```
ht/retail/<region>/<store>/<department>/sale/v1/<sale>
```

That is a hierarchical topic: **the topic is a path of levels, each level
carries a fact about the event, and subscribers pick the slice they want
with wildcards.** `*` matches exactly one level; `>` at the end matches
whatever follows. The broker does the routing, and nobody filters after
the fact.

Press **Play**. It creates the usernames, starts the **Electronics**, **EU**
and **Audit** teams, then the **Stores**, which publish four sales a second
from twelve stores in three regions and four departments.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Publish-Subscribe** is the same delivery. Here the point is the topic's
  levels, and the subscriptions that read them.
- **Event Filtering** (later) is for what topics cannot say: filtering on
  the message's content.

## Run it

Open the three teams' logs. Each prints the topic of every sale it got:

- **Electronics** subscribed to `ht/retail/*/*/electronics/sale/v1/>`: one
  department, in every region and store. Its log shows only electronics.
- **EU** subscribed to `ht/retail/eu/>`: one region, every store and
  department.
- **Audit** subscribed to `ht/retail/>`: everything.

The stores published every sale the same way. The slicing is entirely in
the subscriptions.

## On the broker

- **Connected clients**: `dataTxMsgCount` is what the broker sent each
  team. Audit's matches the stores' `dataRxMsgCount`; EU's is about a third
  of it, electronics' about a quarter.
- **What the electronics team subscribed to** and **What the EU team
  subscribed to**: the subscriptions as the broker holds them.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **A subscription miscounts the levels.** A new EU sales team subscribes
   to `ht/retail/eu/*/sale/v1/>`, forgetting the department level. It
   connects and subscribes without any error, and gets nothing: "nothing
   has matched ... in the last 5 s", while the EU team next to it receives
   every EU sale. `*` matches exactly one level, and after the store comes
   a department, not `sale`. **Reset** stops the team.
2. **The stores add a level.** They restart with a `till` level after the
   store. Electronics, whose subscription counts the levels with `*`, goes
   quiet. EU and audit, whose `>` takes whatever follows, carry on; audit's
   log now shows topics with `till` in them. **Reset** restarts the stores
   without the level.

## Try this

- Start a team of your own: edit the `eu-sales` action's `--topic` in
  `cockpit/scenarios/hierarchical-topics/scenario.yaml` to
  `ht/retail/*/store-007/>` and start it from Step through it. One store,
  wherever it is: `*` for the region, the store by name, `>` for the rest.
- Change it to `ht/retail/*/*/*/sale/v1/>` and start it again: every sale,
  spelled out level by level. It matches the same as `ht/retail/>` today,
  and would stop matching after the second card's change.

## How Solace does this

- A topic is a string of levels separated by `/`, up to 250 bytes. A
  subscription is a topic with optional wildcards: `*` matches exactly one
  level, and `>` as the last level matches one or more levels that follow.
  The broker matches every published message against every subscription.
- A well-formed subscription that matches nothing is accepted without
  comment. The broker does not know what topics a publisher intends to
  use, so it cannot warn.
- Topics are the first filter in Solace, and the cheapest: the broker
  routes on them without reading the message. Put the facts subscribers
  will select on into the levels, in a fixed order, and version the
  structure.

Next: [Event Filtering](320-event-filtering.md).
