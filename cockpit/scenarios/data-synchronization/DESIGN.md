# Data Synchronization (820): design

Category 8, Migration Patterns, second pattern. Prefix `dsync`.

**One idea.** Two systems that are both written to stay the same by
publishing their changes and applying each other's. It needs an origin on
each change (an applied change is not published again) and a version (both
sides settle concurrent edits the same way).

**Story.** An old and a new CRM during a migration. Each edits a random
customer every 3 s, publishes to
`dsync/crm/customer/changed/v1/<side>/<customer>`, and applies the other
side's from its queue. A record is applied if its version is higher, or
equal and from the old CRM. Tables are saved to files under the dashboard's
state folder, so a restart keeps them. Every 5 s each CRM prints edits made,
changes sent, a checksum and the highest version.

**Broker objects.** `q.dsync.legacy` (`.../v1/new/>`), `q.dsync.new`
(`.../v1/legacy/>`); `cp-dsync`, `acl-dsync`, usernames `svc-dsync-legacy`,
`svc-dsync-new`.

**Apps.** `dsync crm --side legacy|new` (`--echo` republishes applied
changes with a new version; `--forget` deletes both tables).

**Play.** apply, build, legacy, new. Cleanup: destroy, forget-tables.

**Cards.**

1. *The sync echoes back and forth.* Both sides with `--echo`: changes sent
   far exceed edits made. Reset: no echo.
2. *The new CRM is offline for a while.* Killed; the old CRM's edits wait on
   q.dsync.new; reset applies them in order and the checksums match again.
