# Request-Reply

A supermarket till needs the price of an item before it can ring it up.
It cannot just announce that it wants a price; it needs an answer, now.

That is request-reply: **an app publishes a request and waits for a reply
addressed back to it, with a timeout.** With the Solace Go API, as here,
it is two direct messages. The request goes to whoever is subscribed to the request topic
at that moment; the reply comes back on a topic the asker's API made for
it. With no reply, the price is still unknown when the till's deadline for
that item expires, and the till is told. The deadline is per item: the till
goes on asking about other items while earlier requests are outstanding.

In the Go API, request-reply works with direct messages only. Nothing is
stored on the way, and the cards show what that means.

Press **Play**. It creates the usernames, starts the **Pricing Service**,
then the **Till**, which asks a price every second.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Publish-Subscribe**: the publisher expects no answer and does not know
  who listens. Here the asker needs exactly one answer, back to itself.
- **Point-to-Point**: one receiver, with a queue that keeps the message
  until it is taken. Here nothing is queued; the asker waits, then gives
  up.

## Run it

Open the **Till** log: "request #12: SKU-07 costs 14.99 (answered by
pricing in 23 ms)". The **Pricing Service** log shows the same request from
its side: "request #12: SKU-07 costs 14.99, replied". The till published
to `rr/pricing/price/request/v1/SKU-07`; the pricing service subscribed to
`rr/pricing/price/request/v1/>` and replied to the topic the till's API
asked it to.

## On the broker

- **Connected clients**: the till's `dataRxMsgCount` is its requests and
  `dataTxMsgCount` the replies it received; the pricing service's are the
  other way round. They match while every request is answered.
- **What the till subscribed to**: the reply topic the API made for the
  till, with its client name in it. That is where replies come back.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **The pricing service goes down.** It is killed. The till's requests
   get no reply and, after 2 seconds each, the API's timeout error. Nothing
   waits for the pricing service: a direct request reaches only a subscriber
   that is there when it is sent. **Reset** starts it again, and the next
   request is answered. The unanswered ones stay unanswered.
2. **The pricing service is slow.** It restarts taking 3 seconds per
   answer, longer than the till waits. Every request times out at the till.
   The pricing service answers one request at a time while the till asks
   one a second, so a backlog of requests builds up inside it (in the
   receiver, not on a broker queue): it still
   logs "replied" for each, but later and later, each long after the till
   gave up, and the till's API discards those replies. **Reset** restarts
   the pricing service at normal speed; the requests waiting in the slow
   one go with it.

## Try this

- Start **Start a second pricing service** from Step through it. Both
  services subscribe to the request topic, so both receive every request
  and both reply: compare the two pricing logs. The till takes whichever
  reply arrives first, sometimes "answered by pricing", sometimes "answered
  by pricing-2", and the API discards the other. Every price is looked up
  twice. To share requests among repliers instead, they would use a shared
  subscription (see [Consumer Groups](420-consumer-groups.md)).
- Change the till's timeout: edit the `till` action in
  `cockpit/scenarios/request-reply/scenario.yaml` to add `--timeout 10s`,
  restart the till, and use the second card. A 3-second answer now gets
  through. The timeout is the asker's decision, and it should match what
  the replier can do.

## How Solace does this

- The Go API's request-reply publisher sends a direct message with a reply
  topic and a correlation id, and subscribes to that reply topic; the
  replier's `Reply` publishes a direct message there. The API matches the
  reply to the request and calls the handler, or calls it with a timeout
  error when the time passes. Request-reply is direct only in the Go API.
- Direct messages are not stored, acknowledged or retried. A request sent
  while no replier is subscribed is gone, and a reply sent after the asker
  stopped waiting is gone too. Retry, fallback and idempotency are the
  asker's and replier's to build.
- Other Solace APIs, such as JMS, can also send a request as a guaranteed
  message to a queue, with the reply on a temporary queue, so a request
  waits for a replier that is briefly away. The Go API does not.
- For a request that must be served eventually rather than answered now,
  put the request on a queue (Point-to-Point) and publish the result as an
  event; the asker subscribes to that.

Next: [Event Streaming](240-event-streaming.md).
