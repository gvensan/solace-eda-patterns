# Narration: Point-to-Point
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py point-to-point

Point-to-point messaging sends each message to one destination, usually a queue, and each message is taken by exactly one receiver. The queue holds the message until the receiver is ready. On Solace, the recommended form is a topic mapped to a queue: the sender publishes to a topic, one queue subscribes to it, and the sender never names the queue.

It fits work that belongs to one known service. Printing a shipping label is a good example: the order service needs a label printed for every order, by the label printer. Broadcast the request, and every listener might print one. Call the printer directly, and a printer that is offline means lost jobs, or an order service stuck waiting and retrying.

Here, the order service publishes each label job to a topic. One queue subscribes to that topic, and the label printer is the queue's one reader. A sender can also address a queue directly by its name, but then it is tied to that one queue. The broker keeps each job on the queue until the printer has taken it and acknowledged it, so no job is lost just because the printer was busy, or away. If the printer crashed before acknowledging a job, the broker would send it again, so a job may occasionally arrive twice.

The price is a queue for each kind of consumer. And one receiver is kept by how the broker is set up, not by the sender: a second queue on the same topic would get a copy of every job.

The Break it cards show both sides. Take the printer offline, and watch the queue hold the jobs, until it fills and the broker starts refusing new ones. Then misspell the topic in the sender, so that no queue takes its jobs. Such a message is normally dropped without a word; this broker is set to refuse it back to the sender, so you can see every job bounce.

Press Play, and watch the jobs pass through the queue to the printer.
