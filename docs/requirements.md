# Event-Driven Architecture Patterns Workshop: Requirements

Status: agreed after the Phase 0 review, updated with the spike findings
Date: 2026-10-16 (Phase 0.5)
Owner: Giri Venkatesan (Solace)

## 1. Goal

Build a new, hands-on workshop in which each event-driven architecture pattern is a small, working system that an attendee can run, break and inspect on a real Solace broker. The experience should match what the Solace Core Developer Workshop offers: open a Codespace, press Play, watch the pattern work, cause a failure on purpose, and see the real error and the recovery.

The workshop covers the patterns in the Solace guide to event-driven architecture patterns. Each pattern gets its own scenario, so the lesson stays focused on one idea at a time.

### Success criteria

- An attendee with no prior setup can reach a running scenario within minutes of opening the Codespace.
- Every pattern in scope has one scenario that demonstrates its main benefit and at least one way it fails.
- Failures show the real error from the Solace API or the broker, with the explanation kept in the surrounding text.
- Any scenario can be run, cleaned up and run again from an empty broker.
- A new scenario can be added by someone who has not built one before, using a written guide, and ideally with help from Claude Code.

### Non-goals

- This is not a replacement for the Core Developer Workshop. It does not re-teach broker basics beyond what each pattern needs.
- It does not target production deployment guidance. The Solace Standard edition used here is not production deployable.
- It does not cover patterns outside the reference list unless added later by decision.

## 2. Reference

### 2.1 Pattern catalogue

