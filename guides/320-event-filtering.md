# Event Filtering

A factory has sensors on every line at two sites, and each publishes a
reading several times a second. Berlin's temperature desk wants Berlin's
temperatures and nothing else. The alarm desk wants every reading with
severity 4 or 5, from anywhere.

That is event filtering: **each consumer receives only the events it
wants.** On Solace the first filter is the topic: a subscription with
wildcards says which events to deliver, and the broker sends nothing else.
When a consumer filters on something that is not in the topic but is
carried as a message property, a selector can do it, at a cost this
scenario shows.

Press **Play**. It creates two queues and the usernames, starts **Berlin
Temperatures** and the **Alarm Desk**, then the **Sensors**, 5 readings a
second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Hierarchical Topics**: how to design topic levels so they can be
  filtered. This scenario uses that design, and shows what happens when the
  thing to filter on was left out of it.
- **Publish-Subscribe**: every subscriber gets every event on its topics.
  Filtering is about choosing those topics, or narrowing further.

## Run it

Open the **Berlin Temperatures** log: "reading #212: berlin line2 temp,
severity 1". Every line says berlin and temp. Its queue's subscription,
`ef/plant/berlin/*/temp/>`, did the filtering: the broker never put a Lyon
or vibration reading on it.

Open the **Alarm Desk** log: only severity 4 and 5, from any site. Severity
is not in the topic, so its queue subscribes to every reading and its
consumer binds with the selector `severity >= 4`. Now watch the q.ef.alarms
box on the diagram: the number "left" only grows. Those are the
readings the selector did not match. Nobody will ever take them.

## On the broker

- **Queues**: q.ef.berlin stays near empty. q.ef.alarms's `msgSpoolUsage`
  rises steadily towards its `maxMsgSpoolUsage` of 10 MB, about four hours
  away at this rate (measured: about 670 bytes a second).
- **What q.ef.berlin subscribes to**: its one subscription is the whole
  filter. (q.ef.alarms subscribes to every reading, `ef/plant/>`.)

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **Unmatched readings fill the alarm queue.** The sensors switch to high
   resolution: 50 readings a second, 8 KB each. q.ef.alarms fills its 10 MB
   in under a minute, and then the broker refuses every reading: "Spool
   Over Quota". Not just for the alarm queue: Berlin Temperatures stops
   receiving too, and an alarm published now reaches nobody. **Reset** puts
   the sensors back and deletes what the selector left behind.
2. **The alarm desk's selector has a typo.** It is redeployed with
   "severity = = 4" and cannot bind: "Invalid Selector". **Reset** starts it
   with the right selector, and it takes the alarms that waited.

## Try this

- Stop the **Alarm Desk** node, then in a terminal at the repository's root
  run `bash cockpit/apps/run.sh ef watch --role sweep --user svc-ef-alarms --client-name ef-sweep --queue q.ef.alarms`.
  With no selector, it takes every reading the selector left behind, in
  order: one way to stop the growth, if someone really wants those
  readings. Stop it with Ctrl+C before starting the alarm desk again: on
  this exclusive queue only one consumer is active at a time.
- Imagine severity were a topic level:
  `ef/plant/<site>/<line>/<kind>/<severity>/reading/v1`. The alarm desk's
  queue could subscribe to `ef/plant/*/*/*/4/>` and `ef/plant/*/*/*/5/>`, and
  nothing would be left behind.

## How Solace does this

- Topic subscriptions filter as the broker routes: a message is put only on
  the queues whose subscriptions match it. The broker matches topics
  without reading the message.
- A selector is an SQL-like expression on message properties, given when a
  consumer binds. The broker evaluates it for each message on the queue and
  delivers only the matches. Messages that do not match stay on the queue
  (`docs/spikes.md`, sections 2 and 18). Partitioned queues refuse
  selectors.
- With the queue's reject-msg-to-sender-on-discard setting on (the
  broker's default, and set here), a message a full queue cannot take is
  refused to the publisher and is not stored on any other queue either, so
  every subscriber misses it (`docs/spikes.md`, section 18). With the
  setting off, the full queue drops it silently and the other queues still
  get it.

Next: [Guaranteed Delivery](330-guaranteed-delivery.md).
