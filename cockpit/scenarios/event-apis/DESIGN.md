# Event APIs (720): design

Category 7, Governance Patterns, second pattern. Prefix `ea`. From
`docs/designs/wave-b-governance.md`, cut to two cards as agreed: the "new
version alongside the old" card is now the scenario's normal state, since it
is the right way rather than a failure.

**One idea.** An event API is a published, versioned contract. Change it by
adding a version beside the old one; retire a version only when nobody uses
it.

**Real and simulated.** Real: versioned topics, the consumers' failure and
silence, and the broker's live subscriptions as the record of who uses v1.
Simulated: the API document, `api.json` (AsyncAPI 2.6, JSON).

**Apps.** `ea shipping` (`--versions`, `--break`), `ea consumer` (one role,
`--version v1|v2`; reports idleness every 5 s after 8 s without a message).

**Cards.** v1 changed in place (labels cannot print; tracking fine); v1
retired too early (labels silent; q.ea.labels still subscribed to v1).

The flows into labels and tracking use `traffic:`, so the line to labels
stops when v1 stops reaching its queue.