The pattern list comes from the Solace Ultimate Guide to Event-Driven Architecture Patterns (https://solace.com/event-driven-architecture-patterns/). It names 30 patterns in 8 categories. The guide itself notes that expert lists vary and that there is no single definition of a pattern. It is thin on broker-specific design for many patterns, so each scenario must be checked against Solace documentation and not against the guide alone.

### 2.2 Prior art

The Solace Core Developer Workshop (https://github.com/SolaceDev/solace-core-developer-workshops) is the model for form and tooling. Its relevant properties are listed here as observed in the repository.

- It runs in GitHub Codespaces or a local devcontainer from a prebuilt image, with a Solace broker 10.8.1 in Docker.
- A FastAPI dashboard on port 3000 drives everything. It has no frontend build step.
- Each section is a folder under `cockpit/scenarios/<id>/` containing `scenario.yaml`, a `tf/` folder of terraform for the broker objects, and an `imports.tsv` listing everything the scenario owns.
- Applications are Go roles compiled into one binary and run as `workshop <scenario> <role>`.
- Each page has four parts: Run it (Play, Pause, Cleanup and a live diagram), Step through it, Break it, and On the broker.
- A checker script, `check_scenarios.py`, enforces structural rules.
- `CLAUDE.md` and `CONTRIBUTING.md` document how to add scenarios, so Claude Code can build them.

The core workshop already covers, in some form, pub/sub, hierarchical topics, streaming, shock absorber, competing consumers, partitioning and the dead message queue. It also has a fan-out and a processor section that are not in the reference list. A `cqrs` Go package is registered in its binary but has no scenario, which may be reusable.

### 2.3 Solace documentation

Facts about edition limits, the Go API and broker features are taken from docs.solace.com. The sources are listed in section 7.

## 3. Form

### 3.1 Deliverable

A new workshop in a new repository, separate from the core workshop. It is delivered as a Codespace that opens a dashboard with one sidebar entry per pattern scenario, plus a written guide for each.

### 3.2 Starting point

The dashboard, process manager, SEMP proxy, diagram renderer and Go helper packages start as a copy of the core workshop's `cockpit/`. Whether to later extract this as a shared base is an open decision (section 6). Until then, fixes to shared code may need to be made in both places.

### 3.3 Attendee experience

- One scenario per pattern, each with the same four-part page as the core workshop.
- Each scenario is independent. Attendees can do them in any order, and Cleanup on one never affects another.
- Attendee-facing text uses plain sentences written for someone new to event-driven design.
- Each guide ends with a link to the next.

### 3.4 Repository documents

- `docs/requirements.md` (this file).
- A `CLAUDE.md` and `CONTRIBUTING.md` adapted from the core workshop, so scenarios can be added with Claude Code.
- A README with the Codespaces badge, a table of scenarios and the container contents.

### 3.5 Codespace parity

The end goal is that an attendee exercises every scenario in a GitHub Codespace, with the same structure, function, flow and usage as the core workshop.

- The README carries a Codespaces badge. Opening it runs the same devcontainer hooks: build the Go binary, start the broker in Docker, start the dashboard on port 3000, and open it in a browser tab. Ports 3000 (dashboard) and 8080 (Solace Broker Manager) are forwarded.
- Every page keeps the same four parts, and the same controls: Play, Pause, Cleanup, Stop everything and Clear broker config.
- Every startup script is safe to run again, so a stopped Codespace resumes cleanly.
- The core workshop starts quickly because it uses a prebuilt image, `ghcr.io/solacedev/solace-core-workshop`, built in a separate infrastructure repository. This workshop reuses that image for now, pinned by digest in `.devcontainer/devcontainer.json` rather than `:latest`, so a rebuild of the core image cannot change this workshop unannounced. It works while the Go module and terraform provider versions match; if they differ, a Codespace installs the difference on startup, which is slower. Ownership and rebuilds are described in `CONTRIBUTING.md`.
- The core workshop's Codespace registration script (which sent the Codespace's IP address and GitHub username to a Solace endpoint) and its script that made forwarded ports public are not carried over. If registration is ever added, attendees must be told what is collected before it runs.
- The core workshop asks for 4 CPUs, 16 GB memory and 32 GB storage. Scenarios that run several brokers may need more. Any such scenario states its needs, and this has not been tested.
- Event mesh needs the broker's connection scale raised and the broker restarted. That has to be scripted into the startup hooks.
- Deliberate differences from the core workshop are the sidebar, which grows to about 30 entries and may group by the guide's 8 categories, the extra pieces some patterns need (a second broker or VPN, a small state store, a policy-checking app), and automated smoke tests.

### 3.6 Local development

Developers must be able to build and test scenarios against a local broker before using a Codespace. This follows from how the core workshop is configured, and has not been run against a local broker yet.

- The dashboard, terraform and Go apps read their broker details from environment variables, with defaults that match the broker the setup script starts. The variables are `SOLACE_HOST`, `SOLACE_SEMP_PORT`, `SOLACE_SMF_PORT`, `SOLACE_SEMP_USER`, `SOLACE_SEMP_PASSWORD`, `SOLACE_MSG_VPN` and `SOLACE_CLIENT_PASSWORD`. Pointing them at another broker redirects everything.
- Two routes are supported. The first is a local devcontainer in VS Code, which starts its own broker in Docker, as a Codespace does. The second is running the dashboard on the developer's machine against a local broker, which needs Docker, Go with a C toolchain (the Solace Go API uses cgo) and Python. `./dev.sh` starts, stops and restarts the broker and dashboard for this route, reads per-machine settings from `.env.dev`, fetches a pinned terraform if none is installed, and runs the dashboard in development mode (edits to scenarios and static files show up without a restart). Both routes were verified on macOS in Phase 0; the dashboard reads `SOLACE_*` throughout.
- The defaults are `admin` / `admin` and the `default` message VPN. A different broker needs the variables set, and terraform needs SEMP rights to create queues, ACL profiles and client usernames.
- If the dashboard runs in a container and the broker on the host, `localhost` does not reach the broker. Use `host.docker.internal` or the host's address.
- Clear broker config deletes every queue, client profile, ACL profile and client username listed in any scenario's `imports.tsv`. It should only be used on a broker that can be cleaned. Scenario objects carry unique prefixes, so unrelated objects are left alone.
- The apps connect without TLS, so a local broker with only TLS ports does not work as is.
- A local broker of a different version or edition can behave differently from the Codespace broker. Examples are the message VPN limit, the connection scale for event mesh, and dead message queue handling. A scenario is not done until it passes on a Standard-edition broker like the Codespace's.

## 4. Shape

### 4.1 Scenario structure

Each scenario follows the core workshop's anatomy.

- A `scenario.yaml` with page text, actions, a diagram, failure modes and inspect views.
- A `tf/` folder with terraform for the broker objects it owns, and an `imports.tsv`.
- A Go package with its roles, registered in the single workshop binary.
- A guide in `guides/` that explains what to watch, matches the Break it cards, and lists things to try.

Rules carried over from the core workshop.

- One broker per diagram, and every flow goes through it. Apps never connect to each other. The one planned exception is the multi-broker deployment patterns, which need a deliberate change to the checker.
- Scenarios never share broker objects. Every object name carries a prefix unique to the scenario.
- `imports.tsv` lists every object a scenario owns.
- `run:` starts consumers before publishers.
- Every scenario offers apply, build, reconcile and destroy actions.
- Apps print the Go API's error as it is. Explanations belong in the card text.
- Credentials stay local to the container.

Rules added in Phase 0. `CLAUDE.md` holds the working version of each.

- **Topics carry the scenario prefix too** (`excl/...`), so no other scenario's queue subscription picks them up.
- **Role-based where instances are interchangeable, a named app where there is only one.** Where several instances of one app share a job and the broker decides who does what (an active consumer and a standby, workers on a shared queue, and the like), a failure card acts on and describes the role ("the active ledger crashes", "a worker hangs"), never a named instance. Its trigger targets whoever holds the role when Break it is pressed, and the page marks and shows the logs of the apps in those roles; where several apps hold a role, the one with the lowest client name is taken. Apps are never rearranged behind the attendee's back to make a name true. Where a scenario has only one app in a part (the Point-to-Point sender, the Dead Message Queue intake), the card names that app.
- **A crash is a kill.** Stopping an app is a clean shutdown that finishes in-flight work; a card that shows a crash kills the process.
- **Every indicator explains itself.** Each dot, count, badge, border and label on the page has hover text and an entry in `guides/000-reading-the-dashboard.md`.
- **The page never shows work that is not happening.** Labels and animations follow the broker's data (for example "active, not acking" for a frozen consumer), not assumptions.
- **Each failure mode states its expected outcome** in an `expect:` block (log lines that must and must not appear after Break it and after Reset), which the smoke test enforces.
- **Numbering.** `order:` is the pattern guide's category number times 100 plus the pattern's position in that category times 10 (Exclusive Consumer, category 3, fifth pattern: 350). The guide file is `guides/<order>-<id>.md`, and `eyebrow:` is the category name, which the sidebar groups by.

### 4.2 Scope and sequencing

Each of the 30 patterns gets its own scenario. The proposed order below is based on what the documentation check showed about risk. It is a proposal and can be changed.

| Wave | Basis | Patterns |
| --- | --- | --- |
| 1 | Documented support, existing framework | Publish-Subscribe, Point-to-Point, Request-Reply, Event Streaming, Hierarchical Topics, Guaranteed Delivery, Backpressure and Push, Exclusive Consumer, Competing Consumers, Consumer Groups, Shock Absorber, Partitioning, Dead Message Queue (the guide's Dead Letter Queue), Access Control and Authorization |
| 2 | Needs application logic or a short spike first | Event Carried State Transfer, CQRS, Change Data Capture, Event Sourcing, Event Filtering, Discard Pause and Retry, Saga, Strangler, Data Synchronization, Event Catalog, Event APIs, Intermediated Governance, Disintermediated Governance, Event Bridge, Event Gateway |
| 3 | Needs more than one broker | Event Mesh |

Event Bridge and Event Gateway moved from wave 3 to wave 2 after the Phase 0 spikes showed both work on a single Standard broker (`docs/spikes.md`, sections 1 and 7). Event Mesh stays in wave 3.

Where two patterns overlap, such as Competing Consumers and Consumer Groups, each keeps its own scenario. The guide for each must state the single idea that separates it from its neighbour, and shared code is reused.

### 4.3 Scenario notes that follow from the documentation check

- **Request-Reply** runs on direct messaging, because the Go API supports it for direct messages only. The scenario must not imply guaranteed delivery.
- **Event Filtering** leads with topic wildcards. A selector exercise is a second step, shown with its cost. Solace advises avoiding selectors where topics will do, and partitioned queues do not support them. Verified from Go: a selector on a queue works, messages it does not match stay on the queue and use its quota, and a partitioned queue refuses the bind (`docs/spikes.md`, section 2).
- **Point-to-Point** addresses the queue by name. The Go API publishes to topics only, but every queue has a network topic, `#P2P/QUE/<queue>`, and a guaranteed message published to it is delivered to that queue, which needs no topic subscription. With the client profile's `reject_msg_to_sender_on_no_subscription_match_enabled`, a message to a queue that does not exist is refused with `No Subscription Match` instead of being dropped silently (`docs/spikes.md`, section 9). Built in batch 1.
- **Exclusive Consumer** uses an exclusive queue with a standby consumer and the Go receiver's activation and passivation listener. Built as the Phase 0 pilot.
- **Competing Consumers** runs with a deliberate small backlog: checkout publishes slightly faster than two workers send, so each worker always holds its limit of unacknowledged jobs and the crash card's redelivery is certain, and starting a third worker visibly drains the queue. Its guide keeps the lesson on adding workers, separate from **Shock Absorber**, whose lesson is the queue soaking up a burst. Built in batch 1.
- **Partitioning** and **Consumer Groups**: consumers beyond a queue's partition count sit idle (measured: 2 partitions, 3 consumers, the third received nothing; `docs/spikes.md`, section 9). Any scenario with partitioned queues sizes its partition count to at least its consumer count. Partitioning built in batch 1.
- **Discard, Pause and Retry** cannot use the broker's delayed redelivery from Go: the queue accepts the settings, but a Go consumer's bind is refused (`Redelivery delay support required`), because the feature needs local transactions, which the Go API does not have (`docs/spikes.md`, section 5). The design is a retry queue with `delivery_delay`, to which the app republishes a message it could not process; `delivery_delay` itself was verified from Go. This design gets a short spike of its own before the scenario is built.
- **Dead Message Queue** must be written for the pinned broker. On 10.26.0, every message a queue removes (redelivery limit, REJECTED, TTL) goes to its dead message queue by default, whatever the message's DMQ-eligible flag; the flag only counts on a queue with `respect_dmq_eligible_enabled`. On 10.8.1 the flag decides. The Go API marks messages eligible unless told otherwise (`docs/spikes.md`, section 6). The scenario's terraform sets `respect_dmq_eligible_enabled`, an attribute 10.8.1 does not have, so the scenario applies only to 10.26.0 or later: a hard dependency on the pinned version. Built in batch 1.
- **Event Sourcing** uses the replay log and an exclusive queue, because client-initiated replay is not supported on non-exclusive endpoints. The scenario must state that a replay log is bounded by size and is not a queryable event store.
- **Backpressure** uses the broker's limit on unacknowledged messages per flow, plus the Go receiver's pause and resume.
- **Event Bridge** can use a loopback bridge between two message VPNs on one broker. Verified: the remote VPN's client profile must allow bridge connections, and the bridge uses a client connection (`docs/spikes.md`, section 1).
- **Event Gateway** can take MQTT and REST into a queue and deliver out through a REST delivery point, all on one broker (`docs/spikes.md`, section 7).
- **Event Mesh** needs the broker's connection scale raised above 100 and more than one broker. It is the highest risk item. Each broker at 1,000 connections uses about 4.15 GiB of memory whether or not clients connect, so two fit a 16 GB Codespace on paper and three are tight; this still needs a Codespace test (`docs/spikes.md`, sections 3 and 4).

### 4.4 Go API limits found while building

The workshop's apps use the Solace Go API (v1.10.0). It lacks some things
other Solace APIs have, and scenarios are designed around them rather than
assuming them.

- **No queue browser.** An app cannot look at a queue's messages without
  taking them. The Dead Message Queue scenario's watcher reads without
  acknowledging instead, so parked messages stay on the queue. This affects
  the governance scenarios too.
- **No delivery count on a received message.** An app cannot ask how many
  times a message has been delivered; it counts attempts itself.
- **No DMQ-eligible flag on a received message.** The flag can be set when
  publishing but not read on receipt; the broker's monitor tree shows it per
  message (`dmqEligibleAsPublished`).
- **No local transactions**, so no broker-side delayed redelivery (section
  4.3, Discard, Pause and Retry).
- **Publishing to topics only**; queues are addressed through their
  `#P2P/QUE/<queue>` network topic.

These also bear on Event Sourcing, where replay is the API's only way to
re-read history.

## 5. Requirements

### 5.1 Functional

1. Each scenario can be started with one Play action that applies configuration, builds apps if needed, starts consumers, then starts publishers.
2. Each scenario has Pause and Cleanup. Cleanup stops processes and removes the scenario's broker configuration.
3. Each scenario has at least one failure mode with Break it and Reset, showing the real error and the recovery.
4. Each scenario has inspect views that read the broker through SEMP and say where to find the same data in Solace Broker Manager.
5. The diagram shows live process state and a count of the broker objects each node depends on.
6. Stop everything and Clear broker config work across all scenarios.
7. A scenario can be applied from an empty broker, and again after Cleanup.
8. A checker validates the scenario files and fails on any broken reference or rule violation.

### 5.2 Non-functional

1. Startup is fast, using a prebuilt image with Go, terraform, the dashboard environment and warmed caches.
2. Setup scripts are safe to run more than once.
3. Resource needs stay within what a Codespace offers. The core workshop asks for 4 CPUs, 16 GB memory and 32 GB storage. Any scenario needing more must say so.
4. The dashboard stays free of a frontend build step, as in the core workshop.
5. Each wave ships with an automated smoke test that drives the dashboard API through Play, each failure mode and Cleanup. The core workshop checks this by hand, and thirty scenarios will need it automated. In place since Phase 0.5: `cockpit/scripts/smoke.py` drives every scenario, checks each failure mode's `expect:` block and exits non-zero on any failure, and a GitHub Actions workflow runs it on Linux against the pinned broker on every push and pull request.
6. Writing follows the project style, with no em dashes or en dashes, and the dashboard is called "Solace Workshop Dashboard" or "the dashboard" in attendee text.

### 5.3 Platform constraints from the Standard edition

The Standard edition table in Solace documentation lists limits for the broker image used in the workshop. Phase 0 measured them on `solace-pubsub-standard:10.26.0` (`docs/spikes.md`). Where the two disagree, both are kept here and the measurement is what scenarios are designed for.

| Limit | Documentation table | Measured on 10.26.0 |
| --- | --- | --- |
| Message VPNs | 1 user-configured VPN (a community thread reports 3 including `default`) | 3 including `default`. The fourth fails with `Message VPN limit reached., MAX_NUM_EXCEEDED`, at both 100 and 1,000 connections. |
| VPN bridges | Up to 500 | `show system` reports `max-bridges 25` at both 100 and 1,000 connections |
| Client connections | Up to 1,000, default scale 100 | Default 100. Raised to 1,000, 995 clients connected before `Too Many Clients`. |
| Dynamic Message Routing | Only above 100 connections | Refused at 100 (`not supported when the configured system limit for connections is 100`), accepted at 1,000 |
| Raising the scale | Can only be increased; the change restarts the broker | The container variable works only when the container is first created; on an existing broker the CLI change reloads it once |
| Ingress | 10,000 messages per second | Not measured |
| Message replay | Available | A replay log was created and enabled; client-initiated replay not yet tested |
| Production use | Not production deployable | Not applicable |

Bridges use client connections, which the measurement confirmed.

## 6. Open questions and decisions

1. **Repository and base.** Open. The repository is a copy of the core workshop's `cockpit/` for now. Whether to extract a shared base used by both workshops is still to decide; until then shared fixes may be needed in both places.
2. **Broker version.** Decided: `solace/solace-pubsub-standard:10.26.0`, the current LTS (Docker Hub's `lts` and `latest` tags point at it on 2026-10-16), pinned in `setup_broker.sh` and the CI workflow. Docker Hub has `10.8.1`, `10.25.0`, `10.26.0` and `10.26.8` (`rolling`); there is no `10.25.10` image. Dead message queue behaviour differs from the core workshop's 10.8.1 (section 4.3).
3. **Wave order.** Decided: Event Bridge and Event Gateway move to wave 2; Event Mesh stays in wave 3 (section 4.2).
4. **Event Portal.** Solace Event Portal is a hosted product and needs an account, which conflicts with the "no account" promise. Governance scenarios would use local files and open tools. Is that acceptable, with Event Portal as a linked follow-on?
5. **Audience and length.** Who attends, and how long is each scenario meant to take? This affects how much each guide includes.
6. **Devcontainer image.** Open. Keep using the core workshop's image (pinned by digest), or build one for this workshop, warmed from its own repository?

## 7. Verification

These items could not be confirmed from the documentation. Each was tested hands-on in Phase 0 on a Standard edition broker; `docs/spikes.md` has what was run and what was seen.

| Item | Status |
| --- | --- |
| Which terraform provider resources exist for replay logs, bridges and DMR clusters | Resolved. All exist in provider 1.3.1, and each applied on the broker (DMR only above 100 connections). There is no resource for the connection scale. [Spikes, section 1](spikes.md#1-terraform-provider-resources) |
| Whether Go message selectors behave as expected on a queue | Resolved. They do; unmatched messages stay on the queue; partitioned queues refuse them. [Spikes, section 2](spikes.md#2-go-message-selectors-on-a-queue) |
| How to raise the connection scale on a Docker broker | Resolved. Container variable on first creation, CLI on an existing broker. [Spikes, section 3](spikes.md#3-raising-the-connection-scale-in-docker) |
| CPU and memory for a broker at 1,000 connections, and whether two or three fit in a Codespace | Resolved for one broker (about 4.15 GiB, under 10% CPU idle). Fitting two or three in a Codespace is estimated, and needs a Codespace run before Event Mesh is scoped. [Spikes, section 4](spikes.md#4-resource-use-at-1000-connections) |
| Event Gateway design: REST delivery points and MQTT ingress | Resolved. Both work on one broker. [Spikes, section 7](spikes.md#7-event-gateway-inputs) |
| Delayed redelivery on queues | Resolved: not usable from the Go API. Discard, Pause and Retry uses a retry queue with `delivery_delay` instead (section 4.3). [Spikes, section 5](spikes.md#5-delayed-redelivery-on-queues) |
| The effect of the broker version on dead message queue behaviour | Resolved. 10.8.1 follows the message's DMQ-eligible flag; 10.26.0 sends every removed message to the DMQ unless the queue respects the flag. [Spikes, section 6](spikes.md#6-dead-message-queue-behaviour-by-version) |

## 8. Sources

- Solace guide to event-driven architecture patterns: https://solace.com/event-driven-architecture-patterns/
- Core workshop repository: https://github.com/SolaceDev/solace-core-developer-workshops
- Editions of the Software Event Broker: https://docs.solace.com/Software-Broker/SW-Broker-Set-Up/Setting-Up-SW-Brokers.htm
- System scaling parameters: https://docs.solace.com/Software-Broker/System-Scaling-Parameters.htm
- Scaling an existing single broker: https://docs.solace.com/Software-Broker/Set-Scaling-Params-Standalone.htm
- Community thread on maximum message VPNs: https://community.solace.com/t/pubsub-maximum-message-vpns/821
- Community thread on raising the connection count in Docker: https://community.solace.com/t/docker-raising-system-scaling-maxconnectioncount/4547
- Go API persistent receiver reference: https://docs.solace.com/API-Developer-Online-Ref-Documentation/go/src/solace.dev/go/messaging/pkg/solace/persistent_message_receiver.go.html
- Go API config package: https://docs.solace.com/API-Developer-Online-Ref-Documentation/go/solace.dev/go/messaging/pkg/solace/config
- Go API, consuming persistent messages: https://docs.solace.com/API/API-Developer-Guide-Go/Go-PM-Subscribe.htm
- Go API, request-reply: https://docs.solace.com/API/API-Developer-Guide-Go/Go-API-Request-Reply.htm
- Using selectors: https://docs.solace.com/API/API-Developer-Guide/Using-Selectors.htm
- Queue features for messaging patterns: https://docs.solace.com/Messaging/Guaranteed-Msg/Queues.htm
- Configuring message replay: https://docs.solace.com/Features/Replay/Msg-Replay-Config.htm
- Replay and cache comparison: https://docs.solace.com/Features/Replay/Replay-Cache-Compare.htm
- Replaying from the replay log: https://docs.solace.com/API/API-Developer-Guide/Replay-from-Replay-Log.htm
- Configuring VPN bridges: https://docs.solace.com/Features/VPN/Configuring-VPN-Bridges.htm
- DMR overview: https://docs.solace.com/Features/DMR/DMR-Overview.htm
- Configuring a DMR cluster: https://docs.solace.com/Features/DMR/DMR-Mgmt-Cluster-Config.htm
- Terraform provider README: https://raw.githubusercontent.com/SolaceProducts/terraform-provider-solacebroker/master/README.md
- Phase 0 spike findings: [docs/spikes.md](spikes.md)
- Delayed redelivery in the JCSMP API: https://docs.solace.com/API/API-Developer-Guide-JCSMP/JCSMP-API-Delayed-Redelivery.htm
- Configuring dead message queues: https://docs.solace.com/Messaging/Guaranteed-Msg/Setting-Dead-Msg-Queues.htm
- Configuring queues: https://docs.solace.com/Messaging/Guaranteed-Msg/Configuring-Queues.htm
