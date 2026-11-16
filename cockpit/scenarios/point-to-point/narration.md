# Narration: Point-to-Point
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py point-to-point

Point-to-point messaging sends each message to one named destination, usually a queue, and each message is taken by exactly one receiver. The sender knows where its message goes, and the queue holds it until the receiver is ready.

It fits work that belongs to one known service. Printing a shipping label is a good example: the order service needs a label printed for every order, by the label printer. Broadcast the request, and every listener might print one. Call the printer directly, and a printer that is offline means lost jobs, or an order service stuck waiting and retrying.

Here, the order service sends each label job to the printer's queue, by name. The label printer is the queue's one reader. The broker keeps each job on the queue until the printer has taken it and acknowledged it, so no job is lost just because the printer was busy, or away. If the printer crashed before acknowledging a job, the broker would send it again, so a job may occasionally arrive twice.

The price is coupling by name. The sender must know the queue, and if another kind of consumer needs the same jobs, it needs a destination of its own.

The Break it cards show both sides. Take the printer offline, and watch the queue hold the jobs, until it fills and the broker starts refusing new ones. Then misspell the queue's name in the sender. Such a message is normally dropped without a word; this broker is set to refuse it back to the sender, so you can see every job bounce.

Press Play, and watch the jobs pass through the queue to the printer.
