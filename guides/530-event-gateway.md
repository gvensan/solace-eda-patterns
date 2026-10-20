# Event Gateway

A delivery company's vans have telematics boxes that speak MQTT. A partner's
order portal can make HTTP calls and nothing else. The billing system takes
webhooks. The dispatch service uses the Solace API. They all need the same
events.

That is an event gateway: **one entry and exit point that lets clients
speaking other protocols take part in the event system, translated at the
edge.** On Solace the broker is the gateway: it speaks MQTT and REST itself,
alongside its own protocol, and can post messages out to HTTP endpoints.

Press **Play**. It creates two queues, a REST delivery point and the
usernames, then starts **Billing's webhook**, **Dispatch**, the **Van** and
the **Partner Portal**.

This is not a security demo. The workshop's broker does no authentication
on its message VPN: it does not check passwords, and even an unknown
username is accepted. What refuses a publisher here is the ACL of the
username it gives. A real gateway authenticates every client, with
passwords, certificates or OAuth, and over TLS.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- **Event Bridge**: joins two messaging domains that speak the same
  protocol. A gateway brings other protocols in.
- **Point-to-Point**: a queue feeding one consumer. Here one consumer is not
  a messaging client at all: the broker posts to it over HTTP.

## Run it

Open the **Van** log: "report #12: 51.5081, -0.1290 sent over MQTT, PUBACK
received". The **Partner Portal**: "O-12345 posted over HTTP: 200 OK". Open
**Dispatch**: "V-07 position #12 (sent over MQTT, read here over SMF)" and
"O-12345, 2 parcel(s) to collect (sent over HTTP, read here over SMF)". One
queue, one consumer, three protocols. Open **Billing's webhook**: "POST
/billing/orders from the broker: O-12345 billed". Billing has no Solace
client; the broker called it.

## On the broker

- **REST delivery point**: the webhook's host and port as the broker sees
  them, and `up` while the broker can reach it.
- **Connected clients, by protocol**: the van shows as MQTT, the partner as
  REST, dispatch as SMF, all under their own usernames.
- **Queues**: q.gw.billing holds orders until the webhook takes them.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**: the page moves up to the diagram, with the
broken app's log open beside it, and a bar at the top keeps the card's name,
how long it has been broken, and **Reset** in reach. **Card** in that bar
takes you back to the card's log and what to look for. **Why this breaks**
explains the cause and where you would meet it in production.

1. **Billing's webhook goes down.** The broker cannot post to it, so orders
   wait on q.gw.billing and the line to billing stops; dispatch carries on.
   **Reset** starts the webhook, and the waiting orders are posted to it.
2. **The partner posts to a topic it may not use.** A new release posts
   pickup requests straight onto the fleet's topics. Every post is refused
   at the gateway, "403 (Publish ACL Denied)", and never becomes
   an event. The van carries on. **Reset** goes back to the earlier release.

## Try this

- Publish by hand: `curl -u svc-gw-partner:solace-workshop -H
  "Solace-Delivery-Mode: persistent" -d '{"order":"O-TEST","parcels":1}'
  localhost:${SOLACE_REST_PORT:-9000}/TOPIC/gw/partner/order/created/v1/O-TEST`
  and watch dispatch and billing.
- Stop **Dispatch**. Positions and orders wait on q.gw.dispatch, while
  billing keeps getting orders from its own queue.

## How Solace does this

- MQTT and REST are services of the message VPN, each on its own port. An
  MQTT or REST publisher logs in with a client username like any client, and
  its messages are ordinary events: a QoS 1 MQTT message or a persistent
  REST post is guaranteed. Its username's ACL profile decides where it may
  publish (`docs/spikes.md`, section 20). In this workshop's broker the VPN
  does not check passwords, so the ACL is what refuses.
- A REST delivery point is the way out: it consumes a queue and posts each
  message to an HTTP endpoint, deleting it only when the endpoint answers
  2xx, and retrying otherwise.
- In a Codespace or locally, the broker runs in a container and reaches the
  webhook on the machine as `host.docker.internal`, which `setup_broker.sh`
  sets up.

Next: [Dead Message Queue](610-dead-letter-queue.md).
