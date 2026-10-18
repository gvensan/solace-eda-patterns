# Disintermediated Governance

Twenty teams publish and consume events, and every new subscription waits
for a central team's review. Teams want to move faster. What if nobody had
to approve anything, and the platform only set the rules every team must
follow?

That is disintermediated governance: **teams govern themselves inside
guardrails. Each owns part of the event space and acts there without
asking; the platform enforces only the rules.**

Press **Play**. It creates the guardrails (no queues), then starts the
**Billing Team**'s app, which creates its own queue, and the **Orders
Team**'s.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Intermediated Governance**: a central team grants each request. Here
  there are no requests, only limits.
- **Access Control and Authorization**: the broker feature both governance
  styles rely on. Here the ACLs are written as prefixes per team, once,
  instead of per request.

## Run it

Open the **Billing Team** log: "queue q.dg.billing.orders is ours (created
by this team, no request needed), subscribed to dg/teams/orders/public/>",
then "received O-12345 from dg/teams/orders/public/order/created/v1/...",
and "published dg/teams/billing/public/invoice/created/v1/...". It created
its own queue, subscribed it to another team's public events, and publishes
in its own space, all without asking anyone.

## On the broker

- **Queues the teams created**: q.dg.billing.orders, with `owner`
  `svc-dg-billing`. Terraform created no queues.
- **The guardrails**: `cp-dg` lets teams create endpoints, and
  `maxEndpointCountPerClientUsername` is 1: one queue per team.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **A team publishes outside its prefix.** A new billing service publishes
   order events, in the orders team's space. Every publish is refused on its
   receipt, "Publish ACL Denied". **Reset** stops it.
2. **A team goes over its queue quota.** A second billing service tries to
   create a queue of its own; the team already has one. The broker refuses,
   "No More Non-Durable Queue or Topic Endpoint", and the service exits. **Reset** stops it.

## Try this

- Add a third team: add `"shipping"` to `dg_teams` in `tf/access.tf` and
  press **Play**. It gets its prefix and its quota, with nobody approving
  anything.
- Raise the quota to 2 (`max_endpoint_count_per_client_username`), press
  **Play**, and use the second card again.

## How Solace does this

- Each team's ACL profile allows publishing and subscribing under its own
  prefix and subscribing to every team's `public/` level. The broker checks
  every message and every subscription against it.
- A client profile can let clients create their own queues, and caps how
  many each client username may create.
- What the guardrails cannot do is tell teams what other teams publish:
  that needs a shared catalog (see Event Catalog).

Next: [Access Control and Authorization](750-access-control.md).
