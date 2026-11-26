# Change Data Capture

A warehouse's inventory app has run for fifteen years. It writes stock
changes to its own database, and nobody wants to touch its code. The new
search page needs to know the stock of every item, as it changes.

That is change data capture: **the committed changes in a database become
events, without changing the application that made them.** The usual form,
and the one here, is log-based: a connector reads the database's own log of
committed changes and publishes an event for each one. Simpler forms poll
the tables for changed rows, or add triggers; polling adds load to the
database and misses what happens between polls, deletes included.

The application is not changed. The database is set up for it: its change
log kept, and a connector allowed to read it.

Press **Play**. It creates a queue and two usernames, starts the **Search
Index**, the **CDC Connector**, and the **Inventory App**, which sells or
restocks something every second.

To be walked through this page step by step, press **Guided tour** in the
top bar: it tells you what to do, waits while you do it, and explains what
you see. Its second tab, **Quick overview**, is a short spoken version.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event-Carried State Transfer**: the owning service publishes its own
  events, carrying its state. Here the owner publishes nothing; the events
  are taken from its database.
- **Data Synchronization**: two systems kept the same in both directions.
  Change data capture is one way: out of one database.

## Run it

What to do, in the order the page shows it:

1. **Run it**: press **Play**, then read **Follow a message** under the
   diagram. Click the path's name, or a numbered line, to highlight it.
   **Expand** at the top right fills the window with the diagram.
2. **What each app holds**, under that: the inventory database, the
   connector's position, and the search index built from the events.
3. **Break it**: open a tile, press **Break it**, watch, press **Reset**.
4. **On the broker**: the queue and clients as the broker reports them.

Start with what the event carries. The connector publishes one per
committed change, on a topic per SKU:

| Event | Topic | Payload |
| --- | --- | --- |
| A stock row changed | `cdc/inventory/stock/changed/v1/SKU-03` | `{"lsn":45,"table":"stock","op":"update","sku":"SKU-03","before":12,"after":11}` |

It describes a row in a table, not a business event: `table`, `op`
(`insert` or `update`), and the values `before` and `after`. Whether 12 to
11 was a sale or a correction, the event cannot say; that is the cost of
taking events from a database rather than from the application. The `lsn`
is the database's own number for the change, in commit order. The search
index uses it to tell a new change from one it has seen.

Open the **Inventory App** log: "sold SKU-03: stock 12 -> 11 (change #45
written to the database)". It never connects to the broker; the diagram has
no line from it. A new database starts with its opening stock, committed
as changes #1 to #6. The **CDC Connector** log: "change #45 published:
SKU-03 12 -> 11". The **Search Index** log: "SKU-03 shows 11 in stock
(change #45)".

**What each app holds**, under the diagram, shows it without the logs. The
Inventory App's database (on disk) has stock per SKU and the change that
last set it. The connector's card has its saved position against the
latest change in the database. The Search Index (in memory) has the same
columns, and "Matches Inventory App: 6 of 6" compares the two.

The database here is a file of committed changes, one line each, with the
change's number, under the dashboard's state folder. A real connector reads
a real database's log, such as the PostgreSQL write-ahead log or the MySQL
binlog. The idea is the same: read in order, publish, remember how far you
got.

## On the broker

- **Queue**: q.cdc.search, near empty while the search index keeps up.
- **Connected clients**: the connector and the search index. The inventory
  app is not there.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The connector crashes.** It is killed. The inventory app carries on
   writing and notices nothing; the search index gets nothing new, and
   under the diagram it falls out of step with the database. **Reset**
   starts the connector: "saved position found: resuming after change #N",
   then every change it missed, in order, and the two match again.
2. **The connector loses its saved position.** It restarts reading from the
   first change and publishes everything again. The search index logs each
   repeat, "change #12 for SKU-03 again: already applied, ignored", and its
   figures do not go backwards: it still matches the database, and its card
   counts the repeats. **Reset** starts the normal connector.

## Try this

- Stop the **Search Index** node for 20 seconds, then start it. The changes
  made meanwhile waited on q.cdc.search, and it applies them. But its index
  was in memory, so it starts from nothing: a SKU that has not changed
  since shows as missing under the diagram until it next changes. The
  events it had already applied were acknowledged and are gone from the
  queue. A real index is kept on disk, or rebuilt from a snapshot of the
  database.
- Stop the **Inventory App**. The connector has nothing to publish and waits.
  The connector follows the database, not the app.

## How Solace does this

- Each change is a guaranteed message on a topic per item, so consumers can
  take every SKU or a slice of them.
- The connector saves its position only after the broker accepts a change.
  That gives at least once: a change can be published twice, never skipped.
- In production the connector is a tool such as Debezium, publishing to the
  broker. This scenario uses a small Go connector so every step is visible.

Next: [Event Sourcing](140-event-sourcing.md).
