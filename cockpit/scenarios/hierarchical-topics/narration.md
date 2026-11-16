# Narration: Hierarchical Topics
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py hierarchical-topics

A hierarchical topic is an address made of levels, from the general to the specific, with one fact about the event in each level. Subscribers choose the slice of events they want with wildcards, and the broker does the filtering as it routes.

A retail chain shows why. Every sale happens in a region, in a store, in a department. Put those facts into the topic, in a fixed order, and each team can ask for exactly its share: one department everywhere, one region, one store, or everything.

Here, twelve stores in three regions publish four sales a second, each on a topic that names the region, the store and the department. The electronics team uses a star for the region and the store, so it gets electronics from everywhere. The EU team names its region, and takes the rest with a greater-than sign. Audit takes everything. The stores publish every sale the same way; all the slicing is in the subscriptions.

The price is that the levels become a contract. Change them, or add one, and subscriptions that counted on the old shape quietly stop matching.

The Break it cards show both ways this goes wrong. A new team forgets a level, subscribes without any error, and receives nothing. Then the stores add a level, and the team that counted levels goes quiet, while the teams using greater-than carry on.

Press Play, and compare the three teams' logs.
