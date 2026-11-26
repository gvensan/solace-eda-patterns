# Narration: Consumer Groups
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py consumer-groups

Consumer groups let several applications each receive every event, while the instances of each application share the work between them. Each group gets its own copy of the stream, and inside a group each event goes to one member.

A product catalogue shows why both halves matter. Every price change is needed by two teams. Search must update the search index. Alerts must email the customers who are watching that product. Each team needs every price change, and each team runs more than one instance to keep up.

Solace has no consumer group object. Here, each group is its own queue, subscribed to the same topic. The catalogue publishes each price change once, and the broker stores a copy on each queue. The members of a group bind to their group's queue and share it. Each queue has two partitions, so a group of two splits the products between its members, and one product's changes stay in order.

The price is a queue for every group to size and look after. And a group can only use as many members as its queue has partitions: one more than that binds without error, and then has nothing to do.

The Break it cards show both points. Take the whole alerts group offline, and watch search carry on while the alerts queue holds what alerts will need, so alerts is late, not wrong. Then add a third alerts member to a queue with two partitions, and see it stand by.

Press Play, and compare what the search members and the alerts member receive.
