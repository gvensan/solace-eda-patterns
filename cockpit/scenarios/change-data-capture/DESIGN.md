# Change Data Capture (130): design

Category 1, Event Generation Patterns, third pattern. Prefix `cdc`.

**One idea.** Events can come from an application that does not publish
any: a connector reads the database's log of committed changes and
publishes one event per change. The application is untouched. Delivery is
at least once, so each change carries the database's sequence number.

**Story.** A warehouse. The inventory app sells and restocks, writing each
change to its database. The connector publishes them to
`cdc/inventory/stock/changed/v1/<sku>`; the search index keeps stock per SKU
from `q.cdc.search`.

**Simulated, and said so.** The database is a file of committed changes, one
JSON line each with a log sequence number, under the dashboard's state
folder. A real connector reads a real database's log (PostgreSQL WAL, MySQL
binlog); the shape is the same: read in order, publish, save the position.

**Broker objects.** `q.cdc.search`; `cp-cdc`, `acl-cdc`, usernames
`svc-cdc-connector`, `svc-cdc-search`. The inventory app has no username.

**Apps.** `cdc inventory` (no broker connection; `--forget` deletes the
database and saved position), `cdc connector` (`--from-start` ignores the
saved position), `cdc search` (ignores a change not newer than the last one
applied for its SKU).

**Play.** apply, build, search, connector, inventory. Cleanup: destroy,
forget-database.

**Cards.**

1. *The connector crashes.* Killed; inventory keeps writing; reset resumes
   from the saved position, no gap.
2. *The connector loses its saved position.* Restarted from the first
   change; the search index logs each repeat and ignores it.
