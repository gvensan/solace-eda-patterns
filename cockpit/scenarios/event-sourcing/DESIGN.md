# Event Sourcing (140): design

Category 1, Event Generation Patterns, fourth pattern. Prefix `esrc`.

**One idea.** The events are the record; state is derived from them and can
be derived again. On Solace, a replay log keeps a copy of published
messages and a consumer can ask for them again. The replay log is bounded:
it is a recovery buffer, not a store of record, and the scenario shows that.

**Story.** A bank's ledger keeps no balances table. Each deposit or
withdrawal is published to `esrc/bank/account/<acct>/<type>/v1` with a
sequence number and the balance it leaves. The ledger applies its own
events when they come back on `q.esrc.ledger`. At start it binds with
`ReplayStrategyAllMessages` and rebuilds before transacting, checking each
event's recorded balance against the sum so far.

**Broker objects.** Replay log `rl-esrc` (1 MB, the smallest accepted, topic
filter `esrc/>`): one per VPN, so this scenario owns the VPN's. Exclusive
queue `q.esrc.ledger` (replay is refused on non-exclusive queues). `cp-esrc`,
`acl-esrc`, username `svc-esrc-ledger`. Clear broker config does not delete
replay logs (it deletes queues, usernames and profiles only); Cleanup's
terraform destroy does.

**Apps.** `esrc ledger`, `esrc statements` (45 x 30 KB on
`esrc/bank/statement/v1/<acct>`, logged too). `trim.sh` empties the log with
the SEMP action `trimLoggedMsgs`.

**Play.** apply, build, ledger.

**Cards.**

1. *The ledger crashes.* Killed. Reset: replay, "rebuilt from N events", same
   balances, every recorded balance matched.
2. *The replay log rolls over.* Ledger stopped, statements published (1.35 MB
   into 1 MB), ledger started: "rebuilt from 0 events". Reset empties the log
   and the bank starts over; the lost history cannot come back.

Spike: `docs/spikes.md`, section 17.
