# Event Sourcing

A bank's ledger could keep a table of balances and update it with every
transaction. Then the table is the truth, and how each balance came about
is lost. Or the ledger could keep every transaction as an event, and work
out each balance by adding them up. Then the history is the truth, and the
balances can always be worked out again.

That is event sourcing: **the events are the record, and the current state
is derived from them.** Lose the state, and you derive it again from the
events. On Solace, the broker's replay log keeps a copy of published events
and a consumer can ask for them again.

Press **Play**. It creates the replay log, a queue and a username, then
starts the **Ledger**, which rebuilds its balances from the log (empty, the
first time) and makes a transaction every second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event Streaming**: a consumer keeps its position in a stream. Here the
  consumer goes back to the start of the history on purpose, to rebuild.
- **CQRS**: often used with event sourcing, to build read views from the
  events. Here there is one service, and its own state is what is rebuilt.

## Run it

Open the **Ledger** log: "event #12 applied: A-2 deposited 40.00, balance
140.00". Each event records the balance it left. The ledger does not
update a balance when it decides on a transaction: it publishes the event,
and applies it when the broker delivers it back. A withdrawal larger than
the balance is refused and publishes nothing.

## On the broker

- **Replay log**: `rl-esrc`, 1 MB, the smallest the broker accepts.
  `msgSpoolUsage` grows with every event. A message VPN can have only one
  replay log.
- **What the log keeps**: the topic filter `esrc/>`. Without it the log
  would keep every guaranteed message published in the VPN, every other
  scenario's included.
- **Queue**: q.esrc.ledger is exclusive. The broker refuses a replay into a
  non-exclusive queue.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The ledger crashes.** It is killed, and its balances, which were only
   in memory, with it. **Reset** starts it: it asks the broker to replay
   every logged event and rebuilds, "rebuilt from 57 events: A-1 120.00,
   ...", the same balances as before, every recorded balance matching.
2. **The replay log rolls over.** With the ledger stopped, a statements job
   publishes 1.35 MB of statements into the 1 MB log. The broker drops the
   oldest messages to make room, every transaction among them, and tells
   nobody. The ledger starts and finds no history: "rebuilt from 0 events".
   **Reset** empties the log and the bank starts over: the lost events
   cannot be brought back.

## Try this

- Crash the ledger from Step through it a few times, and compare the
  balances it rebuilds each time with the last ones it logged.
- Press **Publish the monthly statements** while the ledger runs, then
  crash it. The log now starts part way through the history: the ledger
  says "the log starts at event #N", and some balances no longer match the
  events they were built from.

## How Solace does this

- A replay log is configured per message VPN, with a size, and a topic
  filter that says which messages it keeps. It logs messages as they are
  published, whether or not anyone consumes them.
- A Go receiver asks for a replay with `WithMessageReplay`: all logged
  messages, those since a time, or those after a given message id. The
  broker replaces what the queue holds with the replayed messages and
  delivers them in order. It is refused on a non-exclusive queue
  ("Client Initiated Replay Not Allowed on Non-Exclusive Queue"), and a
  replay from before the oldest logged message is refused ("Replay Start
  Time Not Available"). Measured in `docs/spikes.md`, section 17.
- The replay log is bounded and cannot be searched or queried. It is for
  recovering consumers, not for keeping history forever. Event-sourced
  systems keep their events in an event store, and snapshots of the state
  so a rebuild does not start from the first event.

Next: [Publish-Subscribe](210-publish-subscribe.md).
