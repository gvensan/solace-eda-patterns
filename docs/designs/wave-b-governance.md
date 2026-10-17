# Wave B governance briefs

Status: for review. No scenario code is written until these are approved.
Date: 2026-10-17

Four governance patterns from the pattern guide's category 7: Event
Catalog (710), Event APIs (720), Intermediated Governance (730) and
Disintermediated Governance (740). Access Control and Authorization (750)
is built.

## The constraint that shapes all four

The workshop runs a Standard software broker in a Codespace, with no
Solace Cloud account. **Solace Event Portal is a Solace Cloud service**: the
catalog, the designer, discovery of what runs on a broker, and the
AsyncAPI export all live there, not on the broker. A software broker has
no catalog, no schema registry and no notion of an application's
ownership. What a broker does have, and what is real here:

- topics, and what flows on them, observable by any subscriber;
- ACL profiles and client usernames, enforced on every publish and
  subscribe (shown in Access Control);
- SEMP, with which a central team, a pipeline or a self-service tool can
  change all of the above.

So every brief below separates **real** (the broker does it) from
**simulated** (the workshop stands in for Event Portal or a process). Each
also says what breaks without Event Portal. A brief whose pattern has no
honest hands-on form says so and proposes a guide-only page.

The briefs reuse one shared asset, an **AsyncAPI document** per brief in
the scenario folder (`asyncapi.yaml`), written by hand. AsyncAPI is the
open specification Event Portal exports; using it keeps the workshop
honest about the format without pretending a file is a portal.

---

## Event Catalog (710)

**One idea.** Events are only reusable if people can find them: what
exists, on which topics, with what payload, who produces and who
consumes. A catalog is that inventory, and it is only useful if it matches
what actually runs.

**What the attendee does on the broker.** The scenario runs a small
estate: an orders service and a payments service publishing three event
types on `ec/...` topics, and two consumers. A **catalog checker** app
subscribes to `ec/>` and compares every topic and payload it sees with the
scenario's `asyncapi.yaml`: each event is "in the catalog", or
"undocumented: ec/shop/order/cancelled/v1 is not in the catalog". The
attendee opens `asyncapi.yaml`, reads the event definitions, and runs the
consumers against the documented topics.

**Diagram.** Orders and payments publish to the broker; two consumers and
the catalog checker subscribe. The checker's node shows "3 of 3 event
types documented" from its own log.

**Cards.**

1. *A team ships an event nobody catalogued.* The orders service restarts
   with a new `cancelled` event. The checker logs "undocumented" for it;
   nothing breaks at the broker. The lesson: a catalog drifts unless
   something compares it with traffic. Reset restarts orders without it.
2. *A payload changes shape.* Payments renames a field. The checker reports
   "payload does not match the catalog: missing amount"; a consumer
   written against the catalog prints a parse error. Reset restores the
   field.

**Real.** The traffic, the topics, the subscriptions, the consumers'
failures. **Simulated.** The catalog itself (a hand-written AsyncAPI file
and a checker app stand in for Event Portal and its discovery agent).

**Without Event Portal.** No discovery, no shared designer, no versioned
catalog, no AsyncAPI generation; the "catalog" is a file in the repo that
only this workshop reads. The guide says plainly that Event Portal does
this as a product, and that the checker shows the idea, not the tool.

**Verdict.** Honest hands-on form exists (drift detection against real
traffic). Build it.

---

## Event APIs (720)

**One idea.** An event API is a published contract: the channels (topics)
an application offers or accepts, the message schemas, and a version.
Consumers build against the contract, not against the producer's code, and
the producer changes it only by publishing a new version.

**What the attendee does on the broker.** The scenario's `asyncapi.yaml`
describes a **shipping event API v1**: `ea/shipping/parcel/dispatched/v1/{parcel}`
with a schema. A producer implements v1; two consumers are built from the
document. The attendee publishes a **v2** of the API (a new topic version,
`.../v2/...`, with a changed schema) alongside v1, migrates one consumer,
and retires v1.

**Diagram.** The producer publishing on v1 and, after the first card, on
v1 and v2; consumers subscribed to one version each.

**Cards.**

1. *A breaking change without a new version.* The producer changes the v1
   payload in place. The consumer built from the v1 contract fails to
   parse it. Reset reverts.
2. *A new version alongside the old.* The producer publishes v2 and keeps
   v1; the old consumer is untouched, the migrated one reads v2. Not a
   failure: the card shows the right way, against card 1.
