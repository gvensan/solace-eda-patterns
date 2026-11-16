# Narration: CQRS
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py cqrs

Command Query Responsibility Segregation, or CQRS, separates the part of a system that changes data from the part that answers questions about it. Commands go to a write model, which checks and records them. Queries go to a separate read model, shaped for the questions it answers. The two models can share a database, but in event-driven systems the read model usually has its own store, kept up to date from the write side's events.

The split pays off when writes and reads want different things, and an online shop is a good example. Checkout writes one order at a time, and every write must be checked. The account page asks again and again, and wants a ready-made answer: how many orders, and which was the latest.

Put both on one database, and every question adds up a customer's orders on the same tables checkout writes to. As the shop grows, the questions slow down the orders. And if that one database is down, nobody can place an order or look one up.

In this scenario, checkout sends commands to the order service, the write side. Here it accepts every command; a real one would check stock and payment first. It saves each order, and publishes an event saying an order was placed. The order history service, the read side, builds its own view from those events: one line per customer, in exactly the shape the account page asks for. The two sides share nothing but the events.

The price is that the read side is always a little behind the writes, and the product has to decide what users see in between. Two models are also more to build and run than one, so CQRS pays only where reads and writes really differ. The Break it cards show both sides. Slow the read side down, and watch the account page get answers that describe the past, while checkout carries on. Then take the write side down, and see questions still answered, while commands wait on their queue until it is back.

Press Play, and compare the Order Service and Order History tables: the same orders, kept twice, in two shapes.
