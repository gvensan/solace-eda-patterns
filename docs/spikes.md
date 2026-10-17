# Phase 0 spikes

Date: 2026-10-16

Short hands-on tests of the items in section 7 of `requirements.md`. Each
section says what was run and what the broker or API returned. Nothing here
is taken from the pattern guide.

## Test environment

This is a developer laptop, not a Codespace. Results that depend on the host
(memory, CPU) need repeating in a Codespace before they are relied on.

| Thing | Version |
| --- | --- |
| Host | macOS, Apple silicon, Docker Desktop (16 CPUs, 7.65 GiB for the Docker VM) |
| Broker, main | `solace/solace-pubsub-standard:10.26.0` (SEMP reports `10.26.0.9054`). On Docker Hub, `lts` and `latest` point at this image. |
| Broker, comparison | `solace/solace-pubsub-standard:10.8.1` (the core workshop's pin, SEMP API `2.41`) |
| Terraform | 1.16.6 |
| Solace terraform provider | `solaceproducts/solacebroker` 1.3.1 (latest, `~> 1.3`) |
| Solace Go API | `solace.dev/go/messaging` v1.10.0 (the core workshop's pin). v1.10.1 tried where noted. |

Tags available on Docker Hub on the test date: `10.8.1` (`10.8.1.594`),
`10.25.0` (`10.25.0.391`), `10.26.0` (`10.26.0.9054`, also `lts` and
`latest`), `10.26.8` (`10.26.8.13158`, also `rolling` and `edge`). There is
no `10.25.10` tag.

Probes used a throwaway Go tool built on the same Go API, a small SEMP
helper, and `curl`. None of it is in the repo.

## 1. Terraform provider resources

**Ran:** `terraform providers schema -json` against provider 1.3.1, then
`terraform apply` of each resource family on the 10.26.0 broker.

**Schema:** 78 resources, 81 data sources. The relevant ones exist:

| Need | Resources |
| --- | --- |
| Replay log | `solacebroker_msg_vpn_replay_log`, `solacebroker_msg_vpn_replay_log_topic_filter_subscription` |
| VPN bridge | `solacebroker_msg_vpn_bridge`, `_bridge_remote_msg_vpn`, `_bridge_remote_subscription` |
| DMR | `solacebroker_dmr_cluster`, `_dmr_cluster_link`, `_dmr_cluster_link_remote_address`, `_dmr_cluster_link_attribute`, `_dmr_cluster_cert_matching_rule*`, `solacebroker_msg_vpn_dmr_bridge` |
| Message VPN | `solacebroker_msg_vpn` |
| Event Gateway | `solacebroker_msg_vpn_rest_delivery_point` (+ `_queue_binding`, `_rest_consumer`), `solacebroker_msg_vpn_mqtt_session` |
| Queue features | `solacebroker_msg_vpn_queue` has `redelivery_delay_*`, `delivery_delay`, `respect_dmq_eligible_enabled`, `partition_count`, `dead_msg_queue`, `max_redelivery_count` |

There is no resource for system scaling (max connections). That is not a
message VPN setting, and it needs a broker reload (section 3).

**Apply results on 10.26.0, default scale (100 connections):**

- Replay log: created, `egressEnabled` and `ingressEnabled` true in the
  monitor tree. Client-initiated replay was not tested.
- Second message VPN: created. A third also worked. A fourth failed:
  `Message VPN limit reached., MAX_NUM_EXCEEDED`. So the limit is three VPNs
  in total including `default`, which matches the community thread and not
  "default plus one" in section 5.3.
- Loopback bridge from `default` to a second VPN on the same broker
  (`remote_msg_vpn_location = "127.0.0.1:55555"`): created. It first failed
  with `Login Response: Bridging Not Allowed`. Setting
  `allowBridgeConnectionsEnabled` on the remote VPN's client profile fixed
  it, and the bridge went to `ready-in-sync`. Three messages published in the
  second VPN reached a subscriber in `default`. The bridge shows as a client
  (`#bridge/remote/br-spike/...`) on the remote VPN, so it uses a connection.
  The provider requires `remote_authentication_basic_password` whenever a
  bridge username is set.
- DMR cluster: refused, `dynamic-message-routing is not supported when the
  configured system limit for connections is 100, FAIL`. At 1,000 connections
  the same call succeeded (section 3).
- `show system` reports `max-bridges 25` at both 100 and 1,000 connections,
  not the 500 in section 5.3.

**Lost state:** after deleting the pilot scenario's terraform state with its
objects still on the broker, `apply` failed, but afterwards five of the six
objects were already in the new state, and Reconcile only had to import the
queue subscription. The provider's error text shows it creates objects with a
SEMP PUT, which would explain the adoption. This differs from what the core
workshop's guide says (apply fails with `ALREADY_EXISTS` for everything).
Reconcile still works either way.

## 2. Go message selectors on a queue

**Ran:** published nine guaranteed messages with a user property
`region` = 1, 2, 0, 1, 2, 0, ... to an exclusive queue. Bound with
`WithMessageSelector("region = '1'")` and client acknowledgement.

**Saw:**

- The selector worked: 3 of 9 delivered (`msg-1`, `msg-4`, `msg-7`).
- The other 6 stayed on the queue (`msgSpoolUsage` 312 bytes). A second
  consumer bound with no selector received exactly those 6.
- A bad selector (`region = = '1'`) fails at bind:
  `error while creating receiver flow: Invalid Selector`.
- On a partitioned queue: `error while creating receiver flow: Selectors Not
  Supported on Partitioned Queue`.

**Implication for Event Filtering:** selectors work from Go, but messages a
selector does not match stay on the queue and fill its quota unless something
else consumes them. That is the "cost" to show next to topic wildcards.

## 3. Raising the connection scale in Docker

`system_scaling_maxconnectioncount` is the container variable used here. The
docs page for scaling a standalone broker gives only the CLI route.

| Route | Ran | Result |
| --- | --- | --- |
| A. Variable on a new container | `docker run ... -e system_scaling_maxconnectioncount=1000` | `show system`: `max-connections 1000` |
| B. Variable on a recreated container that reuses the storage volume | First boot with `-v vol:/var/lib/solace` and no variable, `docker rm`, then `docker run` with the same volume and the variable | Still `max-connections 100`. The variable is ignored once the volume has a configuration, as the community thread reported. |
| C. CLI on a running broker | Script in `/usr/sw/jail/cliscripts/` with `enable / configure / system / scaling / scale max-connections 1000`, run with `cli -A -s` | The broker reloaded, Docker's restart policy restarted it (`RestartCount` 1), and it came back with `max-connections 1000`. The script needs no `y`: scripts do not prompt. |

At 1,000 connections a DMR cluster could be created. The message VPN limit
was unchanged: a fourth VPN still failed with `MAX_NUM_EXCEEDED`.

**For the startup hooks:** `setup_broker.sh` only creates the container on
first start, and the Codespace's broker keeps its configuration inside the
container (no volume). So the variable works when the container is created.
For an existing container, use route C, which needs `--restart unless-stopped`
(already set) and a wait for SEMP after the reload.

## 4. Resource use at 1,000 connections

`docker stats` on the 10.26.0 broker, after it settled.

| State | Memory | CPU |
| --- | --- | --- |
| Default scale (100), idle | 1.27 to 1.31 GiB | about 8% |
| Scale 1,000, idle | 4.15 GiB | about 7 to 9% |
| Scale 1,000, 995 client connections held for 90 s | 4.15 GiB | 6.6 to 10% |

- The memory cost comes from the scale setting, not from connections. 995
  idle connections added nothing measurable.
- Connection 996 was refused with `Too Many Clients`. A few of the 1,000 are
  held by the broker's own clients.
- Two brokers at 1,000 connections would need about 8.3 GiB, and three about
  12.5 GiB, before the devcontainer, Go builds and the dashboard. The Docker
  VM here (7.65 GiB) could not run two, so this was not tested. In a 16 GB
  Codespace, two look feasible and three look tight. This needs a Codespace
  test, and possibly the 32 GB machine type, before the Event Mesh scenario is
  scoped.
- The connections were idle. Load at the 10,000 msg/s ingress limit was not
  tested.

## 5. Delayed redelivery on queues

**Ran:** a queue with `redeliveryDelayEnabled` true, initial 1,000 ms,
multiplier 200 (2.0), max 4,000 ms, `maxRedeliveryCount` 4. A Go consumer
bound with client acknowledgement and settled every message FAILED.

**Saw:** both brokers (10.26.0 and 10.8.1) accepted the queue attributes.
The Go bind was refused on both:
`error while creating receiver flow: Redelivery delay support required`.
Same result with Go API v1.10.1. The Go API source has no mention of
redelivery delay and no local transactions.

**Docs:** the JCSMP delayed redelivery page says it is "a Controlled
Availability (CA) feature" and is "triggered by a client-initiated
transaction rollback", for clients consuming with local transactions. Search
results for the general API page say C, JavaScript and Node.js do not
support it.

**Conclusion:** broker-side delayed redelivery cannot be shown from the Go
apps. Settling FAILED redelivers at once.

**What does work from Go:** the queue attribute `delivery_delay` (seconds a
message waits on the queue before it can be delivered). With
`deliveryDelay: 5`, two messages published at :43 were delivered at :48, on
both 10.26.0 and 10.8.1. Discard, Pause and Retry could use a retry queue with
a delivery delay, with the app republishing a failed message to it. That
design still needs checking.

## 6. Dead message queue behaviour by version

**Ran:** for each case, a fresh queue with `maxRedeliveryCount` 2, a DMQ,
`respectTtlEnabled` true, and one message. Removal provoked three ways:
settled FAILED until the redelivery limit, settled REJECTED once, or a 1 s
TTL with no consumer. The message's DMQ-eligible flag was left unset, set
false, or set true. Then counted where the message ended up.

**10.26.0.** New queues have `respectDmqEligibleEnabled: false`. The VPN has
`allowDmqEligibleEndpointOverrideEnabled: true`.

| `respectDmqEligibleEnabled` | DMQ-eligible flag | Max redelivery | REJECTED | TTL expiry |
| --- | --- | --- | --- | --- |
| false (default) | unset, false or true | DMQ | DMQ | DMQ |
| true | unset | DMQ | DMQ | DMQ |
| true | false | discarded | discarded | discarded |
| true | true | DMQ | DMQ | DMQ |

**10.8.1.** The attribute does not exist (`Unknown attribute
'respectDmqEligibleEnabled'`).

| DMQ-eligible flag | Max redelivery | REJECTED | TTL expiry |
| --- | --- | --- | --- |
| unset | DMQ | DMQ | DMQ |
| false | discarded | discarded | discarded |
| true | DMQ | DMQ | DMQ |

**What this means:**

- The Go API treats a message as DMQ-eligible unless the app sets the flag
  false. An unset flag reached the DMQ on both versions.
- On 10.8.1 the message flag decides. On 10.26.0 the queue decides: by
  default every removed message goes to the DMQ whatever the flag says, and
  the flag only counts on a queue with `respect_dmq_eligible_enabled`.
- The docs place this change at 10.25.10, but Docker Hub has no 10.25.10
  image. 10.26.0 behaves as described.
- REJECTED goes to the DMQ on both versions, as the core workshop's
  shock absorber shows.
- A Dead Letter Queue scenario on 10.26.0 can show the difference directly:
  one card that flips `respect_dmq_eligible_enabled` and publishes with the
  flag false.

## 7. Event Gateway inputs

**Ran on 10.26.0**, inside the broker's network namespace: a queue
subscribed to `spike/gw/>`, a REST delivery point bound to it whose REST
consumer pointed at a small HTTP server on port 8099. Then one message
published with `mosquitto_pub` (MQTT, QoS 1, port 1883) and one with an HTTP
POST to the REST port (9000, `Solace-Delivery-Mode: persistent`).

**Saw:** the HTTP server received both as `POST /ingest` with the original
payloads. The REST consumer counted 2 requests sent and 2 successful
responses. On the `default` VPN, MQTT and incoming REST are enabled on 1883
and 9000 out of the box.

Not tested: MQTT or REST with authentication, TLS, or a REST consumer outside
the broker's host.

## 8. Things learned while building the pilot

- **Activation listener.** `WithActivationPassivationSupport` fires on a
  change only. Every receiver starts passive, so the first consumer on an
  exclusive queue hears `Passive -> Active`, and a standby hears nothing until
  it is made active. The app has to say it is on standby itself.
- **SEMP monitor `txFlows`** reports `activityState` as `active-consumer` or
  `inactive`, by client name. The dashboard uses this to show which consumer
  is active. The queue monitor object has no current bind count, only
  cumulative `bindRequestCount` and `bindSuccessCount`.
- **Stop is not a crash.** The dashboard stops an app with SIGTERM, and the Go
  apps then terminate the receiver gracefully, which finishes and acknowledges
  in-flight messages. Nothing is redelivered. A crash has to be a SIGKILL.
- **`pkill -f` matches any command line** with the pattern, including a shell
  whose command text contains it. Patterns are anchored on the binary path
  (`bin/workshop exclusive ledger --role ledger-a `).
- **A stalled consumer holds exactly `max_delivered_unacked_msgs_per_flow`
  messages** (5 in the pilot), and the broker sends it nothing more. The
  standby stays inactive until the stalled consumer's session ends.
- **The Go API builds and runs on macOS** (darwin arm64 with cgo). The
  dashboard runs on the host against a broker in Docker with only the
  `SOLACE_*` variables changed.

## 9. Found while building batch 1

Tested on 10.26.0 with the Go API v1.10.0, while building the batch 1
scenarios.

**Addressing a queue by name.** The Go API publishes to topics only
(`Publish(msg, *resource.Topic, ...)`), but every queue has a network topic,
`#P2P/QUE/<queue>`. Publishing a guaranteed message to it delivers to that
queue, which needs no topic subscription. Point-to-Point uses this.

What the documentation says, and does not say:

- [ACL Configuration](https://docs.solace.com/Configuring-and-Managing/Managing-Access-Control-Lists.htm)
  states that "the topic pattern for a queue is `#P2P/QUE/<queueName>`", that
  ACLs can allow or deny publishing to it, and refers to "client applications
  that are publishing to durable queues by name".
- [Protocol Metadata and Payload Encoding Interactions](https://docs.solace.com/API/Protocol-Metadata-Payload-Encoding.htm)
  says an MQTT topic "can be prefixed with `_P2P/QUE/<queue>` to publish to a
  queue", "as with SMF".
- No docs.solace.com page we found describes publishing a guaranteed message
  to `#P2P/QUE/<queue>` from an SMF API such as Go, or what the publisher
  sees. That behaviour (delivery to the queue, `No Subscription Match` for a
  queue that does not exist) is what we measured, not documented.

**A message that reaches no queue.** Published to `#P2P/QUE/` of a queue that
does not exist, or to a topic nothing subscribes to, a guaranteed message is
acknowledged and dropped without an error. With the client profile's
`reject_msg_to_sender_on_no_subscription_match_enabled`, the publisher gets
`No Subscription Match - Topic '<topic>'` instead.

**A full queue.** With `max_msg_spool_usage` 1 (MB) and 100 KB messages, 11
were accepted and the next 3 refused to the publisher with `Spool Over Quota.
Queue or Topic endpoint limit exceeded - Topic '<topic>'`; the queue counted
them in `maxMsgSpoolUsageExceededDiscardedMsgCount`. This is the default
`reject_msg_to_sender_on_discard_behavior`, `when-queue-enabled`. A quota of
0 behaved differently (one message accepted, the next dropped with no error),
so scenarios use 1 or more.

**Signalling an app by role.** `signal_consumer.sh` finds an app by
`--client-name <name>` on its command line. An app that sets its client name
internally cannot be found, and the Break it step silently does nothing. The
checker now requires any node with `flowOn` to pass `--client-name` matching
its `flowOn.client`.

**Partitioned queues.**
- Messages with a partition key (`JMSXGroupID`, the Go API's
  `config.QueuePartitionKey`) stay with one consumer: with two consumers and
  keys K0 to K2, each key went to one consumer only. Messages without a key
  alternated between the partitions, so one key's messages would split.
- With 2 partitions and 3 consumers bound, two consumers received 15
  messages each and the third received none: a partition belongs to one
  consumer, so consumers beyond the partition count sit idle.
- Every consumer on a partitioned queue reports `activityState`
  `active-consumer`, so the dashboard's roles treat them all as active.
- The broker keeps each partition as an internal queue, `#pq/<id>/0000N`,
  listed in the monitor tree with `partitionNumber` and `partitionQueueName`.
  Its `txFlows` are empty, but the partition's own monitor entry names its
  owner in `partitionClientName` (section 10). The queue reports
  `partitionRebalanceStatus`.
- After a consumer is killed, the survivor received the crashed consumer's
  keys part way through their sequence within the rebalance delay (5 s) plus
  handoff time (3 s), still in order per key.

**Dead message queue details (10.26.0).**
- The Go API v1.10.0 has no queue browser and does not expose a message's
  delivery count or its DMQ-eligible flag on receipt. The scenario's DMQ
  watcher reads without acknowledging, so parked messages stay put, and the
  intake counts attempts itself.
- A message parked on a DMQ is listed under the DMQ's `msgs` in the monitor
  tree with `dmqEligibleAsPublished` (the publisher's flag) and
  `dmqEligible` (false once it is on the DMQ). The source queue counts what
  it moved in `maxRedeliveryExceededToDmqMsgCount`.
- Settling FAILED with `max_redelivery_count` 2 gave three deliveries, then
  the move to the DMQ, as in section 6.
- `max_redelivery_count` 0 means no limit, not zero redeliveries: a message
  settled FAILED on such a queue was redelivered 9,938 times in 4 seconds.
  With 1 it was delivered twice.
- A queue's default `dead_msg_queue` is `#DEAD_MSG_QUEUE`, which does not
  exist on a fresh broker. A message that reached its redelivery limit on such
  a queue was removed and counted in `maxRedeliveryExceededToDmqFailedMsgCount`.

## 10. Partition distribution, for the Partitioning rework

Tested on 10.26.0 with the Go API v1.10.0, on a throwaway queue with six
partitions, `partition_rebalance_delay` 5 and
`partition_rebalance_max_handoff_time` 3. The probe tool and scripts are not
kept in the repo.

**What the documentation says.**
[Message Distribution with Partitioned Queues](https://docs.solace.com/Messaging/Guaranteed-Msg/Partitioned-Queue-Messaging.htm)
says the broker hashes the key and maps it to a partition, that a message
without a key "is randomly assigned a partition", and that consumers beyond
the partition count take no load.
[Partition Rebalancing](https://docs.solace.com/Messaging/Guaranteed-Msg/Partition-Rebalancing.htm)
says the broker aims for every consumer to own the same number of
partitions, plus or minus one, and moves partitions from the most loaded
flow to the least loaded. [Queues](https://docs.solace.com/Messaging/Guaranteed-Msg/Queues.htm)
gives `partition = partition-key-hash MOD partition-count`. No page names
the hash.

**Who owns what.** Each partition's monitor entry (`#pq/<id>/0000N`, found
with `where=partitionQueueName==<queue>`) has `partitionClientName`, the
client name of its owner, empty while it has none. Measured as consumers
came and went (time after the change):

| Change | Owners after it |
| --- | --- |
| c1 binds (first) | c1: 0 to 5, after 0.9 s |
| c2 binds | c1: 3, 4, 5; c2: 0, 1, 2, after 5.3 s |
| c3 binds | c1: 4, 5; c2: 1, 2; c3: 0, 3, after 5.7 s |
| c4 binds | c1: 4, 5; c2: 1, 2; c3: 3; c4: 0, after 6.0 s |
| c2 killed (SIGKILL), 4 bound | 1 and 2 owned by nobody at once; after 5.6 s c3: 2, 3; c4: 0, 1 |
| c4 killed, 3 bound | 0 and 1 owned by nobody; after 5.6 s c1: 1, 4, 5; c3: 0, 2, 3 |
| c2 binds again | after 5.7 s c1: 4, 5; c2: 0, 1; c3: 2, 3 |
| c5 binds (4th) | after 5.5 s c2 gives up 0 to c5: 2, 2, 1, 1 |
| c1 and c3 stop | 2, 3, 4, 5 owned by nobody; after 5.6 s c2: 1, 3, 5; c5: 0, 2, 4 |

So: the spread is even to within one; the first consumer gets everything at
once, but every later change waits the rebalance delay
(`partitionRebalanceStatus` `holddown`); only the partitions that must move
do; and a crashed consumer's partitions stay unowned for the delay, with
their messages waiting.

**Which partition a key lands on.** Publishing order ids one at a time to
a queue with no consumers, and reading which partition's
`spooledMsgCount` went up, gave the partition of each id. It is
deterministic (the same 60 ids landed the same way twice) and the same on
a second queue with six partitions. Ids `O-0001` to `O-1200` landed 196,
197, 187, 201, 200 and 219 per partition. It matched none of FNV-1 or
FNV-1a (32 and 64 bit), CRC-32, Adler-32, Murmur3, Java's `hashCode`, MD5 or
SHA-1 (with and without a trailing NUL), each modulo 6. So nothing in the
workshop computes partitions from keys. The order service uses the measured
table only to choose ids that keep all six partitions busy (two orders
each); the broker still does the placing.

**No key, and an empty key.** Sixty messages without a key went round the
six partitions in turn, ten each, not at random as the documentation says.
Sixty messages whose key was the empty string all went to one partition:
an empty key is still a key.

**Which partition a message came from.** The Go API does not say. But the
replication group message id a consumer gets
(`GetReplicationGroupMessageID`, `rmid1:xxxxx-xxxxxxxxxxx-xxxxxxxx-<hex>`)
ends in the broker's own message id in hex: for 50 of 50 messages it equalled
the `msgId` the broker reported as that partition's `lastSpooledMsgId`. That
layout is not documented. Each partition's `msgs` collection lists the ids it
holds now, and its `meta.count` is how many (waiting, or delivered and not
yet acknowledged). The dashboard polls these (a list every 0.25 s, each
partition's messages every 0.5 s) and places an update by its id, or by its
key once another update with that key has been placed. Measured over ten
runs (Play, then a 30 second window once the sampler had been running 25
seconds, with no other client touching the dashboard): at the Clear pace
59 of 59 updates placed in each of five runs, at the Busy pace 147 of 147
in each of five. None disagreed with the measured table, and none sat in a
lane its consumer did not own. Updates logged in the first seconds after
the sampler starts can miss: in one early window 88 of 133 were placed.

**Crash and join, with traffic.** After a SIGKILL, the two orphaned
partitions went one to each survivor after 5.3 s, and the first update
through each was an order part way through (`#2`, `#3`), with no gap. With
one order in flight per partition, a moved partition sometimes started with
a new order's `#1`, so nothing showed a pickup; with two orders per
partition that is rare. The Clear pace keeps one order per partition, so
the crash card's check rests on the broker reporting the move, not on a
pickup line.

