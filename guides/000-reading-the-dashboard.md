# Reading the dashboard

Every scenario page has the same four parts: **Run it**, **Step through it**,
**Break it** and **On the broker**. This page explains the marks you will
see on them. Point at any of them in the dashboard and it tells you the same
thing.

## The diagram

Each box is an app or the broker. Lines show where events go, and every line
starts or ends at the broker: apps never talk to each other directly.

| Mark | What it means |
| --- | --- |
| <!-- indicator: status-dot --> Dot, top left of a box | The app's state. Green and pulsing: running. Dark green: finished successfully. Red: stopped with an error (open its logs to see why). Orange: not running, and broker configuration it needs is missing. Grey: not running. |
| <!-- indicator: config-count --> `1/1`, `2/3` in a box's bottom left | How many of the broker objects this box needs exist on the broker right now, such as its client username or its queue. `1/1` means all of them. `0/1` means something is missing: press Play, or run Apply configuration. Point at it to see the names. |
| Count inside a dotted box in the broker | The same check for that one object, such as a queue. |
| <!-- indicator: box-depth --> `3 waiting`, `2 parked` in a dotted box | How many messages that queue holds right now: waiting to be read, or delivered and not yet acknowledged. Read from the broker every few seconds. It climbs, in orange, when nothing is reading the queue. |
| <!-- indicator: box-arrival --> Orange dot hopping between two boxes, with a word beside it | A message the broker just moved from one queue to another, such as to a dead message queue, and why: `expired` (it waited past its time to live) or `failed 3 attempts`. It comes from the broker's own counters, then runs on to the app reading that queue. |
| <!-- indicator: logs-button --> `logs` | Opens what that app printed, in the side panel. |
| Moving dots on a line | Events flowing. They move only while the app that sends them is running, so a still picture means nothing is being sent. A line into a queue's reader can also follow the queue itself: then it moves only while messages are arriving on that queue, as the broker counts them (across a bridge, or out to a webhook). They show that events flow, not each single event. |
| <!-- indicator: faulted-border --> Red dashed border | A **Break it** card is in effect on this app. It is the failure you caused, not a fault in the workshop. |

A box can also show a setting you chose, under its name:

| Mark | What it means |
| --- | --- |
| <!-- indicator: node-setting --> `clear: 6 orders, 2/s` under an app | The setting it runs with now, such as how fast a publisher goes. Click the box to change it in the side panel. |
| <!-- indicator: setting-choice --> **Clear**, **Busy** in the side panel | The choices for that setting; the filled one is in use. Choosing another restarts the app if it is running, so it takes effect at once. |

Some scenarios also say what the broker decided about a consumer, under its
name:

<!-- indicator: flow-role -->

| Label | What it means |
| --- | --- |
| active consumer | The broker is delivering the queue's messages to this app. |
| standby (dashed box) | The app is bound to the queue, but the broker is delivering to another consumer. It gets nothing until that one goes away. On a partitioned queue, it is a consumer that owns no partition: one more than the queue has partitions, or one waiting for the broker to give it one. |
| active, not acking | The broker still treats this app as an active consumer, but it is holding messages and has acknowledged none for several seconds. The broker sends it nothing more, and the messages it holds wait until it disconnects. On a shared queue the other consumers carry on; on an exclusive queue nothing moves. |

## Partitioned queues

Partitioning draws its queue as one lane per partition. Everything on the
lanes is read from the broker or from the consumers' own logs, about once a
second; nothing moves on a timer.

| Mark | What it means |
| --- | --- |
| <!-- indicator: lane-owner --> `P0`, `P1` and a name on a lane | The partition's number and the consumer the broker gave it to. Everything in that partition goes to that consumer, and to no other. **no owner** (orange, dashed): its consumer went away and the broker has not given it to another yet, so its messages wait. |
| <!-- indicator: lane-held --> `2 held` on a lane | Messages in that partition right now: waiting, or delivered and not yet acknowledged. It climbs, in orange, when nothing reads the partition. |
| <!-- indicator: lane-in --> `412 in` on a lane | Messages that have gone into that partition since the queue was created. Keys are hashed onto partitions, so the counts differ. |
| <!-- indicator: lane-edge --> Solid line from a lane to a consumer | Which consumer that partition is delivered to. When the broker moves a partition, the line swings to the new owner and the lane flashes green. |
| <!-- indicator: partition-status --> `waiting to rebalance`, `moving partitions` above the lanes | A consumer came or went. The broker waits a few seconds (the rebalance delay), then moves partitions. Blank means every partition has an owner and nothing is moving. |
| <!-- indicator: event-dot --> Coloured dot with a letter | One update a consumer handled, drawn a moment after it happened, along the path it took: from the publisher, through its partition, to that consumer, where it shrinks away: consumed and gone from the queue. The colour and letter stand for one order while it is in flight. When it is finished, its letter goes to a new order in the same partition, so a letter keeps to one lane and one consumer. One letter at two consumers means one order really did split. Point at it for the order, its sequence number and the log line. |
| <!-- indicator: event-dot-unplaced --> Faint dashed dot crossing the queue | An update the dashboard could not place in a partition: the broker had already removed it before the dashboard looked. It still goes to the consumer that handled it. |
| <!-- indicator: consumer-partitions --> `partitions P1 and P4` under a consumer | The partitions the broker has given this consumer. **no partition: idle** (orange): it is bound to the queue but owns nothing, so it receives nothing. |
| <!-- indicator: seq-chip --> `A3`, `C1` in a consumer | The last updates this consumer handled, as a line packed against the right end: each one enters at the left of the line and leaves from the right end, oldest first, and the rest slide right as it drains: the order's letter and its sequence number. One order's numbers should rise one at a time. Each leaves after a few seconds, so the row shows recent work, not a backlog. |
| <!-- indicator: seq-chip-flagged --> `A3!` with a red outline | The consumer's own log flagged that update: a **gap** (a number was skipped, so it went to another consumer) or an **out of order** arrival. It stays for 20 seconds, so you have time to point at it; newer updates wait behind it, in order. |

