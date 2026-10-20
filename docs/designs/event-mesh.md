# Event Mesh (520): design brief

Status: for review. Design and spike only; nothing is built. Spike:
`docs/spikes.md`, section 21. Date: 2026-10-20.

## One idea

Several brokers joined into one event mesh route an event from the broker
it was published on to every broker that has a subscriber for it, and only
those. An app connects to its nearest broker and never knows where the
other side is. An event bridge (510) is one configured link carrying a
chosen set of events; a mesh learns what to route from the subscriptions
themselves.

## Does it fit a 16 GB Codespace?

**Two brokers: yes, on paper. Three: no.**

Measured, Standard 10.26.0 at the smallest settings DMR accepts (1,000
connections, 500,000 subscriptions, 100 queue messages, 25 bridges): 3.25
and 3.59 GiB idle, 6.8 GiB for the pair, unchanged with the link up and
events crossing. A 16 GB Codespace, minus the devcontainer, the dashboard,
the Go apps and a browser tab, leaves about 12 GiB, so two brokers fit with
roughly 5 GiB spare. Three brokers need about 10.1 GiB for the brokers
alone; on the 7.65 GiB Docker VM used for the spike a third broker killed
all three. For three in a Codespace the 32 GB machine type is needed.

The existing single workshop broker runs at the 100-connection scale (1.3
to 1.6 GiB) and cannot join a mesh (DMR is refused at 100 connections,
spike 3). So the mesh scenario needs **two more brokers of its own**, at
the 1,000-connection scale, or the workshop broker recreated at that scale
plus one more. Recreating the workshop broker costs every other scenario
about 2 GiB of memory for nothing, so the design starts two mesh brokers
on demand and stops them at Cleanup.

Not yet measured in a Codespace: two brokers beside the dashboard, the Go
build and Chrome. `docs/codespace-test.md` should gain a section for it
before this is built, and the decision between "two brokers on demand" and
"guide only" should wait for that number.

## What the attendee does on the broker

The story: a retailer with a store in Lyon and a warehouse in Leeds, each
with its own broker. The store's till publishes orders to the Lyon broker.
The warehouse's picking app subscribes on the Leeds broker. The mesh
carries each order from Lyon to Leeds because, and only because, Leeds has
a subscriber. A second store app on Lyon subscribing to pick confirmations
shows the other direction.

Traffic is a trickle: one order every two seconds, a few hundred bytes.
The pattern is about where events go, not how many.

Play, in order:

1. **Start the mesh brokers**: a process action running
   `mesh/start.sh`, which `docker run`s `mesh-lyon` and `mesh-leeds` at
   the spike's settings on their own Docker network and waits for SEMP on
   both (about 90 s cold, a few seconds warm). Host ports chosen not to
   clash with the workshop broker.
2. **Apply configuration**: terraform against *both* brokers (two provider
   aliases, `lyon` and `leeds`): the DMR cluster on each, the link (created
   disabled, address added, then enabled, which the provider handles as
   one resource with its remote address), `dmrEnabled` on each `default`
   VPN, a DMR bridge in each VPN to the other node, the picking queue on
   Leeds subscribed to `mesh/shop/order/>`, the confirmations queue on Lyon
   subscribed to `mesh/warehouse/pick/>`, usernames on each.
3. Build, then the picking app (Leeds), the store app (Lyon, subscribing to
   confirmations), the till (Lyon, publishing orders).

Apps pick their broker with `SOLACE_HOST`/`SOLACE_SMF_PORT` in the
action's `env`, as Event Bridge picks a VPN with `SOLACE_MSG_VPN`.

## The diagram

This is the first scenario with two brokers, which the checker forbids
("one broker per diagram"). The brief proposes a deliberate checker
change: a diagram may hold two `kind: broker` nodes when the scenario sets
`mesh: true`, and then every flow must still touch a broker, and a flow
between the two brokers is allowed and drawn as the mesh link.

```
Till (Lyon) --> [Lyon broker] ==mesh link==> [Leeds broker] --> Picking
Store app   <-- [Lyon broker] <==          <== [Leeds broker] <-- Picking
```

The link line carries the mesh's state from the Lyon broker's DMR link
monitor (`up`, `failureReason`), read by `/state` the way bridges and
flows are read now, and its packets use `traffic:` on the Leeds queue, so
the line moves only while orders actually arrive on Leeds.

## Cards

1. **The mesh link goes down.** The link is disabled over SEMP (a network
   cut between sites). Orders keep being published on Lyon and are
   accepted; nothing reaches Leeds, the link shows down on the diagram,
   and the picking app goes quiet. Reset enables the link, and the orders
   published meanwhile arrive: guaranteed messages wait on the link's
   queue on Lyon (`queue_max_msg_spool_usage` on the link). Real: all of
   it. Expect: the picking log silent then catching up, Lyon's link queue
   depth rising then draining.
2. **A subscriber appears on the far side.** With the mesh up, a second
   picking app starts on *Lyon* subscribing to the same orders. Orders
   now go to both brokers' subscribers, and a counter on each shows the
   mesh routing to Leeds did not change: the mesh adds a copy where a
   subscriber is, it does not move the stream. Not a failure, the
   "right way" card, like Event Bridge's; drop it if the review prefers
   two failures.
3. **The Leeds broker goes down.** `docker stop mesh-leeds`. The link
   fails, the till is unaffected, orders queue on Lyon's link queue.
   Reset starts Leeds; the link reconnects and the backlog crosses.
   Shows what a mesh does and does not protect: a publisher's own broker
   must be up, a remote one can be away.

Role-based wording is not needed: each app is a named app with one
instance.

## Real and simulated

Real: two brokers, DMR, the link, routing by subscription, the backlog
during a cut. Simulated: nothing, but the two "sites" are two containers
on one machine, which the guide says plainly. The network cut is a
disabled link, not a dropped network.

## What breaks without Event Portal

Nothing; the mesh is broker configuration. Event Portal would show the
mesh's events in one catalog across both brokers, which the guide can
mention.

## Fallback if two brokers do not fit

Guide-only page, 520, using the spike's numbers and SEMP calls, with the
diagram drawn static, and a pointer to Event Bridge (510) as the hands-on
neighbour: "a bridge is one link you configure; a mesh is many links the
brokers manage from the subscriptions." It would also carry the
`mesh/start.sh` script and the terraform so an attendee with a bigger
machine can run it by hand.

## Work needed before building

- Codespace measurement of two brokers beside the dashboard
  (`docs/codespace-test.md`).
- Checker and `diagram.js`: two brokers under `mesh: true`, and a link
  state read for the inter-broker line.
- Terraform: two provider aliases; the DMR link resource's "disabled while
  adding the address" sequence verified with the provider, which the spike
  did only over SEMP.
- `mesh/start.sh` and the Cleanup that stops and removes the two
  containers, plus Clear broker config leaving them alone (they are not
  in `imports.tsv` and not on the workshop broker).
