# Change Data Capture

A warehouse's inventory app has run for fifteen years. It writes stock
changes to its own database, and nobody wants to touch its code. The new
search page needs to know the stock of every item, as it changes.

That is change data capture: **a connector reads the database's own log of
committed changes and publishes an event for each one.** The application
that made the change is not involved and is not changed.

Press **Play**. It creates a queue and two usernames, starts the **Search
Index**, the **CDC Connector**, and the **Inventory App**, which sells or
restocks something every second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event-Carried State Transfer**: the owning service publishes its own
  events, carrying its state. Here the owner publishes nothing; the events
  are taken from its database.
- **Data Synchronization**: two systems kept the same in both directions.
  Change data capture is one way: out of one database.

## Run it

Open the **Inventory App** log: "sold SKU-03: stock 12 -> 11 (change #45
written to the database)". It never connects to the broker; the diagram has
no line from it. The **CDC Connector** log: "change #45 published: SKU-03 12
-> 11". The **Search Index** log: "SKU-03 shows 11 in stock (change #45)".

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
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The connector crashes.** It is killed. The inventory app carries on
   writing and notices nothing; the search index gets nothing new.
   **Reset** starts the connector: "saved position found: resuming after
   change #N", then every change it missed, in order.
2. **The connector loses its saved position.** It restarts reading from the
   first change and publishes everything again. The search index logs each
   repeat, "change #12 for SKU-03 again: already applied, ignored", and its
   figures do not go backwards. **Reset** starts the normal connector.

## Try this

- Stop the **Search Index** node for 20 seconds, then start it. The changes
  waited on q.cdc.search, and it catches up.
- Stop the **Inventory App**. The connector has nothing to publish and waits.
  The connector follows the database, not the app.

## How Solace does this

- Each change is a guaranteed message on a topic per item, so consumers can
  take every SKU or a slice of them.
- The connector saves its position only after the broker accepts a change.
  That gives at least once: a change can be published twice, never skipped.
- In production the connector is a tool such as Debezium, publishing to the
  broker. This scenario uses a small Go connector so every step is visible.

Next: [Publish-Subscribe](210-publish-subscribe.md).
