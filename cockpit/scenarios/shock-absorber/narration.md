# Narration: Shock Absorber
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py shock-absorber

A shock absorber is a queue placed between producers that send in bursts and consumers that work at a steady pace. The queue takes the burst as fast as it arrives, and the consumers drain it at their own speed.

Concert ticket sales are a good example. For most of the day, purchase requests trickle in. When tickets go on sale, fifteen hundred arrive in a couple of seconds. The booking workers, which reserve seats, can each handle about twenty a second. Send the burst straight at them, and they are overwhelmed. Size them for the burst, and they sit idle the rest of the day.

Here, the on-sale page publishes each purchase request to a queue, and the broker stores the burst in seconds. Two booking workers share the queue and work through it at forty a second between them. While the burst fits in the queue, the page is not slowed down, and the workers never see more than they can handle.

The price is that work waits, and a queue has limits. It has a size: a burst bigger than the queue is refused, and the publisher is told. And waiting costs time: a request that waits too long may no longer be worth handling.

The Break it cards show both limits. Release four times the usual burst, and watch the broker refuse what does not fit. Then give each request a seat hold of twenty seconds, and see the ones at the back expire while they wait, and move to a dead message queue.

Press Play, wait for the first burst, and watch the queue fill and drain.
