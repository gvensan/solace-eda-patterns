# Guaranteed Delivery

A loyalty programme. Every purchase at the tills earns points, and the
points service awards them. Losing an award cheats a customer; awarding it
twice costs the business. Both show up in a customer's balance.

Guaranteed delivery is two promises kept together: **the broker confirms a
message to its publisher once it has stored it, and deletes it only when
the consumer acknowledges it.** Together they mean nothing is lost. Delivery
is then at least once, so it asks one more thing of the consumer: recognise
a message it has already handled (an idempotent receiver), so that nothing
is done twice.

Press **Play**. It creates the queue `q.gd.awards`, starts the **Points
Service**, then the **Tills**, which make a purchase every half second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Point-to-Point** showed a queue holding messages for one consumer.
  Guaranteed Delivery is about the acknowledgements on both sides of that
  queue, and what they mean when something fails.
- **Dead Message Queue** is about a message that can never be processed.
  Here every purchase can be; the question is only whether it arrives once.

## Run it

Open the **Tills** log. Each purchase is sent as a guaranteed (persistent)
message without waiting, and a moment later the broker's receipt for it
comes back: "confirmed P-...". The receipt carries the purchase it
confirms, so the tills know exactly which ones the broker has stored. You
may see a purchase awarded before the tills log its confirmation: the
broker can deliver a message before the publisher hears back.

Open the **Points Service** log. For each purchase it writes the award to
its record on disk, then acknowledges it: "awarded P-...: +15 points to
C-3, balance 120". The record is a file, standing in for the database a
real service would update, so it survives the service stopping.

## On the broker

- **Queue**: `spooledMsgCount` grows with every purchase stored;
  `redeliveredMsgCount` counts purchases delivered a second time.
- **What the queue tells publishers**: `rejectMsgToSenderOnDiscardBehavior`
  is `always`, so a purchase the queue cannot take is refused to the tills.
  It is set that way on purpose. With the broker's default,
  `when-queue-enabled`, a queue whose ingress is off confirms a purchase to
  the tills and then drops it, so the tills would believe it was stored and
  never send it again. Refusing is what lets the tills keep it and retry.
  The second card shows it, and Try this shows the default.
- **The points service on q.gd.awards**: `ackedMsgCount` is purchases
  acknowledged, `unackedMsgCount` those delivered and not yet acknowledged.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The points service stops after awarding, before acknowledging.** This
   is not a kill from outside. The dashboard sends the points service a
   signal (SIGUSR2) that asks it to stop itself right after its next award,
   before it acknowledges that purchase: the one moment a crash causes a
   duplicate. It writes the award, says "stopping now: P-... is awarded but
   not acknowledged", and exits. Press **Reset** to start it again. The
   broker delivers that purchase again, marked redelivered, and the service
   finds it on its record: "already awarded, skipped (redelivered)". No
   purchase is awarded twice.
2. **The queue stops taking purchases for a while.** Ingress on
   `q.gd.awards` is turned off. The tills' receipts now say the broker
   refused a purchase, "refused P-...: Endpoint Shutdown", and the tills
   keep it, and every purchase after it, in an outbox, retrying the oldest
   once a second. **Reset** turns ingress back on: the retry gets through,
   the tills send the rest of the outbox at once, in order, and say "outbox
   empty". The points service awards every one, with no GAP. The outbox
   here is in memory: it survives a refusal, not a till crash. A real till
   keeps it on disk.

## Try this

- Stop the **Points Service**, start **Start the points service without its
  duplicate check** from Step through it, and use the first card again (its
  reset starts the normal service, so stop that and start this one again
  instead). The redelivered purchase is awarded a second time: "awarded
  again P-...". That is what the check prevents.
- In `cockpit/scenarios/guaranteed-delivery/tf/queues.tf`, set
  `reject_msg_to_sender_on_discard_behavior` to `"when-queue-enabled"`, the
  broker's default, press **Play**, and use the second card. The tills now
  log "confirmed" for every purchase while ingress is off, and the points
  service shows a GAP afterwards: the broker confirmed those purchases and
  dropped them. **Queue** counts them in `disabledDiscardedMsgCount`. Set it
  back to `"always"` when you are done.

## How Solace does this

- Each message is sent direct (best effort, no confirmation) or
  persistent, also called guaranteed: the broker stores it before
  confirming. In the Go API you choose by building a direct or a persistent
  publisher. The Go API confirms through a publish receipt, with the
  context you passed to `Publish`.
- The broker deletes a guaranteed message from a queue when the consumer
  acknowledges it. Until then, if the consumer goes, the message is
  delivered again, marked redelivered. Delivery is at least once, so
  recognising a repeat is the consumer's job; here the tills set each
  purchase id as the application message id.
- The redelivered flag says a message was sent before, not that it was
  handled before: measured here, purchases this service never saw also
  came back marked redelivered.

Next: [Backpressure and Push](340-backpressure.md).
