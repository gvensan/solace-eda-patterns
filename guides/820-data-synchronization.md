# Data Synchronization

A company is moving from its old CRM to a new one. The move takes months,
and in the meantime some teams edit customers in the old CRM and some in the
new. Both must show the same customers.

That is data synchronization: **each system publishes the changes made in
it and applies the changes the other publishes, so their data stays the
same.** Two rules keep it from going wrong: a change applied from the other
side is not published again, and every record carries a version, so both
sides settle the same way when they are edited at once.

Press **Play**. It creates two queues and the usernames, then starts the
**Old CRM** and the **New CRM**, each editing a customer every 3 seconds.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Change Data Capture**: changes flow one way, out of one database. Here
  both systems are written to, and changes flow both ways.
- **Strangler**: moves work from the old system to the new, one kind of
  event at a time. Data synchronization keeps both systems' data the same
  while both are still in use.

## Run it

Open the **Old CRM** log: "edited C-04 here: phone 555-0381 (version 7),
published", and "applied C-02 from new: phone 555-9120 (version 5)". Every 5
seconds: "last 5 s: 2 edits made here, 2 changes sent. Table checksum
3fa21c, highest version 9". Open the **New CRM** log: the same checksum.
Both tables are the same.

Now and then both sides edit the same customer at once. Look for "kept my
C-03 (version 6 legacy) over the new CRM's version 6": both sides apply the
same rule (higher version wins, the old CRM wins a tie), so both end with
the same record.

## On the broker

- **Queues**: q.dsync.new holds the old CRM's edits for the new CRM, and
  q.dsync.legacy the other way. Each subscribes to the other side's level of
  the topic, so a CRM never gets its own edits back.
- **Connected clients**: each CRM sends about as many messages as it
  receives.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The sync echoes back and forth.** Both CRMs restart publishing every
   change they apply as a new edit. Each edit now bounces between them for
   ever: "last 5 s: 2 edits made here, 41 changes sent", climbing, and the
   highest version races up. **Reset** restarts both without the echo.
2. **The new CRM is offline for a while.** It crashes, and the old CRM's
   edits wait on q.dsync.new. **Reset** starts it: it opens its saved table,
   applies every edit it missed, in order, and within seconds both print
   the same checksum.

## Try this

- After card 1's reset, compare the two checksums. They match again: the
  loop left high versions behind, but the same ones on both sides.
- Stop the **Old CRM** instead, for half a minute. The new CRM carries on
  alone; the old one catches up when started.

## How Solace does this

- Each side publishes to a topic with its own name in it, and each queue
  subscribes to the other side's name only. Origin is in the topic, so the
  broker does part of the loop prevention.
- Each CRM acknowledges a change only after applying and saving it, so a
  crash loses nothing: unacknowledged changes are delivered again.
- Real synchronization tools also record which changes they applied, to
  skip duplicates, and need a rule for conflicts that matches the business:
  last writer wins, one system wins, or a person decides.

Next: [Cleanup](999-cleanup.md).