3. *Retiring v1 too early.* The producer stops v1 while one consumer still
   subscribes to it: that consumer goes silent, with no error (the same
   silent miss as Hierarchical Topics). The lesson: retirement needs to
   know who still consumes, which is the catalog's job.

**Real.** Topic versioning, subscriptions, the parse failure, the silent
miss. **Simulated.** The API document as a hand-written file; nobody
generates code from it here.

**Without Event Portal.** No API products, no consumer registration, so
"who still consumes v1" can only be answered from the broker's live
subscriptions (SEMP), which the card shows as the stopgap.

**Overlap.** Cards 1 and 3 are close to Hierarchical Topics and Event
Catalog. If the overlap reads as repetition, cut to card 2 plus a guide
section, or merge with Event Catalog. **Verdict.** Hands-on form exists;
decide on the overlap at review.

---

## Intermediated Governance (730)

**One idea.** A central team decides who may publish and subscribe to
what. Teams ask; the central team reviews and grants. Control and
consistency, at the cost of a queue of requests.

**What the attendee does on the broker.** Two application teams' apps run
with ACL profiles that deny by default (as in Access Control). The
attendee plays both roles: as the **inventory team**, it starts an app that
needs to subscribe to `ig/sales/order/>`, which is refused ("Subscription
ACL Denied"); it files a request by adding an entry to
`requests.yaml`; as the **central team**, it runs **Approve requests**, a
step that applies the requested ACL exception with terraform (the change a
pipeline would make), and the app's next subscribe succeeds.

**Diagram.** Two team apps and a "central team" node whose action is the
approval step; the broker holds one ACL profile per team.

**Cards.**

1. *A team subscribes before approval.* Refused at the broker, with the
   real error. Reset removes the app.
2. *A request is approved too broadly.* The central team grants
   `ig/sales/>` instead of `ig/sales/order/>`; the app now also receives
   payment events it should not see. The lesson: the central team's
   review is the control, and an over-broad grant is invisible to the
   requester.

**Real.** The ACL denial, the grant applied over SEMP by terraform, its
effect on the next subscribe. **Simulated.** The request queue (a YAML
file) and the review (pressing a button).

**Without Event Portal.** No request workflow, no application ownership:
the requests file stands in for Event Portal's access requests or a
ticketing system. The broker-side enforcement is the same either way.

**Verdict.** Honest hands-on form exists. Build it.

---

## Disintermediated Governance (740)

**One idea.** Teams govern themselves within guardrails: each team owns a
topic prefix and may publish and subscribe there without asking anyone;
the platform enforces only the guardrails (naming, prefixes, quotas).

**What the attendee does on the broker.** Each team's ACL profile allows
its own prefix (`dg/teams/<team>/>`) and subscribing to anything published
under any team's `public` level. A **self-service** step lets a team add a
topic or a queue under its own prefix with no approval: it runs the
platform's guardrail check (a naming rule) and applies it.

**Diagram.** Three team apps, each publishing under its own prefix, and a
"platform" node whose action is the self-service step.

**Cards.**

1. *A team publishes outside its prefix.* Refused at the broker (guaranteed
   publish, so the refusal comes back on the receipt; Access Control
   showed a direct publish would vanish). No one had to review anything
   for the allowed topics.
2. *A team names a topic against the convention.* The guardrail check
   refuses the self-service request before anything reaches the broker.
3. *A team's queue fills the spool.* Self-service lets a team create a
   queue; without a quota guardrail one team's backlog grows until it
   affects others. The client profile's per-username endpoint and spool
   limits are the guardrail that stops it (measured in a spike first).

**Real.** Prefix-based ACLs, the refusals, the queue and its limits.
**Simulated.** The self-service tool (a script with a naming check).

**Without Event Portal.** No shared catalog of what each team owns, so
discoverability across teams is lost; the guardrails themselves are broker
configuration and need no portal.

**Verdict.** Honest hands-on form exists. Build it.

---

## Summary

| Pattern | Hands-on? | Real on the broker | Simulated |
| --- | --- | --- | --- |
| Event Catalog (710) | yes | traffic, drift against the catalog | the catalog (an AsyncAPI file and a checker) |
| Event APIs (720) | yes, overlap to decide | versioned topics, parse failure, silent miss | the API document |
| Intermediated Governance (730) | yes | ACL denial and the approved grant | the request queue and the review |
| Disintermediated Governance (740) | yes | prefix ACLs, refusals, quotas | the self-service tool |

None needs a guide-only page. All four would be richer with Event Portal,
and each guide says what Event Portal adds; none depends on it.
