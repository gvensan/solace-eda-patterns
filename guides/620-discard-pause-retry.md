# Discard, Pause and Retry

A payment processor charges each payment at the bank. Sometimes a charge
fails. What the processor should do next depends entirely on why.

That is discard, pause and retry: **a message that can never succeed is
discarded, a message that failed because of the moment is retried later,
and when a dependency is down altogether, consumption pauses until it is
back.** Get it wrong and you either lose good messages or block on bad ones.

Press **Play**. It creates three queues and the usernames, starts the
**Bank**, the **Payment Processor** and the **Shop**, a payment a second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Dead Message Queue**: what happens to a message the consumer gives up
  on. Here that is one of three answers, used only for messages that can
  never succeed.
- **Backpressure**: a consumer slowing down because it is overloaded. Here
  the processor pauses because what it depends on is down.

## Run it

Open the **Payment Processor** log: "P-0012 charged (attempt 1)". The
**Bank** log: "P-0012: charged". Each payment is a request to the bank and
a reply, and the processor acknowledges the payment only after the bank has
charged it.

## On the broker

- **Queues**: q.dpr.payments holds new payments, q.dpr.retry payments
  waiting for another try, q.dpr.dmq payments discarded. All near empty
  while things go well.
- **Queue settings**: q.dpr.retry's `deliveryDelay` is 5 seconds: a message
  published to it cannot be delivered for 5 seconds. Both queues name
  q.dpr.dmq as their `deadMsgQueue`.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The bank goes down.** The processor's charge gets no answer, so it
   pauses: "pausing payments, checking the bank every 2 s". Payments wait on
   q.dpr.payments, and none is discarded. **Reset** starts the bank, the
   processor resumes, and the waiting payments are charged in order.
2. **Malformed payments arrive.** A faulty till sends payments whose amount
   is not a number. Each is discarded at once to q.dpr.dmq, and the good
   payments around them carry on. **Reset** stops the faulty till.
3. **The bank is busy now and then.** Every third charge is answered
   "busy". That payment goes on q.dpr.retry: "retry 1 of 2 in 5 s", and 5
   seconds later it is charged, "charged (attempt 2)". **Reset** restarts
   the bank as before.

## Try this

- Restart the bank with `--busy-every 1` (edit the `bank-busy` action). Every
  payment is busy three times and ends on q.dpr.dmq. Retries have a limit,
  or a message that will never succeed loops forever.
- Stop the bank during card 3's retries. The delayed payments come back
  from q.dpr.retry, find the bank down, and the processor pauses: the same
  rule applies to retries.

## How Solace does this

- Discard: the Go receiver settles the message as rejected, and the broker
  moves it to the queue's dead message queue.
- Retry: the processor republishes the payment, with its attempt count, to
  a topic q.dpr.retry subscribes to, waits for the broker to accept it, and
  only then acknowledges the original. The queue's delivery delay holds it
  back. Solace queues also have a redelivery delay, but it needs transacted
  sessions, which the Go API does not have (`docs/spikes.md`, section 5).
- Pause: the Go receiver's Pause stops delivery to the app; messages stay on
  the queue. Resume starts it again.

Next: [Saga](630-saga.md).
