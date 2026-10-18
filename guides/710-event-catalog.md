# Event Catalog

A shop's orders team publishes order events; the payments team publishes
payment events. Finance, shipping and analytics teams want to build on them.
How do they find out which events exist, on which topics, with which fields?

That is an event catalog: **an inventory of the events in a system, what
each one is, where it is published and what it carries, so teams can find
and reuse them.** It is only useful while it matches what actually runs.

Press **Play**. It creates a queue and the usernames, starts **Finance**
and the **Catalog Checker**, then the **Payments** and **Orders** teams'
services.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event APIs**: one application's published contract, with versions. The
  catalog is the inventory of every event, across teams.
- **Hierarchical Topics**: the topic naming that makes a catalog readable.

## Run it

Open `cockpit/scenarios/event-catalog/catalog.json`. It is an AsyncAPI
document, the open format Solace Event Portal exports: three events, each
with its topic (`ec/shop/order/created/v1/{order}`) and the fields its
payload must have.

Open the **Catalog Checker** log: "catalog \"Shop events\" 1.0.0: 3 event
types documented", then "in the catalog: ec/shop/order/created/v1/O-12345
(Order created)", and every 10 seconds "last 10 s: 3 event types seen, all in
the catalog". It subscribes to `ec/>` and compares every event with the
document. Open **Finance**: "O-12345: booked 42.00". It was written from the
catalog, and reads the amount field the catalog promises.

## On the broker

- **What is subscribed to the shop's events**: the checker and finance, each
  receiving; the two teams, each sending.
- **Queue**: q.ec.finance, near empty while finance keeps up.

The broker has no catalog. It carries whatever is published to whatever
subscribes, documented or not.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **A team ships an event nobody catalogued.** The orders team's new
   release also publishes order cancelled events. The checker reports
   "undocumented"; nothing else notices. **Reset** goes back to the release
   the catalog describes.
2. **A payload changes shape.** The payments team renames amount to total.
   Finance cannot book any payment, and the checker reports the missing
   field. **Reset** goes back.

## Try this

- Add the order cancelled event to `catalog.json` (copy the shipped channel
  and change its topic and fields), restart the checker, and use the first
  card again. Now it is documented.
- Stop the **Catalog Checker** and use the first card. Nothing tells you at
  all. The checker is the only thing that knew.

## What Event Portal adds

Solace Event Portal is a cloud service, not part of the broker. Its overview
describes a catalog of events, schemas and applications, runtime discovery
of what is actually in use, versions of events and schemas, and AsyncAPI
documents to download for developers. This scenario shows the idea with a
file and a checker; Event Portal is the tool.

Next: [Event APIs](720-event-apis.md).
