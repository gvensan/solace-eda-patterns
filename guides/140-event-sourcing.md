# Event Sourcing

A bank's ledger could keep a table of balances and update it with every
deposit and withdrawal. Then the table is the truth, and how each balance
came about is lost. Or the ledger could record every change as an event,
and work out each balance by adding the events up. Then the history is the
truth, and the balances can always be worked out again.

That is event sourcing: **every change is recorded as an event, appended
and never edited, and the current state is derived from the events.** Lose
the state, and you derive it again from the events. Lose the events, and
there is nothing to derive it from.

Press **Play**. It creates the queues, the event history and the usernames,
then starts the **Ledger**, which rebuilds its balances from the history
(empty, the first time), and the **Teller**, which submits a command every
second.

To be walked through this page step by step, press **Guided tour** in the
top bar: it tells you what to do, waits while you do it, and explains what
you see. Its second tab, **Quick overview**, is a short spoken version.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **CQRS**: often used with event sourcing, to build read views
  (projections) from the events. Here there is one service, and its own
  state is what is rebuilt.
- **Event Streaming**: a consumer keeps its position in a stream and reads
  on from there. Here the consumer goes back to the first event on purpose,
  because the events are all it has.

## Run it

What to do, in the order the page shows it:

1. **Run it**: press **Play**, then read **Follow a message** under the
   diagram: steps 1 to 3 are a command becoming an event and the event
   changing a balance, steps 4 and 5 the ledger rebuilding when it starts.
   Click a path's name, or a numbered line, to highlight it. **Expand** at
   the top right fills the window with the diagram.
2. **What each app holds**, under that: the ledger's balances, and how many
   events they were worked out from.
3. **Break it**: open a tile, press **Break it**, watch, press **Reset**.
4. **On the broker**: the event history and the queues as the broker
   reports them.

Start with the two kinds of message. A **command** asks for a change and can
be refused. An **event** records a change that has been accepted:

| Message | Topic | Payload |
| --- | --- | --- |
| Command: pay in | `esrc/bank/command/A-2/deposit/v1` | `{"command":"103512-7","account":"A-2","action":"deposit","amount":40}` |
| Event: paid in | `esrc/bank/account/A-2/deposited/v1` | `{"seq":12,"command":"103512-7","account":"A-2","version":5,"type":"deposited","amount":40,"balance":140}` |
| Event: taken out | `esrc/bank/account/A-2/withdrawn/v1` | `{"seq":13,"command":"103512-8","account":"A-2","version":6,"type":"withdrawn","amount":15,"balance":125}` |

An event says what happened (`deposited 40`), in the past tense. `seq` is its
place in the whole history, `version` its place among that account's
events, `command` the command it records, and `balance` what the account
held after it, so whoever adds the events up can check the sum as they go.

Follow one command:

1. The **Teller** submits a command, a request that can be refused: "take
   40.00 out of A-2". It waits in q.esrc.commands.
2. The **Ledger** validates it against the current balance, which is what
   A-2's events so far add up to. Too large, and it is refused: nothing is
   recorded, nothing changes. (Event-sourced systems may also record a
   rejection; it just must not change a balance.)
3. Accepted, it is recorded as an event, "A-2 withdrawn 40.00", in the
   history (rl-esrc). **That is the only write: there is no separate
   balance record.** Once the broker confirms it has the event, the ledger applies
   it: A-2 is now its events added up, this one included. Then it takes the
   next command.
4. On restart, the Ledger rebuilds its balances by replaying the complete
   retained event history before processing new commands. The broker
   replays the history into q.esrc.rebuild, oldest first. That line only
   carries messages while the ledger starts.
5. The ledger adds every event up, from #1 to the last, and only then takes
   a command. Same events, same balances. If required history is missing,
   it cannot safely rebuild.

Compare an ordinary service: it would update a balance row in its database
and then publish an event to tell others. There the row is the truth and the
event a notification. Here the event is the record, and the balance only
ever its sum.

Open the **Ledger** log: "command 103512-7 accepted: recorded event #12,
A-2 deposited 40.00, balance 140.00". **What each app holds**, under the
diagram, shows the four balances in its memory and the event that last
changed each one. There is no separate balance record anywhere else: the
`balance` in each event is a check on the sum, never its source.

### Assumptions

The demo keeps to the pattern itself, and states what it takes for granted
rather than building it:

- **One ledger at a time.** q.esrc.commands is an exclusive queue, so a
  second ledger would wait as a standby. With several writers, each append
  would have to check that the account's `version` had not moved
  (optimistic concurrency).
- **The event history is complete and durable.** The replay log stands in
  for an event store. A real event store keeps every event, replicated, and
  knows how many each stream has, so a history that went missing is an error
  there. A replay log is bounded and can be emptied; the second Break it
  card shows what happens when this assumption fails.
