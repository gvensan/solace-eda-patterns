# Narration: Change Data Capture
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py change-data-capture

Change data capture turns the committed changes in a database into a stream of events, without changing the application that writes to it. Its usual form reads the database's own transaction log, so every insert, update and delete can reach other systems as it happens.

It is the pattern for systems that cannot, or should not, be modified. A warehouse's inventory app is a typical case: it is old, it works, and nobody wants to touch it. It writes every sale and every restock to its own database, and it knows nothing about events. Yet the shop's search needs current stock figures.

Changing the inventory app to publish events means editing, testing and redeploying a system everyone is wary of. Polling its tables for changed rows, the simplest form of capture, adds load, and misses what happens between polls, deletes included.

So a connector reads the database's change log, in order, and publishes an event for each change. The search index keeps its stock figures from those events. The inventory app is not changed at all. In the diagram, it has no line to the broker.

There is a price. The events mirror tables, not business intent: they say a row changed, not why. Delivery is at least once. And a database keeps its log only for so long. The Break it cards show how the connector copes. Crash it, and watch it carry on from its saved position, with nothing missed. Then make it lose that position, and see old changes published again, and the search index recognise them by the database's own sequence number, and ignore them.

Press Play, and compare the Inventory App's database with the Search Index: the same stock for every product, a moment apart.
