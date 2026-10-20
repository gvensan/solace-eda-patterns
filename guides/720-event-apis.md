# Event APIs

A shipping service emits an event every time a parcel leaves the depot.
Other teams build on it: one prints labels, one updates tracking. The
shipping team wants to change the event. How does it do that without
breaking teams it may not even know about?

That is an event API: **a published contract for the events an application
offers, with topics, payloads and a version.** Consumers build against the
contract. The producer changes it by adding a version beside the old one,
and retires the old one only when nobody uses it.

Press **Play**. It creates two queues and the usernames, starts **Labels**
and **Tracking**, then the **Shipping Service**, which publishes every
parcel on version 1 and version 2.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event Catalog**: every event across teams, as an inventory. An event
  API is one application's contract, offered to others.
- **Hierarchical Topics**: the version is a topic level (`.../v1/...`,
  `.../v2/...`), so each consumer subscribes to the version it understands.

## Run it

Open `cockpit/scenarios/event-apis/api.json`: an AsyncAPI document with two
channels, `ea/shipping/parcel/dispatched/v1/{parcel}` with `weightKg`, and
`.../v2/{parcel}` with `weight: {value, unit}`.

Open the **Shipping Service** log: "P-12345 dispatched, published on v1,v2".
**Labels**, written against v1: "P-12345: label printed, 2.4 kg". **Tracking**,
written against v2: "P-12345: tracking updated, 2400 g". Two versions of one
event, published side by side while teams move across: this is how a
contract changes safely.

## On the broker

- **Ask the broker who still uses v1**, in Step through it: the check to make
  before retiring a version. It lists every queue whose subscriptions match
  the v1 topic, read over SEMP, and says whether retiring v1 is safe.
- **What subscribes to v1**: q.ea.labels, subscribed to the v1 topic. This is
  the broker's own record of who still uses v1.
- **Queues**: both near empty while everyone keeps up.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **Version 1 is changed in place.** The shipping team sends v1 events in
   the new shape, under the v1 topic. Labels cannot print any label;
   tracking carries on. **Reset** goes back.
2. **Version 1 is retired while labels still uses it.** First the shipping
   team asks the broker who still consumes v1: the **Ask the broker who still
   uses v1** step reads every queue's live subscriptions over SEMP and
   answers "q.ea.labels subscribes to ea/shipping/parcel/dispatched/v1/>".
   The team retires v1 anyway. Labels gets nothing and no error, just
   "nothing received on API v1 for 10 s", and the line to it on the diagram
   stops. **Reset** brings v1 back.

## Try this

- Move labels to v2: change its action's `--version v1` to `--version v2`,
  add `ea/shipping/parcel/dispatched/v2/>` to q.ea.labels in
  `tf/queues.tf` and remove the v1 subscription, press **Play**, and run
  **Ask the broker who still uses v1**: "nobody subscribes to v1: safe to
  retire". Now the second card breaks nobody.
- Compare `api.json` with `catalog.json` in Event Catalog: the same format,
  used for one application's contract instead of an inventory.

## What Event Portal adds

Event Portal, a cloud service, models event APIs and event API products,
gives each version a semantic version number and a lifecycle (released,
deprecated, retired), and generates AsyncAPI documents to share with
developers. This scenario shows the contract and the versions with a file
and topics; Event Portal is where teams manage them.

Next: [Intermediated Governance](730-intermediated-governance.md).
