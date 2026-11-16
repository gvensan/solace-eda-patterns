# State view: show what an app holds

Status: built 2026-11-06, first used by Event-Carried State Transfer.
Date: 2026-11-06

As built, against the proposal below: the tables sit in a strip under the
diagram ("What each app holds"), not in the node's Detail tab, so they stay
in view during Break it. Whether state survives a restart is declared in
the yaml (`holds: { kept: memory | disk }`), not reported by the app, and a
stopped app shows no rows either way: what was on disk may have been
deleted since (Cleanup deletes the customer database). `@state` lines are
hidden from the log, and the count is drawn in the node's bottom row. The
comparison is built as one summary line per copy (`copyOf:`), "Matches
Customer Service: 8 of 8" or how many are missing or different, computed
by the dashboard backend so smoke checks read the same numbers. Per-row
"behind" marks are not built: the copy trails the owner by milliseconds,
below the page's 3 second read. Added after a first look: a lead sentence
over the cards (`holdsCaption:`), a role line per card (`role:`), a "Last
change" line that names what changed, and `counters: true` so counters
are never marked as changed.

## The problem

The diagram shows where events go. It does not show what an app keeps
because of them, and in several patterns that is the point. In
Event-Carried State Transfer the lesson is shipping's copy of the
customers: it fills on Play, empties in card 2, refills on Reset, and sits
a moment behind the customer service's database. Today all of that is only
in log lines spread over three panes.

Other scenarios with the same blind spot (to confirm one by one before
adopting): CQRS (the read model), Event Sourcing (the state rebuilt from
events), Change Data Capture (the search index), Data Synchronization (the
two customer tables), Saga (each order's step).

## How it works

**App side.** A shared helper in `cockpit/apps/internal/solace`:

```go
solace.State(role, solace.Table{
    Title:   "Shipping's copy of the customers",
    Columns: []string{"customer", "city", "version"},
    Rows:    rows,
    Durable: false, // in memory: gone when the process stops
})
```

It prints one line, `@state {json}`, to stdout. It is called when the
state changes, at most twice a second (a trailing call always lands, so
the last state is never lost to the throttle).

**Dashboard side.** `Run._emit` keeps the latest `@state` line as the
run's snapshot and does not add it to the log, so logs stay readable.
`/api/scenarios/<id>/state` adds it to the node it belongs to, with
`running`.

**Page.** The node's Detail tab gets a "What it holds" table. Optionally
the node shows a count as its sublabel ("8 customers", "0 customers"),
through `indicator(node, "state-count", text)` with hover text and a row in
`guides/000-reading-the-dashboard.md`, as every indicator needs.

## Never showing what is not there

- An app that stops takes in-memory state with it. When the process is not
  running and `Durable` is false, the table is replaced by "Held in
  memory: gone when shipping stopped". No stale rows.
- Durable state (the customer database) is shown greyed, "on disk, the app
  is not running", because it really is still there.
- A row that changed in the last update is highlighted briefly, so a
  republish or a new version is visible as it happens.

## Comparing two holders (optional, second step)

A node can name another node to compare with, `compare: customers` in the
yaml, keyed on the first column. Rows whose version is behind the other
holder's are marked "behind". That makes the stated cost of the pattern,
"the copy is a little behind the owner", something an attendee can watch,
and makes card 2's empty copy obvious at a glance.

## Scope and checks

- Shared code: the Go helper, about 40 lines; `processes.py` and `main.py`,
  about 30; `cockpit.js` and `cockpit.css`, about 100.
- `check_scenarios.py`: a node with `state:` in the yaml must have its
  indicator documented; the yaml schema in CONTRIBUTING.md gains `state:`.
- `smoke.py`: an `expect:` block can check a snapshot (`state: {node:
  shipping, rows: 0}`), so card 2 can assert the copy really emptied.
- First adopter: ECST (customer database, shipping's copy, and a small
  "lookups asked / answered" table on invoicing). Others follow one at a
  time, each with its own check, smoke and browser pass.

## Decisions for review

1. Hide `@state` lines from the log (proposed), or show them too.
2. Count on the node as its sublabel, or only in the Detail tab. The
   sublabel is more visible, but some nodes already use it for role words
   ("active consumer"); in that case the count would go on a second line.
3. Whether to do the comparison step now or after the first adopters.
