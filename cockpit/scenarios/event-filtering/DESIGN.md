# Event Filtering (320): design

Category 3, Consumption Patterns, second pattern. Prefix `ef`.

**One idea.** Filter with the topic first: a subscription names exactly what
a consumer wants and the broker delivers nothing else. Filter on a message
property with a selector only when the topic cannot carry it, and pay for
it: unmatched messages stay on the queue.

**Story.** Sensors publish to `ef/plant/<site>/<line>/<kind>/reading/v1`,
5 a second, with `severity` (1 to 5) as a user property. Berlin
Temperatures reads `q.ef.berlin` (`ef/plant/berlin/*/temp/>`). The Alarm
Desk reads `q.ef.alarms` (`ef/plant/>`, 10 MB) with selector
`severity >= 4`.

**Broker objects.** The two queues; `cp-ef`, `acl-ef`, usernames
`svc-ef-sensors`, `-berlin`, `-alarms`. `purge.sh` empties q.ef.alarms with
the SEMP action `deleteMsgs`.

**Apps.** `ef sensors` (`--rate`, `--pad`), `ef watch` (one role for both
consumers, optional `--selector`).

**Play.** apply, build, berlin, alarms, sensors.

**Cards.**

1. *Unmatched readings fill the alarm queue.* Sensors in high resolution (50
   a second, 8 KB). q.ef.alarms reaches 10 MB in under a minute and the
   broker refuses every reading, "Spool Over Quota", for every queue. Reset:
   normal sensors, purge.
2. *The alarm desk's selector has a typo.* "Invalid Selector" at bind.

Spikes: `docs/spikes.md`, sections 2 and 18.
