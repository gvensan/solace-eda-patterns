# Intermediated Governance

A sales team publishes orders and payments, with card details in the
payments. An inventory team wants the orders. Who decides whether it may
have them, and what exactly it gets?

In intermediated governance, **a central team decides: teams request
access, the central team reviews and grants it, and the platform enforces
each grant.** Nothing flows between teams without a decision.

Press **Play**. It creates both teams' access control (deny by default),
revokes any old grants, has the **Central Team** approve what is in the
requests file, then starts the **Inventory Team**'s app and the **Sales
Team**'s.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Disintermediated Governance**: teams act inside guardrails without
  asking anyone. Here every cross-team subscription is asked for and
  granted.
- **Access Control and Authorization**: how the broker enforces who may
  publish and subscribe. This scenario is about who decides those rules,
  and how.

## Run it

Open `cockpit/scenarios/intermediated-governance/requests.json`: the
inventory team asks for `ig/sales/order/>`, orders only, and says why. Open
the **Approve the requests** step's log: "approved: team inventory may now
subscribe to ig/sales/order/>". The central team's approval is
`central.sh`, standing in for the pipeline a real central team would run.
Open the **Inventory Team** log: "received on ig/sales/order/created/v1/...".
Payments are published too; inventory never asked for them.

## On the broker

- **What the inventory team was granted**: the one topic approved, as a
  subscribe exception on its ACL profile.
- **Each team's defaults**: both profiles disallow publishing and
  subscribing unless an exception allows it.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **A team subscribes before approval.** Every grant is revoked and the app
   restarts. The broker refuses its subscription, "Subscription ACL Denied",
   and the app exits. **Reset** approves the request and starts it again.
2. **A request is approved too broadly.** The central team grants
   `ig/sales/>`, and the inventory team ships a release subscribing to all
   of sales. Payments, card numbers and all, now reach inventory, and
   nothing refused them. **Reset** grants exactly what was asked.

## Try this

- Run **Revoke every grant**, then **Approve the requests** (narrow), then
  start **Start the inventory app, new release** from Step through it. The
  same wide release is now refused. That is what the narrow grant was for.
- Add a second request to `requests.json` (say, `ig/sales/payment/>` for a
  finance team) and look at what **Approve the requests** does.

## What Event Portal adds

Event Portal, a cloud service, can require applications to request
approval to publish or consume specific events, which replaces the
requests file and the script here. The broker side, ACL profiles enforced on
every subscription, is the same with or without it.

Next: [Disintermediated Governance](740-disintermediated-governance.md).
