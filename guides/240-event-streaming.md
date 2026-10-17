# Event Streaming

A bike-share. Every ride start and ride end is an event, four a second
across forty bikes. The **counter** keeps the number of bikes in use: plus
one on a start, minus one on an end.

That is a stream: **an unbounded, ordered sequence of events, and a
consumer with a position in it, building state as it goes.** The state is
only right if the consumer has seen the stream in order from where its
state started.

Press **Play**. It creates `q.es.rides`, starts the **Counter**, then the
**Bikes**.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Publish-Subscribe**: events delivered and gone. No order is kept and no
  consumer has a position.
- **Point-to-Point** and **Guaranteed Delivery**: a queue holding messages
  for one receiver. Here the queue is the consumer's position in an ordered
  stream, and the consumer's state depends on it.

## Run it

Open the **Counter** log. Once a second it prints "in use: 23 (at #812; the
bikes say 23)". The bikes carry their own count in every event, so the two
numbers can be compared: they match. The counter applies each event to its
set of bikes out, writes that set to a snapshot file, and only then
acknowledges the event.

## On the broker

- **The stream's queue**: `spooledMsgCount` is the length of the stream so
  far. `msgSpoolUsage` is how far the counter is behind: near zero while it
  keeps up.
- **The counter on q.es.rides**: `ackedMsgCount` is its position;
  `redeliveredMsgCount` counts events it was given again after a crash.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The counter crashes.** It is killed mid-stream. The queue's count in
   the diagram climbs: the stream goes on and the queue holds the counter's
   place. **Reset** starts it again: "restored 23 rides in use from the
   snapshot", then "resumed at #812, delivered again after the last stop".
   Its count matches the bikes again straight away, with no "end without a
   start".
2. **The city board joins the stream late.** The board starts now, direct.
   It gets every event from this moment and nothing before, so for a bike
   already out it sees the end and never the start: "end without a start
   for B-12: joined late, no history". Its count sits below the bikes' and
   never catches up. **Reset** stops it.

## Try this

- Stop the **Counter**, start **Start the counter without a snapshot** from
  Step through it, let it run a minute, and use the first card. It resumes
  at the right position but with no state: "in use" starts at zero and
  drifts negative as ends arrive for bikes it forgot. Position and state
  both matter.
- Press **Cleanup**, which deletes the queue and the snapshot together,
  then **Play**. The counter starts from zero with a fresh stream, and the
  count is right because the stream and the state began together.

## How Solace does this

- A queue subscribed to the stream's topics keeps the events in the order
  they were published, and removes each one only when the consumer
  acknowledges it. That is the consumer's position. After a crash the
  unacknowledged events come back marked redelivered, then the rest.
- State is the application's: a snapshot written before each
  acknowledgement always matches the position, which is why the counter
  resumes correctly. Solace can also keep the stream itself for replay (a
  replay log, with the Go API's `WithMessageReplay`), so a consumer can
  rebuild state from further back. Event Sourcing covers that.
- A direct subscriber has no position: it gets what is published while it
  is subscribed. Good for a screen that only needs the latest; wrong for a
  count.

Next: [Hierarchical Topics](310-hierarchical-topics.md).
