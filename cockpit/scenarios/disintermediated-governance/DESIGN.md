# Disintermediated Governance (740): design

Category 7, Governance Patterns, fourth pattern. Prefix `dg`. From
`docs/designs/wave-b-governance.md`, with both cards enforced by the broker:
the brief's naming-check card (a script refusing a name) was dropped in
favour of the broker's own guardrails, prefix and quota.

**One idea.** Teams act without asking, inside guardrails the platform sets
once and the broker enforces for everyone.

**Real.** Prefix ACLs per team (`dg/teams/<team>/>`, plus every team's
`dg/teams/*/public/>` to subscribe), the refusal on a guaranteed publish
outside the prefix, client-created queues (self-service), and the client
profile's endpoint limit (one per username). **Simulated.** Nothing; there
is no request or review to stand in for.

**Apps.** `dg team` (one role: `--publish`, `--queue` created on start if
missing, `--subscribe` added to it). `delete-team-queues.sh` removes the
client-created queues at Cleanup, since terraform did not create them and
Clear broker config does not list them.

**Cards.** Publish outside the prefix (Publish ACL Denied); a second queue
over the quota (refused at create).

Spike: `docs/spikes.md`, section 20.
