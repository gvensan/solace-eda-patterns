# Event Bridge

A retailer's head office runs its own messaging, and so do its stores:
separate users, separate topics, separate queues, often separate teams. But
the stores need head office's price changes.

That is an event bridge: **a link that carries a chosen set of events from
one messaging domain into another.** Apps on each side publish and consume
in their own domain, and never connect to the other.

On Solace a domain is a message VPN, and a bridge connects two VPNs, on the
same broker or on two. This scenario uses two VPNs on one broker: head
office is `br-hq`, the stores are the workshop's own VPN. A bridge to
another broker is set up the same way, with that broker's address instead
of this one's.

Press **Play**. It creates head office's VPN with its users and a queue,
the bridge, the store's queue and username, then starts the **Store Till**
and **Head Office Pricing**.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event Mesh**: many brokers that route events between themselves
  dynamically, wherever there is a subscriber. A bridge is one configured
  link, and carries only what it is told to.
- **Event Gateway**: brings other protocols (MQTT, REST) into the broker. A
  bridge joins two Solace domains that both speak the same protocol.

## Run it

Open the **Head Office Pricing** log: "price #12: SKU-03 now 7.99,
published". It is connected to VPN br-hq. Open the **Store Till** log:
"price #12: SKU-03 now 7.99 (sent by head office 0.0 s ago)". It is
connected to the stores' VPN, and has never heard of br-hq.

In between: q.br.to-stores in br-hq collects the prices; the bridge
br-hq-link logs in to br-hq, consumes that queue, and publishes each price
in the stores' VPN on the same topic, where q.br.store takes it.

## On the broker

- **The bridge**: `inboundState` is `ready-in-sync` while it works, and
  `inboundFailureReason` says why when it does not.
- **Head office's queue for the stores**: near empty while the bridge keeps
  up.
- **What crosses the bridge**: q.br.to-stores subscribes to
  `br/hq/price/>`. That subscription is the bridge's whole list.

In Solace Broker Manager, switch to VPN br-hq to see head office's side.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The bridge goes down.** It is disabled. Head office keeps publishing
   and is told nothing; the store receives nothing, and the line into it on
   the diagram stops. The prices wait on
   q.br.to-stores. **Reset** enables the bridge, and they all arrive, in
   order, "sent by head office 12.0 s ago".
2. **Head office publishes something the bridge does not carry.** Head office
   starts publishing promotions. The store's queue subscribes to everything
   from head office, but nothing in br-hq asks for promotions, so they never
   cross, and no error appears anywhere. **Reset** stops them.

## Try this

- Make promotions cross: in `cockpit/scenarios/event-bridge/tf/hq.tf`, give
  q.br.to-stores a second subscription, `br/hq/promo/>`, press **Play**, and
  start **Head office promotions** from Step through it.
- Stop the **Store Till** instead of the bridge. Now prices cross and wait on
  q.br.store in the stores' VPN. Either queue can hold the backlog; which one
  does depends on which side is down.

## How Solace does this

- A message VPN bridge is configured in one VPN with the other's address,
  VPN name and a username there. The far side's client profile must allow
  bridge connections (`docs/spikes.md`, section 1).
- For guaranteed messages the bridge binds a queue on the far side and
  republishes what it takes on the near side. Its subscriptions are the
  bridge's list. For direct messages a bridge has remote subscriptions
  instead.
- A bridge uses a client connection on the far side, and a broker has a
  limit on bridges. Many links between many brokers are what an event mesh
  (dynamic message routing) is for.

Next: [Event Gateway](530-event-gateway.md).
