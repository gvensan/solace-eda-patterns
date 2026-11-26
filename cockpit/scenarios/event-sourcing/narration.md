# Narration: Event Sourcing
# Spoken by the page's audio player. Lines starting with # are not spoken.
# After editing, run: python3 cockpit/scripts/narrate.py event-sourcing

Event sourcing stores every change to a system's state as an event: appended in order, and never edited or deleted. The current state is not the record. It is derived from the events, and can always be rebuilt by replaying them.

Banking is the classic fit, because a ledger must say not just what a balance is, but how it got there. A simpler design stores the balance and updates it in place, with the history kept to one side, where it can drift from the balance it is meant to explain.

This ledger keeps no separate record of balances. A teller sends commands, such as a deposit or a withdrawal. The ledger checks each one against the current balances, and if it is accepted, records it as an event. That event is the only thing the ledger ever writes. Every balance is simply that account's events, added up. So the history cannot drift from the balance, because the balance is worked out from the history. And if the ledger loses its balances, it can add the events up again.

The price is that the event history must be complete and durable. In this demo, the broker's replay log stands in for a real event store.

The Break it cards show why. Crash the ledger, and watch it rebuild exactly the same balances from the events. Then lose the event history itself, and see every balance lost with it, because the events were the only record.

Press Play, and watch the Ledger's balances change as the teller's commands arrive.
