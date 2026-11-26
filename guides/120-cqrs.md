# CQRS

A shop takes orders and shows each customer an order summary on their
account page: "How many orders have I placed, and how much have I spent?"
Taking an order needs checks and a unique number; answering that question
needs a ready-made line per customer. One model shaped for both does
neither well.

CQRS, Command Query Responsibility Segregation, splits them: **one model
handles commands (writes), and a separate model answers queries (reads).**
The two models can even share a database. In event-driven systems, as
here, they usually do not: the write side publishes an event for each
change, and the read side builds its own view from those events. The
events on the broker are then the only link.

CQRS adds a second model to build and run, so it pays only where reads and
writes really differ, as they do here.

Press **Play**. It creates two queues and the usernames, starts the **Order
Summary** service (the read side), the **Order Service** (the write side),
the **Account Page**, and **Checkout**, a command a second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event-Carried State Transfer**: a consumer keeps a copy of someone
  else's data, in the same shape. In CQRS the read side's view is built for
  one kind of question, and looks nothing like the write side's data.
- **Event Sourcing**: the events are the record, and state is rebuilt from
  them. CQRS is often used with it, but here the write side keeps its own
  database and the events only feed the read side.

## Run it

What to do, in the order the page shows it:

1. **Run it**: press **Play**, then read **Follow a message** under the
   diagram: steps 1 to 4 are a command becoming an order, 5 to 8 a
   question and its answer. Click a path's name, or any numbered line, to
   highlight that message's whole route. **Expand** at the top right fills
   the window with the diagram.
2. **What each app holds**, under that: the same orders in two shapes.
3. **Break it**: open a tile, press **Break it**, watch, press **Reset**.
4. **On the broker**: the queues and clients as the broker reports them.

Start with the four kinds of message. Each app prints the ones it sends.

| Message | Topic | Payload |
| --- | --- | --- |
| Command: place an order | `cqrs/shop/command/place-order/v1/C-03` | `{"command":"103512-12","customer":"C-03","amount":45}` |
| Event: an order was placed | `cqrs/shop/order/placed/v1/C-03` | `{"order":12,"customer":"C-03","amount":45,"placedAt":1793962084297}` |
| Query: what has C-03 ordered? | `cqrs/shop/query/order-summary/v1/C-03` | `{"customer":"C-03"}` |
| Answer | (the asker's reply topic) | `{"customer":"C-03","orders":4,"spent":180,"asOf":12,"lagMs":3}` |

The command asks for something to happen and may still be refused; the
event says it has happened, with the order number the write side gave it.
Here every command is accepted; a real write side validates it (stock,
payment) and may refuse it, as the ledger does in
[Event Sourcing](140-event-sourcing.md).
The answer says which order the view is built up to (`asOf`) and how long
after it was placed that order reached the view (`lagMs`): a read side is
always a little behind, so it says what its answer is built from. `lagMs`
describes the last order applied, and stands still when the view stops
changing. To see how far behind the view is now, compare the two latest
order numbers under the diagram.

Open the **Checkout** log: "command 103512-12: place an order for C-03
(45.00), accepted by the broker". The **Order Service** log: "command
103512-12: order #12 placed for C-03, event published". The **Order Summary** log: "order #12
applied: C-03 now has 4 orders, 0.0 s after it was placed". The **Account
Page** log: "C-03: 4 orders, 180.00 spent (view as of order #12, applied
0.0 s after it was placed; answered in 2 ms)".

**What each app holds**, under the diagram, shows the same orders in two
shapes. The Order Service's database (on disk) keeps every order as it was
placed; the table shows the latest 8. The Order Summary (in memory) keeps
one line per customer: how many orders and how much spent, the shape the
account page asks for. Compare the notes under the two tables: "29
orders in all ... Latest: order #29" and "Built from 29 orders, up to
order #29". Normally they are the same, or one behind.

## On the broker

- **Queues**: q.cqrs.commands holds commands until the write side takes
  them; q.cqrs.view holds events until the read side applies them. Both are
  near empty while everything keeps up. The questions and answers use no
  queue: they are direct messages.
- **Connected clients**: `dataTxMsgCount` is what the broker sent to each
  app. The order service's grows by one a second, the commands. The order
  summary's grows faster: it gets every event and every question. The
  account page's questions never reach the order service.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The read side falls behind.** The order summary service takes 2
   seconds per event, half the rate orders arrive. The account page is
   still answered at once, but from a view "applied 9.0 s after it was
   placed", and growing. Under the
   diagram, the Order Summary falls behind the Order Service's latest
   order, and q.cqrs.view fills up. Checkout and the order service do not
   notice. **Reset** puts it back to normal speed and it catches up.
2. **The write side goes down.** The order service is killed. Checkout's
   commands are still accepted by the broker and wait on q.cqrs.commands:
   each is an order submitted, not yet an order placed.
   Under the diagram its card says it is not running and its orders stay
   on disk. The account page is still answered, from a view that has
   stopped changing. **Reset** starts the order service, which opens its
   database, carries on from the last order number and places every
   waiting order.

## What the order service checks

- **No command placed twice.** Each order keeps the id of its command. If
  the order service crashes after saving an order but before acknowledging
  its command, the broker sends the command again; the order service finds
  the order, logs "already placed as order #16 before the restart: not
  placed twice", and publishes its event again, in case the crash stopped
  it going out. The order summary ignores an order it has already applied:
  "order #25 again: already applied, ignored".

## Try this

- Use **Slow the order summary service down** from Step through it, watch
  the lag grow for half a minute, then **Put the order summary service back
  to normal speed**. Count how long the catch-up takes.
- Stop the **Order Summary** node. The account page's questions now get
  "no answer", while checkout and the order service carry on: the read side
  can fail without stopping writes, too. Start it again. Its view was in
  memory, so it starts from nothing, and the events it had already applied
  were acknowledged and are gone from q.cqrs.view. It applies the ones
  that waited while it was down, but its totals are now short of the
  Order Service's database, and stay short: "Built from 18 orders, up to
  order #29" against "29 orders in all". A real read side needs a way to
  rebuild: a replay of every event (see Event Sourcing), a snapshot, or a
  view kept on disk.

## How Solace does this

- Commands go to a queue: each must be handled once, by the write side,
  even if it is down when the command is sent.
- Events go to topics, and each read side has its own queue subscribed to
  them. A second view (search, reporting) is another queue on the same
  topics, added without touching the write side.
- Queries here use request-reply, which is direct messaging: a question
  nobody answers within the timeout is not kept.

Next: [Change Data Capture](130-change-data-capture.md).
