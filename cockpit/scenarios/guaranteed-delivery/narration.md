# Narration: Guaranteed Delivery
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py guaranteed-delivery

Guaranteed delivery means a message, once accepted by the messaging system, is not lost. The broker stores it before confirming it to the publisher, and deletes it only when the consumer acknowledges it. Delivery is then at least once, so a consumer must also recognise a message it has already handled.

A loyalty programme shows what is at stake. Every purchase at the tills earns points. Lose an award, and a customer is cheated. Award it twice, and the business pays twice. Both show up in a customer's balance.

Here, the tills publish each purchase as a guaranteed message, and wait for the broker's receipt saying it was stored. The points service takes each purchase from its queue, records the award, and only then acknowledges it. If it restarts before acknowledging, the broker delivers the purchase again, and the points service sees it has already been awarded, and skips it.

The price is work on both sides: the publisher must keep a message until it is confirmed, and the consumer must deal with repeats.

The Break it cards test both promises. The points service crashes between recording an award and acknowledging it, and the purchase comes back, marked redelivered. Then the queue stops taking purchases for a while, and the tills keep what was refused, and send it again once the queue is back.

Press Play, and compare the tills' log with the points service's.
