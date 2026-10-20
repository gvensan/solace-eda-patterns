# Solace Event-Driven Architecture Patterns Workshop

A hands-on tour of event-driven architecture patterns on the Solace Broker.
Each pattern is its own small, working system that you run, break and
inspect: press Play, watch the pattern work, cause a failure on purpose, and
see the real error and the recovery.

The patterns come from the Solace
[guide to event-driven architecture patterns](https://solace.com/event-driven-architecture-patterns/).
Everything runs in a container. There is nothing to install and no account to
create.

<p align="center">
  <a href="https://codespaces.new/SolaceDev/eda-patterns-workshop/tree/main?quickstart=1">
    <img src="https://github.com/codespaces/badge.svg" alt="Open in GitHub Codespaces" width="600">
  </a>
</p>

## Getting started

Open the repository in a Codespace with the button above, or locally in VS
Code with the Dev Containers extension (**Reopen in Container**). Either way
the container builds the apps, starts a Solace broker in Docker, and opens the
Solace Workshop Dashboard on port 3000.

The first start takes a few minutes, mostly pulling the broker image. When it
finishes, read [Reading the dashboard](guides/000-reading-the-dashboard.md),
then pick any scenario in the sidebar. They are independent, so you can
do them in any order.

## Scenarios

The sidebar follows the eight categories of the pattern guide.

| # | Scenario | Category | What it shows |
| --- | --- | --- | --- |
| 000 | [Reading the dashboard](guides/000-reading-the-dashboard.md) | | What every dot, count and label on the page means |
| 110 | [Event-Carried State Transfer](guides/110-event-carried-state-transfer.md) | Event Generation Patterns | Consumers keep a copy from events that carry the state; the owner goes down and only the caller stops; a copy lost and given back |
| 120 | [CQRS](guides/120-cqrs.md) | Event Generation Patterns | Commands on one side, a view built from events on the other; reads carry on while writes are down; a view that falls behind and says so |
| 130 | [Change Data Capture](guides/130-change-data-capture.md) | Event Generation Patterns | Events from a database's change log, with the app untouched; a connector that resumes from its saved position; old changes published again and ignored |
| 140 | [Event Sourcing](guides/140-event-sourcing.md) | Event Generation Patterns | Balances rebuilt from events replayed by the broker; a crash costs nothing; a replay log that rolls over takes the history with it |
| 210 | [Publish-Subscribe](guides/210-publish-subscribe.md) | Communication Patterns | One event, a copy for every subscriber; what a subscriber misses while away; two copies of a subscriber double the work |
| 220 | [Point-to-Point](guides/220-point-to-point.md) | Communication Patterns | One sender, one named queue, one receiver; what the queue holds while the receiver is away |
| 230 | [Request-Reply](guides/230-request-reply.md) | Communication Patterns | Ask and wait, with a timeout; nobody there to answer; a reply that comes too late |
| 240 | [Event Streaming](guides/240-event-streaming.md) | Communication Patterns | A consumer's position and state in an ordered stream; resuming after a crash; joining late with no history |
| 310 | [Hierarchical Topics](guides/310-hierarchical-topics.md) | Consumption Patterns | Topic levels as a contract; wildcard slices; a subscription that miscounts, a publisher that adds a level |
| 320 | [Event Filtering](guides/320-event-filtering.md) | Consumption Patterns | Topic subscriptions filter for free; a selector filters on a property and leaves the rest on the queue until the publisher is refused; a selector typo |
| 330 | [Guaranteed Delivery](guides/330-guaranteed-delivery.md) | Consumption Patterns | The broker confirms what it stored; the consumer acknowledges what it handled and skips what it has seen; refused purchases kept and resent |
| 340 | [Backpressure and Push](guides/340-backpressure.md) | Consumption Patterns | A consumer that pauses and a broker that stops at the unacknowledged limit; a direct subscriber that must drop |
| 350 | [Exclusive Consumer](guides/350-exclusive-consumer.md) | Consumption Patterns | One active consumer and a standby on an exclusive queue, failover, and the hang it cannot detect |
| 410 | [Competing Consumers](guides/410-competing-consumers.md) | Consumer Scalability Patterns | Identical workers share one queue; add one to go faster, lose one and the others carry on |
| 420 | [Consumer Groups](guides/420-consumer-groups.md) | Consumer Scalability Patterns | A queue per group: every group gets every change, members share; a group offline catches up; a member beyond the partitions sits idle |
| 430 | [Shock Absorber](guides/430-shock-absorber.md) | Consumer Scalability Patterns | A queue taking a burst the workers drain at their pace; a burst too big; requests that go stale |
| 440 | [Partitioning](guides/440-partitioning.md) | Consumer Scalability Patterns | Same key, same partition, same consumer, in order; partitions moving on a crash or a join; what a missing key does |
| 510 | [Event Bridge](guides/510-event-bridge.md) | Deployment Patterns | Two VPNs joined by a bridge that carries a chosen set of events; prices wait while the bridge is down; an event the bridge was never told about |
| 530 | [Event Gateway](guides/530-event-gateway.md) | Deployment Patterns | MQTT and HTTP in, one Solace API consumer, a REST delivery point out to a webhook; orders wait while the webhook is down; a partner refused at the gateway |
| 610 | [Dead Message Queue](guides/610-dead-letter-queue.md) | Error Handling Patterns | Retry a fixed number of times, then park on a DMQ; orders that expire while waiting; what decides parked or dropped on 10.26.0 and 10.8.1 |
| 620 | [Discard, Pause and Retry](guides/620-discard-pause-retry.md) | Error Handling Patterns | Discard a message that can never succeed, retry one that failed because of the moment, pause when the bank is down |
| 630 | [Saga](guides/630-saga.md) | Error Handling Patterns | Three services, each its own step and event; a sold-out item undone by a refund; a service down delays the sagas that need it |
| 710 | [Event Catalog](guides/710-event-catalog.md) | Governance Patterns | An AsyncAPI catalog compared with real traffic; an event nobody catalogued; a payload that changes shape |
| 720 | [Event APIs](guides/720-event-apis.md) | Governance Patterns | A versioned event contract published v1 and v2 side by side; a version changed in place; a version retired while still in use |
| 730 | [Intermediated Governance](guides/730-intermediated-governance.md) | Governance Patterns | A central team approves each request and the broker enforces the grant; subscribing before approval; a grant broader than the request |
| 740 | [Disintermediated Governance](guides/740-disintermediated-governance.md) | Governance Patterns | Teams act in their own topic prefix and create their own queues; the broker refuses a publish outside the prefix and a queue over the quota |
| 750 | [Access Control and Authorization](guides/750-access-control.md) | Governance Patterns | Per-username ACL profiles; a refused publish reaches nobody; a refused subscription |
| 810 | [Strangler](guides/810-strangler.md) | Migration Patterns | Move refunds from the old system to a new service by moving a subscription; both systems paying the same refund; refunds stranded on a failed new service |
| 820 | [Data Synchronization](guides/820-data-synchronization.md) | Migration Patterns | Two CRMs written at once, kept the same by publishing and applying each other's edits; an edit that echoes for ever; a CRM offline catches up |
| 999 | [Cleanup](guides/999-cleanup.md) | | Tearing it down |

## Pattern coverage

All 30 patterns of the pattern guide, by number (category times 100 plus
position times 10). The commit is the one that added the scenario or the
brief.

| # | Pattern | Status | Commit |
| --- | --- | --- | --- |
| 110 | [Event-Carried State Transfer](guides/110-event-carried-state-transfer.md) | built | 1a507dd |
| 120 | [CQRS](guides/120-cqrs.md) | built | 3a0d04b |
| 130 | [Change Data Capture](guides/130-change-data-capture.md) | built | 0b6185e |
| 140 | [Event Sourcing](guides/140-event-sourcing.md) | built | fe46c03 |
| 210 | [Publish-Subscribe](guides/210-publish-subscribe.md) | built | 4086a28 |
| 220 | [Point-to-Point](guides/220-point-to-point.md) | built | 287d512 |
| 230 | [Request-Reply](guides/230-request-reply.md) | built | eae80cb |
| 240 | [Event Streaming](guides/240-event-streaming.md) | built | 4c3fb5e |
| 310 | [Hierarchical Topics](guides/310-hierarchical-topics.md) | built | 454b372 |
| 320 | [Event Filtering](guides/320-event-filtering.md) | built | 8f77358 |
| 330 | [Guaranteed Delivery](guides/330-guaranteed-delivery.md) | built | 94bae00 |
| 340 | [Backpressure and Push](guides/340-backpressure.md) | built | b98391a |
| 350 | [Exclusive Consumer](guides/350-exclusive-consumer.md) | built | 054e7da |
| 410 | [Competing Consumers](guides/410-competing-consumers.md) | built | a098285 |
| 420 | [Consumer Groups](guides/420-consumer-groups.md) | built | 106eaec |
| 430 | [Shock Absorber](guides/430-shock-absorber.md) | built | fa158c3 |
| 440 | [Partitioning](guides/440-partitioning.md) | built | 54eac9b |
| 510 | [Event Bridge](guides/510-event-bridge.md) | built | 9f8ec14 |
| 520 | Event Mesh | brief, [docs/designs/event-mesh.md](docs/designs/event-mesh.md) | f1c7c76 |
| 530 | [Event Gateway](guides/530-event-gateway.md) | built | f3ede54 |
| 610 | [Dead Letter Queue (built as Dead Message Queue)](guides/610-dead-letter-queue.md) | built | 558aa17 |
| 620 | [Discard, Pause and Retry](guides/620-discard-pause-retry.md) | built | 7f1301c |
| 630 | [Saga](guides/630-saga.md) | built | 40b7907 |
| 710 | [Event Catalog](guides/710-event-catalog.md) | built | afbbc8a |
| 720 | [Event APIs](guides/720-event-apis.md) | built | f727057 |
| 730 | [Intermediated Governance](guides/730-intermediated-governance.md) | built | 02a759f |
| 740 | [Disintermediated Governance](guides/740-disintermediated-governance.md) | built | 1f527a3 |
| 750 | [Access Control and Authorization](guides/750-access-control.md) | built | 83b5d99 |
| 810 | [Strangler](guides/810-strangler.md) | built | 5a78129 |
| 820 | [Data Synchronization](guides/820-data-synchronization.md) | built | bfc1dec |

## What you get in the container

| Thing | Where |
| --- | --- |
| Solace Workshop Dashboard | Port 3000 (the **Ports** tab in a Codespace, <http://localhost:3000> locally) |
| Solace Broker Manager | Port 8080, `admin` / `admin`, or the link in the dashboard sidebar |
| Broker messaging (SMF) | `localhost:55555` |
| Broker MQTT and REST (Event Gateway) | `localhost:1883` and `localhost:9000` |
| Solace Broker | Docker container `solace_10.26.0`, Standard edition |

Also installed: Go with a C toolchain (the Solace Go API wraps the native
client library), Terraform with the Solace broker provider, Python for the
dashboard, and the Solace Try-Me VS Code extension for poking at topics by
hand.

## How it fits together

```
Browser (port 3000)
     |
FastAPI dashboard ── scenarios/<id>/scenario.yaml   what the scenario does
     |               scenarios/<id>/tf/*.tf         broker configuration
     |               apps/                          one Go binary, all roles
     v
terraform + Go apps  ──>  Solace broker
```

- **`cockpit/`** is the dashboard. Scenarios are data: a folder with a
  `scenario.yaml`, terraform for its broker configuration, and a diagram
  description. Adding a scenario means adding a folder.
- **`cockpit/apps/`** is one Go module producing one binary, invoked as
  `workshop <scenario> <role>`. The Solace Go API links the native client
  through cgo, so one binary keeps the build to a single link step.
- **`guides/`** is the written material, one file per scenario.

`cockpit/README.md` has the details, including how to add a scenario of your
own.

## Running an app by hand

The dashboard runs commands you can also run yourself:

```bash
bash cockpit/apps/run.sh exclusive ledger --role ledger-c --client-name excl-ledger-c
```

Connection details come from the environment, with defaults matching
`setup_broker.sh`, so this reaches the same broker the dashboard uses.

## Developing without a Codespace

There are two ways to build and test scenarios on your own machine.

1. **Local devcontainer.** Open the repo in VS Code and choose **Reopen in
   Container**. It starts its own broker in Docker, exactly as a Codespace
   does.
2. **Dashboard on your machine, broker in Docker.** You need Docker, Go
   with a C toolchain (on macOS, `xcode-select --install`) and Python 3.
   `./dev.sh` does the rest, including fetching a pinned Terraform into
   `.tools/` if you do not have one:

   ```bash
   cp .env.dev.example .env.dev   # once; edit it if 8080 or 55555 is taken
   ./dev.sh start                 # broker (if needed) and dashboard
   ./dev.sh status                # what is running, and where
   ./dev.sh logs                  # follow the dashboard log
   ./dev.sh restart               # restart the dashboard
   ./dev.sh stop                  # stop the dashboard and its apps
   ./dev.sh stop --broker         # ...and the broker container
   ./dev.sh check exclusive-consumer   # checker, then the smoke test
   ```

   It runs the dashboard in development mode, so edits show up without
   restarting (see below). Every part (dashboard, terraform, Go apps, setup
   script) reads the broker from the same variables, which you can set in
   `.env.dev`:

   | Variable | Default |
   | --- | --- |
   | `SOLACE_HOST` | `localhost` |
   | `SOLACE_SEMP_PORT` | `8080` |
   | `SOLACE_SMF_PORT` | `55555` |
   | `SOLACE_SEMP_USER` | `admin` |
   | `SOLACE_SEMP_PASSWORD` | `admin` |
   | `SOLACE_MSG_VPN` | `default` |
   | `SOLACE_CLIENT_PASSWORD` | `solace-workshop` |

   If something on your machine already uses 8080 or 55555, set other ports
   in `.env.dev`, for example `SOLACE_SEMP_PORT=18080`,
   `SOLACE_SMF_PORT=15555` and `SOLACE_MAP_EXTRA_PORTS=false` (which leaves
   out the MQTT, REST, AMQP and web messaging ports the dashboard does not
   use). Broker Manager is then on the SEMP port you chose.

   What reloads on its own:

   | You change | What happens |
   | --- | --- |
   | A `scenario.yaml` | The running dashboard picks it up on its next request. Running apps keep running. In development mode the open page reloads too. |
   | `cockpit/static/` (JS, CSS) | Served uncached. In development mode the open page reloads; otherwise reload it yourself. |
   | `cockpit/app/` (Python) | In development mode the server restarts by itself. That stops every app it was running. Otherwise run `./dev.sh restart`. |
   | Go code under `cockpit/apps/` | Rebuilt the next time an app starts. Stop and start the app (or press Play) to run the new code. |
   | Terraform under `tf/` | Applied the next time Play or Apply configuration runs. |

   Development mode is `COCKPIT_DEV=1`, which `./dev.sh` sets. A Codespace runs
   without it, so an attendee's page never reloads under them.

Notes for any broker other than the one `setup_broker.sh` starts:

- **Clear broker config** deletes every queue, client profile, ACL profile
  and client username that any scenario lists in its `tf/imports.tsv`. Only
  use it on a broker that can be cleaned.
- The apps connect without TLS, so a broker that only offers TLS ports does
  not work as is.
- If the dashboard runs in a container and the broker on the host,
  `localhost` does not reach the broker. Use `host.docker.internal`.
- Terraform needs SEMP rights to create queues, ACL profiles and client
  usernames.
- A scenario is not done until it passes on a Standard edition broker of the
  pinned version, like the Codespace's. Other versions behave differently
  (see [docs/spikes.md](docs/spikes.md)).

To check every scenario still lines up after an edit, and to drive one
through Play, each Break it and Reset, and Cleanup:

```bash
python3 cockpit/scripts/check_scenarios.py
${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py exclusive-consumer
```

## Repository layout

```
cockpit/           the Solace Workshop Dashboard
  apps/            Go applications, one module and one binary
  scenarios/       one folder per scenario: scenario.yaml + tf/
  scripts/         checker, smoke test, reconcile and failure helpers
docs/              requirements and spike findings
guides/            the written workshop
setup_broker.sh    starts the broker in Docker
dev.sh             start, stop and restart everything on your own machine
.env.dev.example   settings for dev.sh; copy to .env.dev
```

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) explains how the setup works behind the
scenes, how scenarios are defined and run, and how to add one.
[CLAUDE.md](CLAUDE.md) gives Claude Code the same map, so it can build new
scenarios for you.
