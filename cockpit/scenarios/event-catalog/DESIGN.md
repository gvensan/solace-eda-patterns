# Event Catalog (710): design

Category 7, Governance Patterns, first pattern. Prefix `ec`. From
`docs/designs/wave-b-governance.md`.

**One idea.** Events are reusable only if people can find them, and a
catalog is only useful while it matches what runs: something has to compare
it with the traffic.

**Real and simulated.** Real: the events, topics, queue, and the consumer's
failure. Simulated: the catalog, a hand-written AsyncAPI 2.6 document in JSON
(`catalog.json`; AsyncAPI is the format Event Portal exports, per its
overview page), and a checker app standing in for Event Portal's runtime
discovery. JSON, not YAML, so the Go apps need no new library.

**Apps.** `ec shop --team orders|payments` (`--cancellations` adds an
undocumented event; `--rename` renames amount to total), `ec checker` (direct
subscriber to `ec/>`, reports each finding at most once a minute, and a
10-second summary), `ec finance` (reads `amount` from q.ec.finance).

**Cards.** An undocumented event (checker flags it, nothing else fails); a
payload change (finance cannot book, checker flags missing amount).

**Without Event Portal.** No discovery, designer, versions or shared
catalog; the guide says what Event Portal adds, from its docs.
