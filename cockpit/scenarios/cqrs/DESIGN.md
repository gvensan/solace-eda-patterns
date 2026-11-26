# CQRS (120): design

Category 1, Event Generation Patterns, second pattern. Prefix `cqrs`.

**One idea.** Commands and queries are handled by separate models. CQRS
itself allows one database; here, in the usual event-driven form, writing
and reading are separate services with separate data, joined only by
events. The write side handles commands and publishes what
happened; the read side builds the view it needs from those events and
answers queries. Each fails and scales on its own, and the read side is
always somewhat behind.

**Story.** A shop. Checkout sends "place order" commands
(`cqrs/shop/command/place-order/v1/<customer>`) to `q.cqrs.commands`. The
order service numbers each order and publishes "order placed"
(`cqrs/shop/order/placed/v1/<customer>`). `q.cqrs.view` feeds the order
history service, which keeps a count and total per customer and answers the
account page by request-reply (`cqrs/shop/query/order-summary/v1/<customer>`)
with the view's age.

**Broker objects.** `q.cqrs.commands`, `q.cqrs.view`; `cp-cqrs`, `acl-cqrs`,
usernames `svc-cqrs-checkout`, `-write`, `-view`, `-account`.

**Apps.** `cqrs checkout`, `cqrs write`, `cqrs view` (SIGUSR1 slow, SIGUSR2
normal), `cqrs account`.

**Play.** apply, build, history, orders, account, checkout.

**Cards.**

1. *The read side falls behind.* History slows to 2 s per event. The account
   page is answered at once, from a view seconds behind; q.cqrs.view grows;
   the write side is untouched. Reset: normal speed, catches up.
2. *The write side goes down.* Killed. Commands are accepted and wait on
   q.cqrs.commands; reads carry on. Reset starts it and the backlog drains.

**Checks.** Each order keeps its command's id (checkout's ids start with
the time it started, so they are unique across restarts). A command that
comes back after a crash is recognised, not placed twice, and its event is
published again in case the crash stopped it; the read side skips an order
number it has already applied. Added 2026-11-16 after review: a kill
between saving and acknowledging used to place one command as two orders,
and the two tables then disagreed for good. Tested with the test-only
flags `--crash-after-save` and `--crash-after-publish`.

**Real.** Queues, events, request-reply, the lag. **Simulated.** The write
side's database is a file of orders (`.state/cqrs/orders.jsonl`, deleted by
Cleanup); the read side's view is in memory, so a restart loses it.
