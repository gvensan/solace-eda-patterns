# Intermediated Governance (730): design

Category 7, Governance Patterns, third pattern. Prefix `ig`. From
`docs/designs/wave-b-governance.md`.

**One idea.** A central team grants access, and the broker enforces each
grant. The grant is the control: it must be as narrow as the request.

**Real and simulated.** Real: deny-by-default ACL profiles, the
subscription refusal, the grant added over SEMP and its effect on the next
subscription. Simulated: the request (`requests.json`) and the review
(`central.sh approve`), standing in for Event Portal's access requests or a
ticket and pipeline. Event Portal's overview confirms access approval
("requiring applications to request approval to publish or consume").

**Broker objects.** `acl-ig-sales` (publish `ig/sales/>`), `acl-ig-inventory`
(nothing until approved; grants are added by `central.sh`, outside
terraform, and Play revokes then re-approves them), `cp-ig`, usernames
`svc-ig-sales`, `svc-ig-inventory`.

**Apps.** `ig sales` (direct orders and payments), the inventory team's app
is Access Control's `ac app` with its own username and subscription.

**Cards.** Subscribe before approval (Subscription ACL Denied); approved too
broadly plus a too-wide release (receives payments).
