# Event Sourcing (140): design

Category 1, Event Generation Patterns, fourth pattern. Prefix `esrc`.

**One idea.** Every change is recorded as an immutable event, and the
current state is derived from the events: commands are validated against
that state, and only accepted changes become events. Lose the state and it
is derived again; lose the events and nothing is left to derive it from.

**Story.** A bank. The teller submits commands on
`esrc/bank/command/<acct>/<deposit|withdraw>/v1` into `q.esrc.commands`.
The ledger takes one at a time, validates it against balances it holds only
in memory, and records an accepted one as an event on
`esrc/bank/account/<acct>/<deposited|withdrawn>/v1` (seq, command id,
account, per-account version, amount, resulting balance). Once the broker
confirms the publish, the ledger applies the event in memory, acknowledges
the command and takes the next. It never consumes its own events live:
append, then apply, as in the standard pattern.

**Assumptions, stated rather than built.** One ledger at a time
(`q.esrc.commands` is exclusive). A complete, durable history: the replay
log stands in for an event store, which would know each stream's length and
treat a missing history as an error. Balances in memory, with no snapshots
or projections. An earlier design added a last-value queue `q.esrc.last` to
detect an emptied history, and had the ledger consume its events back from
a queue; both were removed (2026-11-06) because they covered a stand-in's
weakness or were a demo choice, not event sourcing.

**Rebuild, at start only.** The ledger adds the events subscription
`esrc/bank/account/>` to the durable exclusive queue `q.esrc.rebuild`
(replay is refused on temporary queues), binds it with
`ReplayStrategyAllMessages`, and applies each event. It then publishes a
marker on `esrc/bank/marker/<client>/<nanos>`, which the queue subscribes to
statically. The replay log keeps markers too: a message published during a
replay reaches the queue only from the log, so an unlogged marker never
arrived once the history was a few thousand events (spikes, section 23).
Logged, it arrives after every replayed event, so the rebuild ends on a
fact; markers from earlier starts are skipped. With nothing received for 30
s and no marker, the ledger stops ("rebuild stalled"). The ledger removes the events subscription again, so no live
event collects in the queue, and only then binds `q.esrc.commands`.

**Checks.** A command redelivered after a crash is recognised from the
command id in the rebuilt history and not recorded twice. Each replayed
event must follow the last (`seq`) and carry the balance the sum gives; the
balance is always the sum, and on either failure the ledger stops (exit 1)
and takes no command, since it can see the history is incomplete. Fixed
2026-11-16: it used to log the error and carry on with the carried balance,
which made a partly emptied history look fine. A
publish whose outcome is unknown stops the ledger (exit 1): the restart
rebuilds and the unacknowledged command comes again.

**Broker objects.** Replay log `rl-esrc` (1 MB, the smallest accepted, a
demo setting; topic filters `esrc/bank/account/>` and `esrc/bank/marker/>`): one per VPN, so this
scenario owns the VPN's. Exclusive queues `q.esrc.commands` and
`q.esrc.rebuild` (marker subscription only, between rebuilds).
`cp-esrc` (allows the ledger to add a subscription to its queue),
`acl-esrc`, usernames `svc-esrc-ledger`, `svc-esrc-teller`. Clear broker
config does not delete replay logs (it deletes queues, usernames and
profiles only); Cleanup's terraform destroy does.

**Apps.** `esrc ledger`, `esrc teller`. `trim.sh` empties the history
(SEMP `trimLoggedMsgs`).

**Diagram.** Teller, broker, ledger. Boxes: q.esrc.commands (on the
ledger's lane), rl-esrc, q.esrc.rebuild. Paths: a command becoming an event
and changing a balance (1 to 3), and the rebuild at start (4, the hop
"replayed at start", and 5, "read at start"; both drawn still, since they
carry messages only while the ledger starts).

**Play.** apply, build, ledger, teller.

**Cards.**

1. *The ledger crashes.* Killed; commands wait in q.esrc.commands. Reset:
   rebuild from the history, same balances, then the waiting commands.
2. *The event history is lost.* Breaks the stated assumption on purpose:
   ledger stopped, history emptied, ledger started. It logs "rebuilt from 0
   events: the history is empty" and carries on from 0.00; nothing warns.
   Reset restarts the ledger. The lesson is that the events must be durable.

The replay log is a stand-in for an event store, kept in the background:
the lesson is event sourcing, not replay. An earlier card filled the log
with large statements to show it drop its oldest events; it was removed
(2026-11-06) because it taught a replay log detail, not the pattern. An
earlier Try this rebuilt from a partly emptied history using each event's
carried balance as a snapshot; it was removed (2026-11-06) because it
taught that missing events do not matter. The carried balance is now a
check only.

Measured 2026-11-06: the replay log reports about 600 KB used after the
first events, then about 100 bytes per event, so 1 MB lasts well over an
hour at this rate.

Spikes: `docs/spikes.md`, sections 17 and 23.
