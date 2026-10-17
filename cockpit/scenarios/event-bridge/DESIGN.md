# Event Bridge (510): design

Category 5, Deployment Patterns, first pattern. Prefix `br`.

**One idea.** Two separate messaging domains, each with its own users,
topics and queues, joined by a bridge that carries a configured set of
events from one to the other. Apps on either side know nothing of it.

**One broker, two VPNs.** The rule is one broker per diagram, and a Standard
broker allows three VPNs. Head office is VPN `br-hq`; the stores are the
workshop VPN. A loopback bridge to `127.0.0.1:55555` is configured exactly
as one to another broker would be, with that broker's address (spikes
sections 1 and 19). The guide says so.

**Broker objects.** In `br-hq`: the VPN, `cp-br-hq` (allows bridge
connections), `acl-br-hq`, `svc-br-pricing`, `svc-br-bridge`,
`q.br.to-stores` subscribed to `br/hq/price/>`. In the workshop VPN: bridge
`br-hq-link` binding `q.br.to-stores` (guaranteed), `q.br.store`
(`br/hq/>`), `cp-br`, `acl-br`, `svc-br-store`. Apps pick the VPN with
`SOLACE_MSG_VPN` (`env` in the action's cmd).

**Apps.** `br hq` (`--promos` publishes to `br/hq/promo/v1/<sku>`),
`br store` (prints each message's age).

**Play.** apply, build, store, hq.

**Cards.**

1. *The bridge goes down.* Disabled over SEMP; prices wait on
   q.br.to-stores; reset enables it and they arrive late, in order.
2. *Head office publishes something the bridge does not carry.* Promotions
   match the store's subscription but nothing in br-hq: silently gone.

**Known gap.** Clear broker config deletes only queues, usernames and
profiles in the workshop VPN, so it leaves `br-hq` and the bridge;
Cleanup's terraform destroy removes them (`docs/polish.md`).
