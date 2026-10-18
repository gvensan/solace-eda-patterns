# Event Gateway (530): design

Category 5, Deployment Patterns, third pattern. Prefix `gw`. Stopped in Wave
B for want of a shared setup change; now built on it: `setup_broker.sh` maps
MQTT and REST to configurable host ports (`SOLACE_MQTT_PORT`,
`SOLACE_REST_PORT`) and adds `host.docker.internal:host-gateway`, so the
broker's REST delivery point can reach a webhook on the host or in the
Codespace.

**One idea.** The broker is the gateway: other protocols in and out at the
edge, one event system inside.

**Real.** MQTT 3.1.1 QoS 1 publishing with PUBACK (a small client written in
Go in the package, so no new library), REST incoming with HTTP basic
authentication and `Solace-Delivery-Mode: persistent`, an SMF consumer of
both, and a REST delivery point posting from q.gw.billing to a plain HTTP
webhook, retrying every 3 s. **Simulated.** Nothing.

**Broker objects.** `q.gw.dispatch` (`gw/fleet/>`, `gw/partner/>`),
`q.gw.billing` (`gw/partner/order/>`), `rdp-gw-billing` with queue binding
and REST consumer (`host.docker.internal:<SOLACE_WEBHOOK_PORT>`), `cp-gw`,
`acl-gw`, usernames `svc-gw-vans`, `-partner`, `-dispatch`.

**Cards.** The webhook goes down (orders wait on q.gw.billing, delivered on
restart); the partner posts outside its ACL (403 Publish ACL Denied at the
gateway). A wrong-password card was planned, but the workshop VPN's basic
authentication type is `none`, so passwords are not checked (spike 20).

**Unverified.** The host route in a Codespace (docker-in-docker): tested on
macOS with Docker Desktop only.

Spike: `docs/spikes.md`, section 20.
