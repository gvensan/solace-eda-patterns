# Event Streaming (240): design

Category 2, Communication Patterns, fourth pattern. Prefix `es`.

**One idea.** A stream is an unbounded, ordered sequence of events, and a
consumer of a stream has a position in it. Processing a stream usually
means keeping state across events (a count, a window, a running total), and
that state is only right if the consumer has seen the stream from where
its state started, in order. A consumer that resumes carries on from its
position; one that joins late has no history.

**Story.** A bike-share. Every ride start and ride end is an event,
`es/bikes/ride/started/v1/<bike>` and `es/bikes/ride/ended/v1/<bike>`, four
a second across 40 bikes. The **counter** keeps the number of bikes in use:
plus one on a start, minus one on an end. The city's **board** shows the
same number on a screen.

**Broker objects.** Queue `q.es.rides`, exclusive, subscription
`es/bikes/ride/>`, `max_delivered_unacked_msgs_per_flow` 10. `cp-es`,
`acl-es`, usernames `svc-es-bikes`, `svc-es-counter`, `svc-es-board`.

**Apps.** `es bikes` publishes the events, guaranteed, numbered. `es counter`
(one app, client name `es-counter`) reads the queue with client
acknowledgement, keeps the set of bikes in use, writes that set to a file
under `COCKPIT_STATE_DIR` before acknowledging each event (its snapshot),
and prints "in use: N" once a second; an end for a bike it has no start for
is flagged. `--no-snapshot` keeps the set in memory only. `es board` takes
the events direct and counts the same way, with no queue and no snapshot.

**Play.** apply, build, counter, bikes. (The board is started by a card.)

**Cards.**

1. *The counter crashes.* SIGKILL. Reset starts it again: it restores its
   set from the snapshot ("restored 23 rides in use from the snapshot"),
   resumes the queue at its position (the unacknowledged events come back
   marked redelivered), and the count carries on right: no "end without a
   start". `expect:` break: counter `has: ["exited with code -9"]`. Reset:
   counter `has: ["restored \d+ rides in use", "resumed at #\d+"]`,
   `lacks: ["end without a start"]`.
2. *The board joins the stream late.* The board starts mid-stream, direct.
   It gets every event from now on, and no history: within seconds it logs
   ends for bikes it never saw start, "end without a start for B-12: joined
   late, no history", and its count sits below the counter's until every
   bike it never saw start has ended once (about 30 s, measured). Reset stops
   it. `expect:` break: board `has: ["end without a start"]`; counter
   `lacks: ["end without a start"]`.

**Try this.** Restart the counter with `--no-snapshot` and use card 1: it
resumes at the right position but with an empty set, so its count is low
until every bike that was out has ended once. State and position both
matter. Replay (a replay log, and the
Go API's `WithMessageReplay`) is the broker's way to rebuild state from
further back; Event Sourcing covers it.

**Differs from neighbours.** Publish-Subscribe: events delivered and gone;
no order kept, no position. Point-to-Point and Guaranteed Delivery: a queue
holding messages for one receiver; here the queue is a position in an
ordered stream and the consumer's state depends on it.

**Spike.** None: guaranteed consumption with client acknowledgement and
redelivery after a kill are measured (sections 6 and 12).
