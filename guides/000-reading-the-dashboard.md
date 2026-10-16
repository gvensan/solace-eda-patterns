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
| Dot, top left of a box | The app's state. Green and pulsing: running. Dark green: finished successfully. Red: stopped with an error (open its logs to see why). Orange: not running, and broker configuration it needs is missing. Grey: not running. |
| `1/1`, `2/3` in a box's bottom left | How many of the broker objects this box needs exist on the broker right now, such as its client username or its queue. `1/1` means all of them. `0/1` means something is missing: press Play, or run Apply configuration. Point at it to see the names. |
| Count inside a dotted box in the broker | The same check for that one object, such as a queue. |
| `logs` | Opens what that app printed, in the side panel. |
| Moving dots on a line | Events flowing. They move only while the app that sends them is running, so a still picture means nothing is being sent. They show that events flow, not each single event. |
| Red dashed border | A **Break it** card is in effect on this app. It is the failure you caused, not a fault in the workshop. |

Some scenarios also say what the broker decided about a consumer, under its
name:

| Label | What it means |
| --- | --- |
| active consumer | The broker is delivering the queue's messages to this app. |
| standby (dashed box) | The app is bound to the queue, but the broker is delivering to another consumer. It gets nothing until that one goes away. |
| active, not acking | The broker still treats this app as the active consumer, but it is holding messages and has acknowledged none for several seconds. The broker sends it nothing more, and no other consumer gets them. |

## The rest of the page

| Mark | What it means |
| --- | --- |
| `3 running` in the top bar | Apps or steps running across every scenario. **Stop everything** ends them all. |
| Dots on the Step through it flowchart | Each step's state, as on the diagram. Click a step to see what it does, and to run or stop it on its own. |
| Badge on a Break it card | **Ready**: you can break it. **Not ready**: press Play first. **Broken**: the failure is in effect, press Reset. **Reset the other first**: another card is in effect. **Recovered**: Reset is done and the log shows the recovery. |
| `Updated 10:31:05` in On the broker | When the tables were last read from the broker. They are read again every 5 seconds while the page is open; **Refresh** reads them now. |
| Event broker, bottom left | **Ready** means the dashboard can reach the broker. **Starting** means it is still booting, which takes about a minute. **Unreachable** means the broker container is not running. |

Next: pick any scenario in the sidebar. They are independent, so do them in
any order.