- **Balances live in memory.** Rebuilding from event #1 is fine for a few
  hundred events. Real systems add snapshots, so a rebuild starts from a
  known state, and projections (see [CQRS](120-cqrs.md)) for read views.

### What the ledger checks

- **No command recorded twice.** Each event names its command. If the ledger
  crashes after recording an event but before acknowledging its command, the
  broker sends the command again; the rebuilt history says it is done, and
  the ledger logs "not recorded twice". The API's own retransmission of a
  publish does not cover this: it is a business-level check.
- **The events add up.** While rebuilding, each event must follow the one
  before it (`seq`), and the `balance` it carries must match the sum so far.
  The balance is always the sum: the carried one is a check, never a
  stand-in. If either check fails, the ledger stops with the reason
  ("the history starts at event #29, not #1") and takes no command, because
  balances added up from a history with a hole in it are wrong.
- **No guessing on a write.** If the broker does not confirm an event in
  time, the ledger stops rather than guess whether it was recorded; a
  restart rebuilds and finds out.

## On the broker

- **Replay log**: `rl-esrc` retains the events, and is the event history
  here. `msgSpoolUsage` grows with every event. Its 1 MB is a demo setting
  (the smallest the broker accepts), not a promise to keep everything: as it
  nears full, the broker removes the oldest events to make room. A message VPN can have only one.
- **What the log keeps**: the topic filters `esrc/bank/account/>`, the
  events, and `esrc/bank/marker/>`, the ledger's end-of-replay markers (see
  How Solace does this). Without them the log would keep every guaranteed
  message published in the VPN, every other scenario's included.
- **Queues**: q.esrc.commands holds the waiting commands. q.esrc.rebuild is
  used only while the ledger starts: the broker replays into an exclusive
  queue only, and only the events that match its subscriptions, so the ledger
  adds the events subscription, takes the replay, and removes it again.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The ledger crashes.** It is killed, and its balances, which were only
   in its memory, with it. The teller carries on, and q.esrc.commands
   climbs. **Reset** starts it: it rebuilds, "rebuilt from 57 events: A-1
   120.00, ...", the same balances come back, and only then does it work
   through the waiting commands.
2. **The event history is lost.** This breaks the demo's assumption on
   purpose. With the ledger stopped, every event is removed from rl-esrc.
   The ledger starts, finds nothing, logs "rebuilt from 0 events: the
   history is empty, every account starts at 0.00", and carries on as if
   the bank had just opened. Nothing warns anyone: every customer's money is
   gone. That is why the events must live in a durable event store, backed
   up like any database.

## Try this

- Crash the ledger from Step through it a few times, and compare the
  balances it rebuilds each time with the last ones it logged.
- Crash it, wait until q.esrc.commands has ten or so waiting, and press
  **Start the ledger**. Its output shows the rebuild finishing before the
  first command is taken, then the waiting ones worked through in order.
- Press **Empty the event history** while the ledger runs. Nothing changes
  yet: the ledger already has its balances, and goes on recording events.
  Now crash it and start it. The history it gets back begins part way
  through, and the ledger stops: "the history starts at event #29, not #1:
  the events before it are missing". It can see the hole, so it refuses to
  decide anything. Compare the second Break it card, where the history is
  emptied completely and nothing can tell. To go on, stop the ledger, press
  **Empty the event history** again and start it: it begins at 0.00.

## How Solace does this

- The broker is not an event store. Here its replay log stands in for one:
  it retains published messages up to a fixed size, dropping the oldest
  when full, and replays them into an exclusive queue on request. That is
  made for recovering a consumer that missed messages, not for keeping a
  record forever. An event-sourced system in production needs a durable
  history: an event store, with snapshots and retention designed on
  purpose, and the broker to carry the events.
- A Go receiver asks for a replay with `WithMessageReplay`: all logged
  messages, those since a time, or those after a given message id. The
  broker replaces what the queue holds with the replayed messages and
  delivers them in order. It is refused on a non-exclusive queue, and on a
  temporary queue ("Replay Not Supported on Anonymous Queue Without
  Subscriptions"), which is why q.esrc.rebuild is a durable queue.
- To know when the replay is over, the ledger publishes a marker of its own
  on a topic q.esrc.rebuild subscribes to, and the replay log keeps markers
  as well as events. A message published during a replay is written to the
  log and reaches the queue from there, after everything logged before it,
  so the marker arrives after every replayed event, and the rebuild ends on
  a fact, not on a quiet spell. If the marker were not logged, it would
  never arrive. Measured in `docs/spikes.md`, sections 17 and 23.

Next: [Publish-Subscribe](210-publish-subscribe.md).
