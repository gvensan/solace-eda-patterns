# Narration: Publish-Subscribe
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py publish-subscribe

Publish-subscribe separates the apps that send events from the apps that use them. A publisher sends each event once, to a topic, without knowing who will receive it. Every app subscribed to that topic gets its own copy.

An online shop shows why that matters. Every order placed is news to several teams: shipping must pack it, analytics counts it, and email thanks the customer. If the shop called each of them in turn, it would have to know them all, wait for each, and change its code whenever a new team wanted the orders.

Here, the shop publishes each order once, on a topic that names the region and the order. Shipping and email subscribe to every order. Analytics uses a wildcard to take European orders only. The broker matches each event against every subscription, and delivers a copy to each match. A new team can start listening tomorrow, and the shop never changes.

These events are direct messages: the fastest kind, delivered to whoever is subscribed at that moment, and stored for no one. That is a choice, not part of the pattern. Publish-subscribe can be guaranteed too, with a queue subscribed to the topic.

The Break it cards show both edges. Stop shipping for a while, and see the orders it missed while it was away. Then start a second copy of shipping, and see every order arrive twice, because each subscriber gets its own copy.

Press Play, and open each subscriber's log to see who received what.
