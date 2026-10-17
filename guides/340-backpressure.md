# Backpressure and Push

Cold-chain monitoring in a refrigerated warehouse. Sensors report 40
temperature readings a second. A **reporter** writes every reading to a
compliance database, which slows down at times, and must not lose any. A
**live display** on the wall shows the current temperatures, and only cares
about the latest.

The broker pushes messages to consumers; it does not wait to be asked. So
**a consumer needs a way to say "not so fast", and what it cannot take has
to wait somewhere, or be dropped.** With a queue, the broker holds the
backlog. Without one, the consumer's own buffer is all there is.

Press **Play**. It creates `q.bp.readings`, starts the **Reporter** and the
**Live Display**, then the **Sensors**.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Shock Absorber**: the queue soaks up a producer's burst. Backpressure is
  the consumer controlling how fast it takes.
- **Competing Consumers**: adds consumers to go faster. Backpressure keeps
  one consumer safe when it cannot.
- **Exclusive and Competing Consumers' hang cards**: a frozen consumer hits
  the unacknowledged limit by accident. Here a slow consumer uses it, and
  pause and resume, on purpose.

## Run it

The sensors publish every reading as a **direct** message: no storage, no
acknowledgement. Two consumers take the same readings in two ways:

- The **Reporter** reads `q.bp.readings`. The queue's subscription stores a
  copy of every direct reading as a guaranteed message. The reporter writes
  each one to the database (10 ms), then acknowledges it. Every 200 writes
  it logs how old the latest reading was.
- The **Live Display** subscribes direct, with no queue. The Go API holds at
  most 20 readings it has not yet shown. Every 2 seconds it logs the
  reading it shows and how old it is.

## On the broker

- **Queue**: `msgSpoolUsage` is near zero while the reporter keeps up.
  `maxDeliveredUnackedMsgsPerFlow` is 100: the most readings the broker
  sends the reporter before it has acknowledged them.
- **The reporter on q.bp.readings**: `unackedMsgCount` is the readings it
  holds now, a handful while it keeps up.
- The display has no queue, so the broker keeps nothing for it.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The database slows down.** Each write now takes 100 ms: 10 a second,
   against 40 arriving. Writes pile up in the reporter, and at 50 pending it
   pauses its own intake: "paused with 50 writes pending". The broker keeps
   sending until 100 are unacknowledged (**The reporter on q.bp.readings**,
   `unackedMsgCount` 100), then stops, and the readings wait on the queue:
   the diagram counts them on `q.bp.readings`. **Reset** brings the database
   back. The reporter resumes, works through what waited, says "caught up",
   and no reading is missing.
2. **The live display falls behind.** It now takes 250 ms to show a
   reading: 4 a second, against 40 arriving. Nothing waits for a direct
   subscriber. The API's buffer of 20 fills, and it drops the oldest to make
   room: "dropped 60 readings in the last 2 s (oldest first)". The reading
   on screen stays less than a second old, which is what a live display
   wants. The reporter misses nothing. **Reset** stops the drops.

## Try this

- Stop the **Live Display**, start **Start the live display, dropping the
  newest** from Step through it, and use the second card. Now the API keeps
  the old readings and drops the new ones: the display keeps showing older
  and older readings ("12.4 s old"). Drop-latest is also the Go API's
  default, with a buffer of 50.
- Stop the **Reporter** for a minute, then start it again. The readings
  waited on the queue, and it writes every one.

## How Solace does this

- Solace pushes messages to consumers. For guaranteed messages the brake is
  the number of messages a consumer holds unacknowledged: the queue's
  `max_delivered_unacked_msgs_per_flow`. A Go consumer can also `Pause()` and
  `Resume()` its receiver.
- A direct-published message is stored by any queue whose subscription
  matches it (measured: `docs/spikes.md`, section 13), so one stream can
  feed a guaranteed consumer and a direct one.
- For direct messages there is no per-message brake. The Go API buffers
  them and, when the buffer is full, drops the oldest
  (`OnBackPressureDropOldest`) or the newest (`OnBackPressureDropLatest`),
  counting them in the `ReceivedMessagesBackpressureDiscarded` metric.

Next: [Exclusive Consumer](350-exclusive-consumer.md).
