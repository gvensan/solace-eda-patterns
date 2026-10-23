# Access Control and Authorization (750): design

Category 7, Governance Patterns, fifth pattern. Prefix `ac`. Direct
messaging, so the cards are about the broker's answer, not about delivery.

**One idea.** Who may publish what, and who may subscribe to what, is
decided on the broker, per username, in an ACL profile. An app that
oversteps is refused at the moment it publishes or subscribes, with an
error that says so, and nothing it tried reaches anyone. Topics are the
unit of control: an ACL grants or denies topic patterns.

**Story.** A bank. The **payments** service publishes payment events to
`ac/bank/payment/made/v1/<id>` and may subscribe to anything under
`ac/bank/`. The **reporting** team may subscribe to payment events but
publish nothing. A **partner** may subscribe only to `ac/bank/public/>`,
where the bank would publish rates.

**Broker objects.** No queues. One client profile `cp-ac`. Three ACL
profiles, each with publish and subscribe default `disallow` and
exceptions:

| ACL profile | Username | May publish | May subscribe |
| --- | --- | --- | --- |
| `acl-ac-payments` | `svc-ac-payments` | `ac/bank/payment/>`, `ac/bank/public/>` | `ac/bank/>` |
| `acl-ac-reporting` | `svc-ac-reporting` | nothing | `ac/bank/payment/>` |
| `acl-ac-partner` | `svc-ac-partner` | nothing | `ac/bank/public/>` |

`imports.tsv` lists the exception objects too (they are children of the
ACL profile and go with it, but listing them keeps Reconcile complete).

**Apps.** `ac app --role <name> --user <username> [--subscribe <topic>]
[--publish <topic>]`: subscribes (exits on a refusal, printing the broker's
error and the dashboard's explanation), and publishes once a second,
printing any refusal. Play runs payments (publishes payment events,
subscribes `ac/bank/>`), reporting (subscribes `ac/bank/payment/>`) and
partner (subscribes `ac/bank/public/>`, which nothing publishes to until
the guide's Try this: an accepted subscription is the point).

**Play.** apply, build, reporting, partner, payments.

**Cards.**

1. *Reporting tries to publish a refund.* A failure-only action starts a
   reporting app that publishes to `ac/bank/payment/refund/v1`. Each publish
   is refused: the API's failure listener prints the broker's error (publish
   ACL denied) and the explanation. Nobody receives it: payments, subscribed
   to `ac/bank/>`, sees nothing. Reset stops it. `expect:` break: the app
   `has: ["ACL"]` (exact text from the run); payments `lacks: ["refund"]`.
2. *The partner subscribes to payment events.* A failure-only action starts a
   partner app subscribing to `ac/bank/payment/>`. The subscription is
   refused and the app exits, printing the error and the explanation. Reset
   stops it (already stopped). `expect:` break: the app `has: ["ACL",
   "exited with code 1"]`.

**On the broker.** The three ACL profiles and their exceptions
(`/aclProfiles`, `/aclProfiles/<name>/publishTopicExceptions`,
`/subscribeTopicExceptions`), and the clients' `subscriptions`.

**Differs from neighbours.** The other governance patterns (catalogue,
APIs) describe events; access control enforces who may use them, on the
broker, at run time.

**Spike.** None beyond reading the exact error strings from the first run:
the subcodes (`PublishAclDenied`, `SubscriptionAclDenied`) are already
explained by `internal/solace/errors.go`, from the core workshop.
