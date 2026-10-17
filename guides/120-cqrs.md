# CQRS

A shop takes orders and shows customers their order history. Taking an
order needs checks and a unique number; showing history needs fast lookups
by customer. One database tuned for both does neither well.

CQRS, Command Query Responsibility Segregation, splits them: **a write side
handles commands and publishes an event for each change, and a read side
builds its own view from those events and answers queries from it.** The two
share no database. The events on the broker are the only link.

Press **Play**. It creates two queues and the usernames, starts the **Order
History** service (the read side), the **Order Service** (the write side),
the **Account Page**, and **Checkout**, a command a second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event-Carried State Transfer**: a consumer keeps a copy of someone
  else's data. In CQRS the read side's view is built for one kind of
  question, and may look nothing like the write side's data.
- **Event Sourcing**: the events are the record, and state is rebuilt from
  them. CQRS is often used with it, but here the write side keeps its own
  state and the events only feed the read side.

## Run it

Open the **Checkout** log: "command #12: place an order for C-03 (45.00),
accepted by the broker". The **Order Service** log: "command #12: order
#48112 placed for C-03, event published". The **Order History** log: "order
#48112 applied: C-03 now has 4 orders, 0.0 s after it was placed". The
**Account Page** log: "C-03: 4 orders, 180.00 spent (view as of order
#48112, 0.0 s behind; answered in 2 ms)". Every answer says how old the view
is.

## On the broker

- **Queues**: q.cqrs.commands holds commands until the write side takes
  them; q.cqrs.view holds events until the read side applies them. Both are
  near empty while everything keeps up.
- **Connected clients**: the account page's requests go to the order history
  service only. Nothing reads from the order service to answer a question.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **The read side falls behind.** The order history service takes 2
   seconds per event, half the rate orders arrive. The account page is
   still answered at once, but "9.0 s behind" and growing, and q.cqrs.view
   fills up. Checkout and the order service do not notice. **Reset** puts
   it back to normal speed and it catches up.
2. **The write side goes down.** The order service is killed. Checkout's
   commands are still accepted by the broker and wait on q.cqrs.commands.
   The account page is still answered, from a view that has stopped
   changing. **Reset** starts the order service, which places every waiting
   order.

## Try this

- Use **Slow the order history service down** from Step through it, watch
  the lag grow for half a minute, then **Put the order history service back
  to normal speed**. Count how long the catch-up takes.
- Stop the **Order History** node. The account page's questions now time
  out, while checkout and the order service carry on: the read side can fail
  without stopping writes, too.

## How Solace does this

- Commands go to a queue: each must be handled once, by the write side,
  even if it is down when the command is sent.
- Events go to topics, and each read side has its own queue subscribed to
  them. A second view (search, reporting) is another queue on the same
  topics, added without touching the write side.
- Queries here use request-reply, which is direct messaging: a question
  nobody answers within the timeout is not kept.

Next: [Change Data Capture](130-change-data-capture.md).
