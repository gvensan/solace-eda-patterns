# Publish-Subscribe

An online shop publishes an event each time an order is placed. Shipping,
analytics and email each need those events, for different reasons.

That is publish-subscribe: **a publisher sends an event to a topic once,
and every subscriber to that topic gets its own copy.** The shop does not
know who is listening, and a new team can start listening without the shop
changing anything.

These are **direct** messages: delivered to whoever is subscribed at that
moment, and kept for no one. That is the fast, cheap kind of messaging, and
the first card shows its cost.

Press **Play**. It creates the usernames, starts **Shipping**, **Analytics**
and **Email**, then the **Shop**, which places two orders a second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Point-to-Point**: one receiver per message, and a queue that keeps the
  message until the receiver takes it. Here every subscriber gets a copy,
  and nothing is kept.
- **Hierarchical Topics**: the same delivery, with the topic's levels as
  the contract between publisher and subscribers.

## Run it

Open the three subscribers' logs. Each order the shop publishes appears in
shipping's log and email's log: "event #12: order O-12, eu, total 84.10".
Analytics shows only the EU ones. The shop's log names the topic of every
tenth event: `ps/shop/order/placed/v1/eu/O-12`. The region is a topic level,
and analytics subscribed to `ps/shop/order/placed/v1/eu/>`; the shop
published the same way for everyone.

## On the broker

- **Connected clients**: `dataTxMsgCount` is what the broker sent each app.
  Shipping and email match the shop's `dataRxMsgCount`; analytics has about
  a third of it.
- **What shipping subscribed to** and **What analytics subscribed to**: the
  subscriptions the broker holds for each client. A subscription lives with
  the connection: when the app goes, so does the subscription.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **Shipping goes away for a while.** Shipping is killed. Email, shown on
   the card, carries on receiving every order. Press **Reset**: shipping
   comes back and says "missed #41 to #52, 12 events published while it was
   away". Those orders were never shipped, and the broker never knew
   anyone was missing them.
2. **A second shipping is started to share the load.** It receives every
   order, and so does the first: compare the event numbers in the two logs.
   Every order is now shipped twice. Pub/sub fans out; it does not share.
   **Reset** stops the second copy.

## Try this

- Stop **Analytics** from its node, then start it again. It reports nothing
  missed: its subscription is EU orders only, and it checks the numbers of
  the events it receives, so a gap can only show when the numbers it gets
  are consecutive. Shipping and email, which get every event, can count
  what they missed.
- Edit the analytics action in `cockpit/scenarios/publish-subscribe/scenario.yaml`
  to subscribe to `ps/shop/order/placed/v1/*/O-*0` and restart it: only
  orders whose number ends in 0, by topic alone. Put it back afterwards.

## How Solace does this

- A topic is a string with levels; a subscription is a topic with optional
  wildcards (`*` for one level, `>` for the rest). The broker matches each
  published message against every subscription and sends a copy to each
  match. The publisher never names a subscriber.
- Direct messages are not stored: no queue, no acknowledgement, no
  redelivery. A subscriber away at the moment of publishing misses the
  message. For an event that must not be lost, a queue subscribes to the
  topic and keeps a copy (Point-to-Point, Guaranteed Delivery).
- To share one topic's events among several copies of an app rather than
  fan out to each, Solace has shared subscriptions (`#share/<group>/<topic>`;
  Consumer Groups shows them), or a queue (Competing Consumers).

Next: [Point-to-Point](220-point-to-point.md).
