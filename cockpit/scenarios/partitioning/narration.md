# Narration: Partitioning
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py partitioning

Partitioning splits a stream of messages by a key. Messages with the same key always go to the same partition, and each partition is read by one consumer at a time. So everything about one key is handled in order, while different keys are handled in parallel.

Order fulfilment shows the need. Each order moves through created, paid, packed, shipped and delivered, and those steps must be handled in that order: packing an order that has not been paid for is a real mistake. But many orders are in flight at once, and one consumer is too slow. Share the work without a key, and one order's steps would reach different consumers, in no particular order.

Here, the order service publishes each update with the order id as its partition key. The queue has six partitions, drawn as lanes in the diagram. The key picks the lane, and the broker gives each lane to one of three consumers, two each. Several orders share a lane, and one order always stays in the same one.

The price comes in three parts. A very busy key loads a single consumer. Partitions move when consumers join or leave, so the owner of an order can change. And a consumer beyond the number of partitions has nothing to do.

The Break it cards show the moves, and the one mistake that undoes the pattern. Crash a consumer, and watch its lanes pass to the others. Add a fourth consumer, and see one lane move to it. Then publish without the key, and watch one order's updates scatter across consumers.

Press Play, pick one order's letter, and follow it down its lane.
