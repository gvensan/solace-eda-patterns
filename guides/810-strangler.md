# Strangler

A shop's order system is fifteen years old. Rewriting it in one go is too
risky, so the team replaces it a piece at a time, starting with refunds.
The new refund service runs beside the old system until it is trusted, and
one day the old system's refund code is simply never called again.

That is the strangler pattern (after the strangler fig, which grows around
a tree until the tree is gone): **move one capability at a time from the old
system to a new one, by changing where its events go.** With a broker in the
middle, that is a change of subscriptions. The shop publishes as before and
the old system is not modified.

Press **Play**. It creates two queues and the usernames, sets the starting
routing (refunds to the old system only), starts the **Old Order System**
and the **New Refund Service**, then the **Shop**.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Data Synchronization**: keeps two systems' data the same while both are
  in use. The strangler moves the work itself, one kind of event at a time.
- **Hierarchical Topics**: the topic design is what makes this possible.
  Refunds have their own topic, so they can be routed on their own.

## Run it

Open the **Old Order System** log: orders, payments, and now and then
"refunded R-00042 (old order system)". The **New Refund Service** is
connected and idle: its queue subscribes to nothing.

Now migrate, from Step through it:

1. **Step 1: route refunds to the new service.** Wait for the next refund:
   it appears in both logs. Both queues match the topic.
2. **Step 2: stop routing refunds to the old system.** From now on refunds
   appear only in the new service's log. The old system still handles
   orders and payments.

Press **Play** again to go back to the start.

## On the broker

- **Who gets refunds** and **The new service's subscriptions**: the two
  queues' subscriptions, which are the whole migration. Watch them change as
  you take each step.
- **Queues**: q.stg.refunds counts refunds that nobody has handled yet.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **Refunds go to both systems.** Step 1 is taken and step 2 is not. Each
   refund is paid by both systems. **Reset** routes refunds to the old
   system only.
2. **The new service fails after the cut-over.** Refunds are moved, old
   route first, then the new service crashes. Refunds wait on
   q.stg.refunds; the old system gets none. **Reset** starts the new service,
   which handles the waiting refunds, and routes refunds back to the old
   system.

## Try this

- Take the two steps the other way round: step 2 first, wait for a refund,
  then step 1. A refund published in between matches no queue: the shop's
  log has it, and neither service ever sees it. Step 1 first doubles
  refunds; step 2 first loses them. That is why the move should be one
  quick change, and why the new service should recognise work already done.
- After the cut-over, stop the **Old Order System** node. Orders and
  payments wait for it; refunds carry on. The two capabilities are now
  independent.

## How Solace does this

- A queue's topic subscriptions decide what it receives. Adding or removing
  one is a configuration change on the broker, here made over SEMP, the
  same API terraform and Solace Broker Manager use.
- Messages already on a queue stay there when its subscriptions change.
  Moving the route does not move the backlog.
- In a real migration the routing would be in the pipeline that configures
  the broker, so the cut-over is reviewed and can be undone like any other
  change.

Next: [Data Synchronization](820-data-synchronization.md).
