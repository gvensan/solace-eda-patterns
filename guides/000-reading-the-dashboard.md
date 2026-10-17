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
| <!-- indicator: logs-button --> `logs` | Opens what that app printed, in the side panel. |
| Moving dots on a line | Events flowing. They move only while the app that sends them is running, so a still picture means nothing is being sent. They show that events flow, not each single event. |
| <!-- indicator: faulted-border --> Red dashed border | A **Break it** card is in effect on this app. It is the failure you caused, not a fault in the workshop. |

Some scenarios also say what the broker decided about a consumer, under its
name:

<!-- indicator: flow-role -->

| Label | What it means |
| --- | --- |
| active consumer | The broker is delivering the queue's messages to this app. |
| standby (dashed box) | The app is bound to the queue, but the broker is delivering to another consumer. It gets nothing until that one goes away. |
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
| <!-- indicator: event-dot --> Coloured dot with a letter | One update a consumer handled, drawn a moment after it happened, along the path it took: from the publisher, through its partition, to that consumer, where it shrinks away: consumed and gone from the queue. The colour and letter stand for one order, so the same letter is the same order. When an order is finished, its letter goes to a new one. Point at it for the order, its sequence number and the log line. |
| <!-- indicator: event-dot-unplaced --> Faint dashed dot crossing the queue | An update the dashboard could not place in a partition: the broker had already removed it before the dashboard looked. It still goes to the consumer that handled it. |
| <!-- indicator: consumer-partitions --> `partitions P1 and P4` under a consumer | The partitions the broker has given this consumer. **no partition: idle** (orange): it is bound to the queue but owns nothing, so it receives nothing. |
| <!-- indicator: seq-chip --> `A3`, `C1` in a consumer | The last updates this consumer handled, newest on the right: the order's letter and its sequence number. One order's numbers should rise one at a time. Each fades after a few seconds, so the row shows recent work, not a backlog. |
| <!-- indicator: seq-chip-flagged --> `A3!` with a red outline | The consumer's own log flagged that update: a **gap** (a number was skipped, so it went to another consumer) or an **out of order** arrival. It stays for 20 seconds, so you have time to point at it. |

## The rest of the page

| Mark | What it means |
| --- | --- |
| <!-- indicator: running-count --> `3 running` in the top bar | Apps or steps running across every scenario. **Stop everything** ends them all. |
| <!-- indicator: step-dot --> Dots on the Step through it flowchart | Each step's state, as on the diagram. Click a step to see what it does, and to run or stop it on its own. |
| <!-- indicator: card-badge --> Badge on a Break it card | **Ready**: you can break it. **Not ready**: press Play first. **Broken**: the failure is in effect, press Reset. **Reset the other first**: another card is in effect. **Recovered**: Reset is done and the log shows the recovery. |
| <!-- indicator: inspect-updated, refresh-button --> `Updated 10:31:05` and **Refresh** in On the broker | When the tables were last read from the broker. They are read again every 5 seconds while the page is open; **Refresh** reads them now. |
| <!-- indicator: broker-status --> Event broker, bottom left | **Ready** means the dashboard can reach the broker. **Starting** means it is still booting, which takes about a minute. **Unreachable** means the broker container is not running. |

| <!-- indicator: close-panel --> `×` on the side panel | Closes the panel. |

Next: pick any scenario in the sidebar. They are independent, so do them in
any order.
