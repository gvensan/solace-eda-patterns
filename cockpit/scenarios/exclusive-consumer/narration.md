# Narration: Exclusive Consumer
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py exclusive-consumer

An exclusive consumer is one consumer that receives every message from a queue, in order, while any others bound to the same queue wait on standby. If the active one goes away, the broker makes a standby active, and it carries on from where the queue stands.

A ledger is the classic case. Postings must be applied one at a time, in order. Apply a refund before the payment it refunds, and the balance is wrong. So there should be exactly one writer. But one writer is also one point of failure.

Here, two ledgers bind to one exclusive queue. The broker makes the first one active, and keeps the second on standby. The messaging API tells each ledger which it is. The payments service publishes a posting every half second, and only the active ledger receives them.

The price is the throughput of a single consumer. And the broker can only see connections: a consumer that hangs while staying connected keeps the queue to itself.

The Break it cards show both sides. Crash the active ledger, and watch the standby take over within a moment, starting with the postings the crashed one had not acknowledged. Then make the active ledger hang instead, and see that nothing fails over, and nothing is consumed, until it is stopped.

Press Play, and see which ledger the broker has made active.
