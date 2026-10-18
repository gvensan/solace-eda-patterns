# Access Control and Authorization

A bank's payment events. The **payments** service publishes a payment
event for every payment made, and the day's rates. The **reporting** team
needs the payment events. A **partner** needs the rates, and must never
see a payment.

That is access control: **who may publish what, and who may subscribe to
what, is decided on the broker, per username, and enforced the moment an
app tries.** On Solace the unit of control is the topic: an ACL profile
allows or denies topic patterns for publishing and for subscribing.

Press **Play**. It creates one client profile, three ACL profiles with their
topic exceptions, and three usernames, then starts **Reporting**, the
**Partner** and the **Payments** service.

If a mark on the page is unclear, point at it, or see
[Reading the dashboard](000-reading-the-dashboard.md).

## How it differs from its neighbours

- The other governance patterns describe events: a catalogue, an API. Access
  control enforces who may use them, on the broker, at run time.
- **Publish-Subscribe** showed subscribers choosing topics. Here the broker
  decides which topics each username may choose.

## Run it

Three usernames, three ACL profiles:

| Username | May publish | May subscribe |
| --- | --- | --- |
| `svc-ac-payments` | `ac/bank/payment/>`, `ac/bank/public/>` | `ac/bank/>` |
| `svc-ac-reporting` | nothing | `ac/bank/payment/>` |
| `svc-ac-partner` | nothing | `ac/bank/public/>` |

Open the **Reporting** log: "received on ac/bank/payment/made/v1". The
**Partner** receives nothing, because nothing has been published to
`ac/bank/public/` yet; its subscription was accepted, which is the point.
Every app connected without trouble: all three profiles allow connecting.

## On the broker

- **ACL profiles**: each denies publishing and subscribing by default.
- **What payments may publish** and **What the partner may subscribe to**:
  the exceptions that open exactly the topics each role needs.
- **Connected clients**: each client's `aclProfileName`, the profile the
  broker is enforcing for it.

## Break it

With Play running, scroll to **Break it**. Each card causes one failure on
purpose. Press **Break it**, read the log in the card and what it tells you
to look for, then press **Reset**. **Why this breaks** explains the cause
and where you would meet it in production.

1. **Reporting tries to publish a refund.** An app connects as reporting
   and publishes to `ac/bank/payment/refund/v1` once a second, as guaranteed
   messages. It connects fine. Each receipt is a refusal: the broker's
   reason, as the API returns it, then the dashboard's explanation. Open
   the payments service's log: subscribed to everything under `ac/bank/`, it
   sees no refund. A refused message reaches nobody. **Reset** stops the app.
2. **The partner subscribes to payment events.** An app connects as the
   partner and subscribes to `ac/bank/payment/>`. The subscription is
   refused, "Subscription ACL Denied - Topic 'ac/bank/payment/>'", and the
   app exits. Reporting, on the
   same topic under its own username, carries on. **Reset** stops the app,
   which has already stopped.

## Try this

- Start **Start a reporting app that publishes a refund, direct** from Step
  through it. Its log says "published #5", "published #10": the publish
  calls succeed. Payments still sees no refund. The broker dropped each one
  and the app was never told. A direct publisher cannot know whether it was
  allowed; that is why the card publishes guaranteed.
- Make the partner's subscription legal: in
  `cockpit/scenarios/access-control/tf/access.tf`, add `"ac/bank/payment/>"`
  to the partner's `subscribe` list, press **Play**, and use the second
  card again. Now the subscription is accepted. Put it back and press
  **Play**: the running partner app keeps its subscription until it
  reconnects, so restart it to see the new rule applied.
- Publish rates: edit the `payments` action's `--publish` to
  `ac/bank/public/rate/v1` and restart it. The partner now receives, and
  reporting, subscribed to payments only, does not.

## How Solace does this

- An ACL profile has three parts: whether a username may connect, which
  topics it may publish to, and which it may subscribe to. Each has a
  default action (allow or disallow) and a list of exceptions, which are
  topic patterns with wildcards. A client username names one ACL profile.
- Publishing is checked per message, on the broker. A guaranteed publisher
  gets the refusal on the message's receipt. A direct publish to a forbidden
  topic is dropped without telling the publisher (measured on 10.26.0,
  `docs/spikes.md`, section 15). Subscribing is checked when the subscription
  is added, and the Go API returns the broker's error from
  `AddSubscription`.
- Deny by default and grant by role is the usual shape. The usernames
  here are local to the broker; in production the username comes from an
  identity provider, and the ACL profile is assigned by it.

Next: [Strangler](810-strangler.md).
