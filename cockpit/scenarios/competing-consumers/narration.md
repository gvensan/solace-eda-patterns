# Narration: Competing Consumers
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py competing-consumers

Competing consumers is a pattern in which several identical workers read from one queue, and each message is given to exactly one of them. The workers compete for the messages, so adding a worker adds capacity, and losing one only slows things down.

It fits work that comes in independent pieces. An online shop is a good example: every order needs a confirmation email, and no email depends on any other. At busy times one worker cannot send them fast enough, and a second copy that received every order would send every email twice.

Here, checkout publishes one job for each order. The jobs all land on one shared queue, and the broker hands each job to one of the workers bound to it. Every worker runs the same code. Two workers start with Play, and between them they cannot quite keep up, so the backlog grows. Start a third, and the backlog drains. Nothing else has to change.

The price is order. Jobs are handled in parallel, so nothing says which is finished first. And delivery is at least once: a worker that crashes after sending an email, but before acknowledging the job, leaves that job to be done again by another worker.

The Break it cards show what the queue does when a worker fails. Crash a worker, and watch its unfinished jobs go to the others. Then make a worker hang without disconnecting, and see that only the few jobs it holds are stuck, while the others carry on.

Press Play, watch the queue grow, and then start the third worker.
