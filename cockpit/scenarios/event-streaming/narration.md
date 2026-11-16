# Narration: Event Streaming
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py event-streaming

Event streaming treats events as an unbounded, ordered sequence. Consumers read the stream in order, each keeping its own position in it, and build state as they go.

A bike-share shows why order and position matter. Every ride start and every ride end is an event. To show how many bikes are in use, a counter adds one for each start and takes one away for each end. That number is only right if the counter has seen every event, in order, from the point its count began. Miss one, or apply one twice, and the count drifts, with nothing to say so.

Here, a queue holds the ride events in the order they were published, and removes each one only when the counter acknowledges it: that acknowledgement is the counter's position. Before each acknowledgement, the counter saves a snapshot of its count, together with that position, so after a crash its state and its position still match. Unlike a log-based streaming platform, a queue does not keep events once they are acknowledged. To read history again, Solace can keep the stream in a replay log.

The Break it cards show both sides. Crash the counter, and watch it resume from its snapshot and its position, with the right count. Then start the city's board late, taking the events direct, with no position and no history, and see its count go wrong from the first ride it never saw start.

Press Play, and compare the counter's count with the number the bikes report.
