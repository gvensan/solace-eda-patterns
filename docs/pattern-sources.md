# Where each "On Solace" line comes from

Every scenario's `pattern:` block has a `solace:` line, shown as **On
Solace** in the pattern panel. Each must trace to a spike (`docs/spikes.md`)
or a docs.solace.com page whose title was checked (the docs site answers 200
even for a missing page, so a status code proves nothing). Lines that state
a design choice rather than a broker feature cite the feature they use.

Checked 2026-10-18.

| Scenario | What the line claims | Source |
| --- | --- | --- |
| 110 Event-Carried State Transfer | Guaranteed events; a queue per consumer keeps order and holds changes while it is away | [Guaranteed Messages](https://docs.solace.com/Messaging/Guaranteed-Msg/Guaranteed-Messages.htm), [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm); spikes 12 |
| 120 CQRS | Commands on a queue, events on topics with a queue per read side, queries by request-reply | [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm), [Request-Reply in the Go API](https://docs.solace.com/API/API-Developer-Guide-Go/Go-API-Request-Reply.htm) |
| 130 Change Data Capture | A connector publishes each change as a guaranteed event; Solace Micro-Integrations include a CDC source (Oracle CDC, built on Debezium) | [Oracle CDC Micro-Integration](https://docs.solace.com/Micro-Integrations/Managed/mi-details/oraclecdc.htm), [Guaranteed Messages](https://docs.solace.com/Messaging/Guaranteed-Msg/Guaranteed-Messages.htm) |
| 140 Event Sourcing | Replay log of fixed size, replayed into an exclusive queue on request; a recovery buffer | spikes 17; [Message Replay](https://docs.solace.com/Features/Replay/Msg-Replay-Concepts-Config.htm), [Replaying from the Replay Log](https://docs.solace.com/API/API-Developer-Guide/Replay-from-Replay-Log.htm) |
| 210 Publish-Subscribe | Topics with wildcard subscriptions; direct messages, or queues subscribed to topics | [Wildcard Characters in Topic Subscriptions](https://docs.solace.com/Messaging/Wildcard-Charaters-Topic-Subs.htm), [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm) |
| 220 Point-to-Point | A durable queue addressed by its name keeps each message until taken | spikes 9 (`#P2P/QUE/<queue>`); [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm) |
| 230 Request-Reply | Two direct messages; reply on a topic made for the asker; Go request-reply is direct | spikes 15; [Request-Reply in the Go API](https://docs.solace.com/API/API-Developer-Guide-Go/Go-API-Request-Reply.htm) |
| 240 Event Streaming | A queue per consumer keeps order; acknowledgements are the position; a replay log can feed history | spikes 17; [Consuming Persistent Messages (Go)](https://docs.solace.com/API/API-Developer-Guide-Go/Go-PM-Subscribe.htm) |
| 310 Hierarchical Topics | Levels separated by /, * for one level, > for the rest | [Wildcard Characters in Topic Subscriptions](https://docs.solace.com/Messaging/Wildcard-Charaters-Topic-Subs.htm) |
| 320 Event Filtering | Selectors on a queue binding; partitioned queues refuse them; unmatched messages stay | spikes 2 and 18; [Using Selectors](https://docs.solace.com/API/API-Developer-Guide/Using-Selectors.htm) |
| 330 Guaranteed Delivery | Broker confirms each publish once stored; redelivers what is not acknowledged | spikes 12; [Guaranteed Messages](https://docs.solace.com/Messaging/Guaranteed-Msg/Guaranteed-Messages.htm) |
| 340 Backpressure and Push | Unacknowledged limit per consumer; Go Pause and Resume; a direct subscriber drops when its buffer fills | spikes 13; [Consuming Persistent Messages (Go)](https://docs.solace.com/API/API-Developer-Guide-Go/Go-PM-Subscribe.htm) |
| 350 Exclusive Consumer | First bound consumer active, others standby; failover when its session ends | spikes 8; [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm) |
| 410 Competing Consumers | A non-exclusive queue shares its messages among bound consumers | [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm); spikes 9 |
| 420 Consumer Groups | A queue per group; members bind it, partitioned for order per key | spikes 14; [Message Distribution with Partitioned Queues](https://docs.solace.com/Messaging/Guaranteed-Msg/Partitioned-Queue-Messaging.htm) |
| 430 Shock Absorber | A queue with a spool quota; the publisher is told when it is full | spikes 16 and 18 (`Spool Over Quota` on the publisher) |
| 440 Partitioning | The broker hashes the partition key to a partition, one consumer per partition | spikes 10; [Message Distribution with Partitioned Queues](https://docs.solace.com/Messaging/Guaranteed-Msg/Partitioned-Queue-Messaging.htm) |
| 510 Event Bridge | A message VPN bridge between VPNs on one or two brokers; guaranteed messages through a queue on the far side | spikes 1 and 19; [Message VPN Bridges](https://docs.solace.com/Features/VPN/Message-VPN-Bridges-Overview.htm) |
| 610 Dead Message Queue | A DMQ per queue for redelivery limit, time to live, or rejection | spikes 6 and 11; [Configuring Dead Message Queues](https://docs.solace.com/Messaging/Guaranteed-Msg/Setting-Dead-Msg-Queues.htm) |
| 620 Discard, Pause and Retry | Reject to the DMQ, republish to a delayed queue, Go Pause and Resume | spikes 5; [Delayed Delivery](https://docs.solace.com/Messaging/Guaranteed-Msg/Delayed-Delivery.htm), [Consuming Persistent Messages (Go)](https://docs.solace.com/API/API-Developer-Guide-Go/Go-PM-Subscribe.htm) |
| 630 Saga | A queue per service subscribed to the events it reacts to | [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm) |
| 750 Access Control and Authorization | ACL profiles with defaults and topic exceptions, assigned to client usernames | spikes 15; [Client Authorization](https://docs.solace.com/Security/Client-Authorization-Overview.htm) |
| 810 Strangler | Queue subscriptions decide who gets what; a cut-over is a configuration change | [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm); the scenario's `route.sh` over SEMP |
| 820 Data Synchronization | Topics name the origin; each side's queue subscribes to the other side only | [Wildcard Characters in Topic Subscriptions](https://docs.solace.com/Messaging/Wildcard-Charaters-Topic-Subs.htm), [Queues and Topic Endpoints](https://docs.solace.com/Messaging/Guaranteed-Msg/Endpoints.htm) |

No line needed removing. One was extended: Change Data Capture now names
the Oracle CDC Micro-Integration, confirmed on its docs page.
