# Event Streaming

A bike-share. Every ride start and ride end is an event, four a second
across forty bikes. The **counter** keeps which bikes are in use: the bike
is added on a start and removed on an end.

That is a stream: **an unbounded, ordered sequence of events, and a
consumer with a position in it, building state as it goes.** The state is
only right if the consumer has seen the stream in order from where its
state started.

Press **Play**. It creates `q.es.rides`, starts the **Counter**, then the
**Bikes**.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Publish-Subscribe**: events delivered and gone. Nothing is retained, no
  consumer has a position to resume from, and a missed event is not sent
  again.
- **Point-to-Point** and **Guaranteed Delivery**: a queue holding messages
  for one receiver. Here the queue is the consumer's position in an ordered
  stream, and the consumer's state depends on it.

## Run it

Open the **Counter** log. Once a second it prints "in use: 23 (at #812; the
bikes say 23)". The bikes carry their own total in every event, as a check,
so the two numbers can be compared: they match. A total says how many are
out, never which bikes: that is only in the events. The counter applies each event to its
set of bikes out, writes that set to a snapshot file, and only then
acknowledges the event.

## On the broker

- **The stream's queue**: `spooledMsgCount` is the length of the stream so
  far. `msgSpoolUsage` is how far the counter is behind: near zero while it
  keeps up.
- **The counter on q.es.rides**: `ackedMsgCount` counts the events it has
  acknowledged on this connection, and starts again when it reconnects.
  Its position is the queue itself: the oldest event still there is where
  it resumes. `redeliveredMsgCount` counts events it was given again after
  a crash.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The counter crashes.** It is killed mid-stream. The queue's count in
   the diagram climbs: the stream goes on and the queue holds the counter's
   place. **Reset** starts it again: "restored 23 rides in use from the
   snapshot", then "resumed at #812, delivered again after the last stop".
   Its count matches the bikes again straight away, with no "end without a
   start".
2. **The city board joins the stream late.** The city asks which bikes are
   out and for how long, so the board lists each ride from its start
   event. It starts now, direct, and gets every event from this moment and
   nothing before, so the bikes already out are not on its list: "3 rides
   listed, longest B-07 for 4 s (at #812; the bikes say 21 are out: 18 the
   board cannot name)". Under the diagram, **What each app holds** shows
   the board's short list of rides next to the Counter's full list of
   bikes in use. The total tells it how many rides it is missing, never
   which bikes or since when. When one of them ends: "end without a
   start for B-12: joined late, no history". After about half a minute the
   line says "all of them listed". The starts it missed are gone for good:
   it comes right only because every ride ends. **Reset** stops it.

## Try this

- Stop the **Counter**, start **Start the counter without a snapshot** from
  Step through it, and let it run a minute. Press the first card's
  **Break it**, then, instead of **Reset**, start **Start the counter
  without a snapshot** again. It resumes at the right position but with no
  state: "in use" starts at zero, and each bike that was out logs an "end
  without a start" when it comes back. The count stays low until every one
  of those bikes has ended once. Position and state both matter. The
  snapshot on disk is still the one from before, so press **Cleanup**
  before the next **Play**.
- Press **Cleanup**, which deletes the queue and the snapshot together,
  then **Play**. The counter starts from zero with a fresh stream, and the
  count is right because the stream and the state began together.

## How Solace does this

- A queue subscribed to the stream's topics keeps the events in the order
  they were published, and removes each one only when the consumer
  acknowledges it. That is the consumer's position. After a crash the
  unacknowledged events come back marked redelivered, then the rest.
- Unlike a log-based streaming platform, a queue does not keep an event
  once it is acknowledged: the consumer cannot go back and read it again.
- State is the application's: before each acknowledgement the counter
  writes a snapshot of its count and the event it is at, which is why it
  resumes correctly. An event that comes back after a crash and is already
  in the snapshot is skipped: "#32 is already in the snapshot: skipped". Solace can also keep the stream itself for replay (a
  replay log, with the Go API's `WithMessageReplay`), so a consumer can
  rebuild state from further back. Event Sourcing covers that.
- A direct subscriber has no position: it gets what is published while it
  is subscribed. Good for a screen that only needs the latest value, such
  as the bikes' total; wrong for anything built up from earlier events,
  such as the list of rides in progress.
- The counter resumes because it already had a queue and a snapshot. A
  consumer that is new to the stream has neither: it needs the history
  replayed, or a snapshot to start from.

Next: [Hierarchical Topics](310-hierarchical-topics.md).
