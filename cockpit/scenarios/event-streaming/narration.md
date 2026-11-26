# Narration: Event Streaming
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py event-streaming

Event streaming treats events as an unbounded, ordered sequence. Consumers read the stream in order, each keeping its own position in it, and build state as they go.

A bike-share shows why order and position matter. Every ride start and every ride end is an event. To know which bikes are in use, a counter adds the bike on each start and removes it on each end. That list is only right if the counter has seen every event, in order, from the point its state began. Miss one, or apply one twice, and the list drifts.

Here, a queue holds the ride events in the order they were published, and removes each one only when the counter acknowledges it: that acknowledgement is the counter's position. Before each acknowledgement, the counter saves a snapshot of its list, together with that position, so after a crash its state and its position still match. Unlike a log-based streaming platform, a queue does not keep events once they are acknowledged. To read history again, Solace can keep the stream in a replay log.

The Break it cards show both sides. Crash the counter, and watch it resume from its snapshot and its position, with the right bikes. Then start the city's board late. It lists which bikes are out and for how long, taking the events direct, with no position and no history. Each event carries the fleet's total, so the board knows how many rides it is missing, but not which bikes: that was only in the events it never saw.

Press Play, and compare the counter's list with the number the bikes report.
