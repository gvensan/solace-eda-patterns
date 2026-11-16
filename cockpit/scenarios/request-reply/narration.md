# Narration: Request-Reply
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py request-reply

Request-reply is a conversation of two messages. One app sends a request, and waits for a reply addressed back to it. Because the reply may never come, the wait always has a timeout.

It is the pattern for an app that cannot carry on without an answer. A supermarket till is a good example: it cannot ring up an item until it knows the price. Announcing that it needs a price, and hoping, is not enough. The till needs one answer, to this request, now.

Here, the till publishes a request on a topic that names the item, and waits up to two seconds. The pricing service, subscribed to that topic, looks the price up and replies. The till's messaging API made a reply topic for it, and matches each reply to its request. With the Solace Go API, as here, both are direct messages, and nothing is stored on the way. Other Solace APIs can also send requests as guaranteed messages, on a queue.

The price, with direct messages, is that both sides must be up at the same moment. A slow replier slows every asker, and timeouts and retries are the asker's job.

The Break it cards show what that means. Take the pricing service down, and watch every request time out. Then make it slower than the till will wait, and see replies arrive after nobody is waiting for them, and get dropped.

Press Play, and compare the till's log with the pricing service's.
