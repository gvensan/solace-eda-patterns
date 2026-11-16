# Narration: Event-Carried State Transfer
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py event-carried-state-transfer

Event-carried state transfer is a pattern in which an event carries the state that changed, not just a notice that something changed. The services that receive it can keep their own copy of that data, and never have to call back the system that owns it.

That matters most for reference data, which many services need and one service owns. In a shop, the customer service owns every customer's record: their name, and the city they live in. Shipping needs the city to send each parcel. Invoicing needs the name and the city to send each bill.

The usual first design is to ask. When an order arrives, call the customer service, and wait for the answer. That works, until the customer service is slow, or down. Then every service that asks is slow, or down, with it.

Here, the customer service publishes the whole record whenever a customer changes. Shipping keeps its own copy from those events, so an order that carries only a customer id is enough: shipping looks the customer up in its own copy. Invoicing is left the old way, asking for every single order, so you can compare the two side by side.

The copy has a price. Every consumer stores one, it is a moment behind the owner, and a service that loses its copy needs a way to get it back. The Break it cards show both sides. Stop the customer service, and watch invoicing stall while shipping carries on. Then restart shipping without its copy, and see what it takes to rebuild it.

Press Play, and compare the Customer Service and Shipping tables as the records arrive.