## Following a message

Some diagrams number their lines. Each number is one step a message takes,
and the list under the diagram, **Follow a message**, says what happens at
each step, grouped by the message it belongs to.

| Mark | What it means |
| --- | --- |
| <!-- indicator: flow-step --> `1`, `2` in a circle on a line | The step this line is, in the list under the diagram. Numbers run on from one message's path to the next, so no two lines share one. Click it to highlight the whole path. |
| <!-- indicator: flow-path --> A line you can click | Every numbered line can be clicked. Its whole path stays bright, with the apps and queues it passes, and everything else fades. Click it again, click empty space, or press Esc to clear it. |
| <!-- indicator: box-hop --> A short line between two boxes inside the broker | The broker moving messages from one of its boxes into another, with no app sending them. The label says when it happens (Event Sourcing: "read back at start", the event history copied into the ledger's queue when the ledger starts). Nothing moves along it, because it happens only then. |
| <!-- indicator: path-pick --> A path's name in **Follow a message** | Highlights that path on the diagram, the same as clicking one of its lines. The highlighted path is outlined in the list. |
| <!-- indicator: canvas-expand --> **Expand** and **Collapse**, top right of the diagram | Fills the window with the diagram, its side panel and what each app holds, for a bigger picture or a projector. The **Broken** bar stays above it, so Reset is in reach. **Collapse** or Esc brings the page back. |

## What each app holds

Some scenarios are about what an app keeps because of the events it
receives: a copy of another service's data, a read model, a database. Those
show a card per app under the diagram, filled from the app's own reports
and read every 3 seconds. An app that is not running shows no rows: what it
held in memory went with it.

| Mark | What it means |
| --- | --- |
| <!-- indicator: hold-count --> `8 customers` in a box's bottom row | How many rows the app holds right now, as it last reported. **0** (orange) means it holds nothing: a consumer that has lost its copy, say. Blank when the app is not running. |
| <!-- indicator: hold-kept --> **in memory** or **on disk** on a card | Where the app keeps it. **in memory** (orange): a restart or a crash loses all of it. **on disk**: it is still there after a restart. |
| <!-- indicator: hold-changed --> Green row in a card's table | That row changed since the last read, a few seconds ago: a new version arrived, or a row was added. Counters (lookups asked, say) are never marked: they change on every read. |
| <!-- indicator: hold-last-change --> `Last change, 12:37:40: C-03: city Lyon to Leeds, version 6 to 7` under a table | The last time that table changed, and what changed. When one app keeps a copy of another's data, the same change shows on both cards, a moment apart. |
| <!-- indicator: hold-match --> `Matches Customer Service: 8 of 8` on a card | The app keeps a copy of another app's data, and this compares the two on every column both share. Green: every row is there and the same. Orange: some rows are **missing** from the copy, or **different** (older or newer). **with Customer Service down** means it is compared with what the owner last reported before it stopped. |

## The rest of the page

| Mark | What it means |
| --- | --- |
| <!-- indicator: broker-total --> `2 on broker` under the diagram | How many queues, client profiles, ACL profiles and client usernames the message VPN holds right now, across every scenario and including the broker's own defaults. Read every 5 seconds. The broker box in the diagram lists this scenario's own. |
| <!-- indicator: running-count --> `3 running` in the top bar | Apps or steps running across every scenario. **Stop everything** ends them all. |
| <!-- indicator: step-dot --> Dots on the Step through it flowchart | Each step's state, as on the diagram. Click a step to see what it does, and to run or stop it on its own. |
| <!-- indicator: card-tile --> The row of tiles in Break it | One tile per card. Click a tile to open its card underneath. The open tile is outlined and cannot be pressed again: red while its failure is in effect, green once it has been reset. While a card is in effect the other tiles are faded and locked (**Locked**); reset it first. |
| <!-- indicator: card-tried --> `✓ tried` on a tile | You have broken and reset that card during this visit. You can open it and try it again. |
| <!-- indicator: card-badge --> Badge on a Break it card or tile | **Ready**: you can break it. **Not ready**: press Play first. **Starting**: Play is still starting the apps; wait for it. **Broken**: the failure is in effect, press Reset. **Reset the other first** (**Locked** on a tile): another card is in effect. **Recovered**: Reset is done and the log shows the recovery. |
| <!-- indicator: card-done --> **Done** on a recovered card | The card stays open after Reset so you can watch the recovery in the log. **Done** closes it and goes back to the tiles. |
| <!-- indicator: failure-bar, failure-timer --> **Broken** bar at the top of the page, with `0:42` | A Break it card is in effect, and for how long. It stays at the top while you scroll. **Diagram** and **Card** jump to either; **Reset** puts things back, as on the card. |
| <!-- indicator: inspect-updated, refresh-button --> `Updated 10:31:05` and **Refresh** in On the broker | When the tables were last read from the broker. They are read again every 5 seconds while the page is open; **Refresh** reads them now. |
| <!-- indicator: broker-status --> Event broker, bottom left | **Ready** means the dashboard can reach the broker. **Starting** means it is still booting, which takes about a minute. **Unreachable** means the broker container is not running. |
| <!-- indicator: sidebar-toggle, nav-collapsed --> `«` and `»`, left of the page title | Shrinks the sidebar to pattern numbers, for more room, and back. Collapsed, point at a number to see its pattern's name. The dot at the bottom is still the broker's status. Your choice is remembered in this browser. |
| <!-- indicator: nav-group-fold --> `▾` or `▸` before a category in the sidebar | `▾` means the category's patterns are listed; `▸` means they are folded away under the heading. Click the heading to switch. A search still finds a pattern in a folded category, and opening a pattern from a link unfolds its category. |
| <!-- indicator: nav-fold-all --> `▴` or `▾` button beside **Find a pattern** | `▴` collapses every category to its heading; when all are collapsed it turns to `▾` and expands them all. What you fold is remembered. |
| <!-- indicator: nav-coming-soon, related-coming-soon --> *coming soon* under a category in the sidebar, or after a pattern's name under "Next to it" | That category's patterns are not released yet, so they are not listed and cannot be opened. They appear when they are released. |
| <!-- indicator: nav-search --> Find a pattern, the box under the logo | Type part of a pattern's name or its category and the list shortens to the matches. Enter opens the first match; Escape clears the box. |
| <!-- indicator: narration-open, narration-close --> **Listen: the use case** in the top bar (**Guided tour** on a pattern that has one) | A spoken overview of the pattern and the scenario's use case, a minute and a half to two minutes. It opens a column on the right with the controls and the transcript, and the page narrows to make room; the button hides while the column is open. `×` or Esc closes the column and the audio keeps playing: the button then says **Listening** and the time. Patterns without an overview have no button. |
| <!-- indicator: tour-tab, overview-tab --> **Guided tour** and **Quick overview** tabs in the guide column | Two ways through a scenario. The guided tour goes step by step, waits while you act, and explains what you see. The quick overview is a short spoken story with nothing to do but listen. |
| <!-- indicator: tab-audio-hint --> `audio 1:32` or `text only` under a tab's name | Whether that tab is spoken, and for how long. `text only` means there is no voice yet: the steps are text to read. |
| <!-- indicator: tour-step --> `Step 3 of 14` in the guided tour | Where you are in the tour. It is remembered if you close the tour or leave the page. |
| <!-- indicator: tour-kind --> **Read**, **Your turn** or **Look** beside the step number | What the step asks of you: only read, do something on the page, or look at the part of the page outlined in orange. Every step ends on **Continue**; the tour never moves on by itself. |
| <!-- indicator: tour-check --> A green or orange line under a step, such as `Done. Press Continue.` | Whether the page shows what the step asked for. Orange means not yet, with what is missing. You can still press **Continue**, but the next step will describe something that is not there. |
| <!-- indicator: tour-home --> The house button beside **Back** in the guided tour | Goes to step 1 of the tour. It changes nothing on the page: apps keep running and a broken card stays broken. Press **Cleanup** first if you want to start the scenario from nothing. |
| <!-- indicator: narration-play, narration-rewind, narration-stop --> Play, `-10s` and the square, in the player | Play and pause; go back 10 seconds; stop and go back to the start. Nothing plays until you press play. |
| <!-- indicator: narration-progress, narration-time --> The bar and `0:42 / 1:31` in the player | How far through the overview you are, and its length. Drag or click the bar to jump. |
| <!-- indicator: narration-line --> A sentence in the transcript | The sentence being spoken is marked with a green bar and tint; those already spoken are greyed. Click any sentence to play from there. |

| <!-- indicator: close-panel --> `×` on the side panel | Closes the panel. |

Next: pick any scenario in the sidebar. They are independent, so do them in
any order.
