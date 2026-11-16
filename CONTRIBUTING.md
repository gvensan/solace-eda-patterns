# Contributing

This guide explains how the workshop works behind the scenes, how a scenario
is defined and run, and how to add, reorder, rename or remove scenarios to
build a workshop for a particular audience. If you use Claude Code, the
[CLAUDE.md](CLAUDE.md) in the repo root gives it the same map.

## The moving parts

```
Codespace / devcontainer
 |
 +-- Solace broker            Docker container solace_10.26.0 (SEMP 8080, SMF 55555)
 |
 +-- Solace Workshop Dashboard  FastAPI on port 3000, cockpit/
 |     |
 |     +-- scenario registry   reads cockpit/scenarios/*/scenario.yaml at startup
 |     +-- process manager     runs terraform and the Go apps as subprocesses
 |     +-- SEMP proxy          reads the broker for status, diagrams and tables
 |     +-- static UI           plain HTML, CSS and JS, no build step
 |
 +-- terraform                 one tf/ folder per scenario, Solace provider
 +-- Go apps                   one binary, cockpit/apps/, every scenario role
 +-- guides/                   the markdown attendees read beside the dashboard
```

The dashboard's code lives in `cockpit/`. The folder kept its original name,
so paths, the `cockpit.app` Python module and the `COCKPIT_*` environment
variables all say "cockpit". Attendees only ever see "Solace Workshop
Dashboard".

### What happens when a Codespace starts

The Codespace runs the core workshop's prebuilt image, pinned by digest in
`.devcontainer/devcontainer.json`:

```
ghcr.io/solacedev/solace-core-workshop@sha256:6bc20e9c3115d7e8673359d069333b536dac1a20582da5343287f6c03e2f4119
```

That is the image `:latest` pointed at on 2026-10-16 (built 2026-10-12,
linux/amd64 only). It has Go 1.26.9, Terraform 1.16.5, the C toolchain the
Solace Go API needs (cgo), Docker-in-Docker, the GitHub CLI and the
dashboard's Python virtualenv at `/opt/workshop/venv` (`COCKPIT_VENV`). Its Go
build cache, Terraform provider cache and virtualenv are warmed from the
**core** workshop repo (`SolaceDev/solace-developer-workshops`, ref
`core-workshops`), not from this one. Our `go.mod`, provider version and
`requirements.txt` currently match it, so nothing is installed on startup; if
they drift, a Codespace installs the difference and starts more slowly.

**Who owns it.** The image belongs to the core workshop. It is published to
the SolaceDev organisation's GHCR by the `build-core-workshop-image.yml`
workflow in
[SolaceDev/solace-developer-workshops-infra](https://github.com/SolaceDev/solace-developer-workshops-infra)
(an internal repo; the Dockerfile is in `core-workshop-image/`). Changes to it
go through that repo's maintainers.

**How it is rebuilt.** A push to that repo's `main` touching
`core-workshop-image/` rebuilds it and moves `:latest`; a `core-vX.Y.Z` tag
publishes a versioned tag; it can also be run by hand (`workflow_dispatch`,
with `workshop_ref` choosing which branch's dependencies warm the caches).
Because we pin a digest, none of that changes this workshop until someone
updates the digest here. To move to a newer image: find its digest with
`docker buildx imagetools inspect ghcr.io/solacedev/solace-core-workshop:latest`,
put it in `devcontainer.json`, then open a fresh Codespace and run the smoke
test. Whether this workshop should get its own image, warmed from its own
repo, is still open (`docs/requirements.md`, section 6).

`.devcontainer/devcontainer.json` then runs these in order:

| Hook | Script | What it does |
| --- | --- | --- |
| `updateContentCommand` | `cockpit/apps/build.sh` | Builds the Go workshop binary into `cockpit/.state/bin/`. Included in prebuilds. |
| `postCreateCommand` | `configureEnv.sh`, `start_cockpit.sh` | Starts the broker, starts the dashboard. |
| `postStartCommand` | `setup_broker.sh` | Restarts the broker when a stopped Codespace resumes. |
| `postAttachCommand` | `start_cockpit.sh` | Starts the dashboard if it is not already running. |

Every one of these scripts is safe to run again.

The core workshop's Codespace registration (`util/register.sh`, which sent
the Codespace's public IP address and GitHub username to a Solace endpoint)
and its `portSetup.sh` (which made Codespace ports public) were removed from
this workshop on purpose. Nothing here sends attendee data anywhere, and every
forwarded port stays private. If registration is ever added back, attendees
must be told what is collected and why before it runs, for example in the
README and on the first page of the dashboard. Port 3000 opens in a browser
tab automatically, and port 8080 is Solace Broker Manager (`admin` / `admin`).

### How the dashboard runs things

A scenario is a list of **actions**. Each action is either:

- **terraform:** `apply`, `plan` or `destroy` in the scenario's `tf/` folder.
  State is written to `cockpit/.state/<scenario-id>/`, never beside the `.tf`
  files, so the repo stays clean and resetting a scenario is a directory
  delete. `terraform init` runs on first use.
- **process:** any command, usually `bash ../../apps/run.sh <scenario> <role>
  [flags]`, which runs one role of the Go binary. `long_running: true` means
  it keeps running until stopped (a consumer, a publisher loop).

Every subprocess gets the broker connection details in its environment
(`SOLACE_HOST`, `SOLACE_SMF_PORT`, `SOLACE_SEMP_URL`, `SOLACE_CLIENT_PASSWORD`,
the `TF_VAR_*` values, and so on), so terraform, the Go apps and helper scripts
always target the same broker. The values come from `cockpit/app/config.py`.

**Play** runs the scenario's `run:` list in order. A long-running step gets a
few seconds to connect, then Play moves on. **Pause** stops the scenario's
processes. **Cleanup** stops them and runs the `cleanup:` list (by default,
the destroy action).

The UI polls the dashboard, which polls the broker over SEMP. Every node in a
diagram shows its real process state and how many of the broker objects it
depends on exist right now.

### How a scenario page is laid out

1. **Run it:** Play, Pause, Cleanup, and the live diagram.
2. **Step through it:** every action as a flowchart (Play's steps, Cleanup's
   steps, then anything optional). Each step can be run on its own.
3. **Break it:** one card per failure mode, with Break it and Reset.
4. **On the broker:** read-only SEMP tables, each a closed shade.

## Anatomy of a scenario

```
cockpit/scenarios/<id>/
  scenario.yaml        everything the dashboard shows and runs
  tf/
    versions.tf        provider block, copied as is
    variables.tf       solace_url, solace_username, solace_password, msg_vpn, client_password
    *.tf               the broker objects this scenario owns
    outputs.tf         optional
    imports.tsv        every object it owns, for Reconcile and Clear broker config
cockpit/apps/scenarios/<pkg>/<pkg>.go   the Go roles, if the scenario runs apps
guides/<NNN>-<id>.md                    the attendee guide
```

Copy an existing scenario that looks like yours rather than starting blank:
`exclusive-consumer` for queues and consumers. The core workshop
(https://github.com/SolaceDev/solace-core-developer-workshops) has examples of
direct messaging (`pub-sub`), consume and republish (`processor`) and a
configuration-only scenario (`broker-tour`) to borrow shapes from.

### scenario.yaml

```yaml
id: my-scenario               # must match the folder name
order: 45                     # sidebar position, lowest first, ties sorted by title
eyebrow: Patterns             # small label above the title
title: My Scenario            # sidebar and page title
summary: >
  One paragraph on what this section shows.
objectives:
  - What the attendee should understand by the end

run: [apply, build, consumer, publisher]   # Play, in order: consumers before publishers
cleanup: [destroy]                          # optional, defaults to destructive actions

pattern:                      # the pattern in general, shown beside the summary
  what: One or two sentences on what the pattern is, without the story.
  when: When to use it.
  cost: What it costs, or what to watch out for.
  solace: How Solace supports it, only what docs or a spike confirm.
  related: [other-scenario-id, another-id]   # linked under "Next to it"

diagram: { ... }              # see below
actions: [ ... ]              # see below
failure_modes: [ ... ]        # see below
inspect: [ ... ]              # see below
```

### Actions

```yaml
actions:
  - id: apply
    label: Apply configuration
    kind: terraform
    command: apply            # apply | plan | destroy
    tf_dir: tf
    variant: primary          # primary | secondary | danger
    description: What it creates.

  - id: build
    label: Build the apps
    kind: process
    cmd: [bash, ../../apps/run.sh, build]
    cwd: .                    # relative to the scenario folder
    variant: secondary

  - id: consumer
    label: Start the consumer
    kind: process
    cmd: [bash, ../../apps/run.sh, mypkg, consume, --queue, q.my.orders]
    cwd: .
    long_running: true
    variant: secondary

  - id: reconcile             # recommended for every scenario with a tf/ folder
    label: Reconcile with broker
    kind: process
    cmd: [bash, ../../scripts/reconcile.sh, my-scenario]
    cwd: .
    variant: secondary

  - id: destroy
    label: Remove configuration
    kind: terraform
    command: destroy
    tf_dir: tf
    variant: danger
```

Other fields: `confirm:` (a prompt shown before running),
`failure_only: true` (hidden from the flowchart, used only by a failure mode),
and `stands_for: <action id>` on a variant of another action (the same app
started differently for a card, such as a slow pricing service). While a
variant runs, the diagram node of the action it stands for shows as running,
its panel says "Running as" the variant, Stop stops the variant, and its Logs
tab shows the variant's output.

The page header has two columns: the scenario's `summary:` and
`objectives:` on the left, the `pattern:` block on the right. Write
`pattern:` in general terms, with no shop or bank in it: it is what an
attendee should take away after the example is forgotten. The checker
requires `what`, `when`, `cost` and `solace`, and that every `related` id is
a scenario.

Each part of the page has an anchor, so a guide can link straight to it:
`#<id>/run`, `#<id>/steps`, `#<id>/break`, `#<id>/broker`, or
`#<id>/break-2` for the second card.

### Diagram

```yaml
diagram:
  caption: What the picture shows.
  duration: 2.6               # seconds for a packet to cross
  liveWhen: pub               # optional: no packets anywhere unless this node runs
  nodes:
    - id: pub
      label: Order Service
      sublabel: svc-my-publisher
      kind: app               # app | broker | config
      col: 0                  # column, left to right; nodes in a column stack
      action: publisher       # clicking starts or stops it, and opens its log
      requires:               # broker objects this node needs, checked live
        - label: svc-my-publisher
          semp: /msgVpns/{vpn}/clientUsernames/svc-my-publisher
    - id: broker
      label: Solace Broker
      kind: broker
      col: 1
      action: apply
      contains:               # dotted boxes drawn inside the broker
        - id: queues
          label: Queues
      provides:               # objects it holds; group puts one in a box
        - label: q.my.orders
          group: queues
          semp: /msgVpns/{vpn}/queues/q.my.orders
    - id: enrich
      label: Enricher
      kind: app
      shape: processor        # hexagon: consumes at the top, publishes from the bottom
      col: 2
      action: enrich
  flows:
    - { from: pub, to: broker, label: orders, delay: 0 }
    - { from: broker, to: enrich, via: queues, delay: 0.6 }   # via: the box it leaves from
    - { from: enrich, to: broker, label: enriched, delay: 1.2 }
    # A consumer of what a processor republishes names it as its source,
    # so its packets stop when that processor stops.
    # - { from: broker, to: sink, source: enrich, delay: 1.8 }
```

A publish can also name the box it goes into, so the line ends at the
queue whose subscription takes it rather than at the broker's edge. When a
topic goes into several queues, write one flow per queue and label one of
them: the lines fork from the publisher. Event-Carried State Transfer does
this, so orders visibly go into both queues and customer records into one.

```yaml
    - { from: orders, to: broker, via: shipq, label: orders, delay: 0 }
    - { from: orders, to: broker, via: invq, delay: 0 }
```

A label sits near the arrow end of a curved line. Where that crowds a
node, `labelAt:` (0 to 1 along the line) moves it.

Where lines meet nodes is worked out for you. An app's lines are pinned
across its side (one in the middle, two at 30% and 70% down), a direct
line meets the broker level with its pin and never level with a queue
box, and the lines on one side keep their order so they do not cross.
Lines into one queue box land apart, and a publish landing on the side a
delivery leaves from lands clear of it; the box grows to hold them.
Keep labels and step badges off arrowheads and the points where lines
meet a box or the broker: move them with `labelAt:` or the step's `at:`.
Give every publish into a queue its `via:`, and only into a queue whose
subscription takes it. If lines still cross in the screenshot, move an
app to another row (the order of nodes in a column) or another column.

Where the broker itself moves messages from one of its boxes into another
(a replay log read back into a queue), draw it as a hop:
`{ id: read-back, from: broker, to: broker, via: <from box>, into: <to box>, label: read back at start }`.
The two boxes sit further apart, with a short line and its label between
them. Nothing animates on it, so the label must say when it happens. A
hop may open a path, because what it moves was stored earlier.

A diagram can number the route each message takes. Give the flows an
`id:` and list `paths:`, one per message, each step a flow and one
sentence. The page puts the step numbers on the lines, lists the paths
under the diagram as **Follow a message**, and highlights a whole path
when its name or one of its lines is clicked. Numbers run on from one
path to the next. Each line carries one number, so a line two messages
share belongs to the first path that uses it, and the second path's text
points to it. The checker makes each path join up: after the first step,
a step starts where an earlier one ended (or where the first began, for a
fan-out), and a step out of a queue needs an earlier step into it. So
draw every hop a message really takes. `at:` on a step (0 to 1) moves its
number along the line. `also: [ids]` on a step names the other branches of
the same publish, when one publish forks into several queue boxes: they are
one step, so they share one number (on the step's own line) and highlight
with it. Without it, a fork would read as several writes by the app.

```yaml
  flows:
    - { id: query, from: account, to: broker, label: query }
    - { id: query-in, from: broker, to: history, label: query, source: account }
  paths:
    - label: The account page asks, the read side answers
      steps:
        - flow: query
          text: "The account page asks about a customer: a direct message, kept on no queue."
        - flow: query-in
          text: The broker delivers the question to the order history.
```

Write step text in block style, as above: in an inline `{ ... }` mapping,
YAML reads a comma in the text as the end of the value.

A consumer node can also say which queue it binds and the client name it
connects with. The dashboard then asks the broker whether that consumer is
the one being delivered to, labels the node "active consumer" or "standby", and
stops packets to a standby. This is what shows the
broker's choice on an exclusive queue. The app has to connect with that client
name (`solace.ConnectAs`).

```yaml
    - id: ledgerA
      action: ledger-a
      flowOn:
        queue: q.excl.postings
        client: excl-ledger-a
```

Every mark on the page explains itself on hover: the status dot, the
`1/1` count, the counts inside the broker, `logs`, the active or standby
label, the red border of a failure in effect, and so on. Hover text is set
only through `indicator(node, "<name>", text)` (in `diagram.js` or
`cockpit.js`; `h()` in `cockpit.js` takes `indicator: ["<name>", text]`).
Each name needs a row in `guides/000-reading-the-dashboard.md` tagged
`<!-- indicator: <name> -->` (one tag may list several names, comma
separated). `check_scenarios.py` fails on hover text set any other way
(`tip()`, `.title =`, a `title` attribute), on a name never given text, on
a name with no tagged row, and on a tagged row for a name nothing uses.

A node with `flowOn` is labelled "active, not acking" when it is the active
consumer, holds unacknowledged messages, and its acked count on the broker
has not moved for 5 seconds (`STALL_SECONDS` in `cockpit/app/main.py`).

#### Queue depth and arrivals on a box

A queue box can show how many messages the queue holds now, and a dead
message queue box can draw each message the broker moves into it, with the
reason, from the counters on the queue it left. Dead Message Queue uses both.

```yaml
      contains:
        - id: orders
          label: q.dlq.orders
          depth: { queue: q.dlq.orders, unit: waiting }
        - id: dmq
          label: q.dlq.orders.dmq
          depth: { queue: q.dlq.orders.dmq, unit: parked }
          arrivals:
            from: orders                 # the box messages leave
            queue: q.dlq.orders          # whose counters say why
            reasons:                     # label shown: monitor counter
              failed 3 attempts: maxRedeliveryExceededToDmqMsgCount
              expired: maxTtlExpiredToDmqMsgCount
```

`/state` returns each box's depth (the queue's `msgs` count) and counters.
Each rise in a counter after the page opened is drawn as a dot hopping
between the boxes, labelled with the reason, then along the flow `via:` the
box to its reader, if that reader is running. Each label has its own
counter, so a hop cannot take another's reason. When several counters move
between two polls, all their hops run one after another, at most three per
reason, the last saying how many more there were.

#### What an app holds: `holds:`

Where the lesson is what an app keeps because of the events it receives (a
copy of another service's data, a read model, a database), the node can
show it. Event-Carried State Transfer uses it for the customer database,
shipping's copy and invoicing's lookups.

```yaml
    - id: shipping
      action: shipping
      holds: { kept: memory, count: customers }
```

- `kept:` is `memory` or `disk`. The card says which, and when the app is
  not running, in-memory state shows as gone rather than as old rows.
- `count:` is optional: the unit for the row count drawn in the node's
  bottom row (`8 customers`, orange at 0).
- `role:` is optional: one sentence, shown on the card, saying what this
  app is in the pattern ("A copy, built only from customer events").
- `copyOf: <node>` is optional: the card compares its rows with that
  node's, on the first column and every column both share, and says
  `Matches Customer Service: 8 of 8`, or how many are missing or different.
- `counters: true` marks a table of counters, whose rows are never marked
  as changed (they change on every read).
- The diagram needs a `holdsCaption:` once any node has `holds:`: a lead
  sentence over the cards that says what to compare.

The app reports it itself, through `solace.NewHolding(title, columns...)`
and `Set(rows, note)` in `cockpit/apps/internal/solace/state.go`, whenever
it changes. Each report is one stdout line, `@state {json}`, at most twice a
second; the dashboard keeps the latest of the run now in place, leaves it
out of the log, and returns it in `/state` as the node's `holds`. The page
draws a card per node under the diagram and marks rows that changed since
the last read.

#### Settings: a choice on a node

A scenario can offer a choice about how an app runs, shown in that node's
side panel, with the current choice as the node's sublabel. Each choice is a
list of arguments, put into any action's `cmd` where it says
`"{setting:<name>}"`, so every start uses it (Play, a card's reset, a click
in the panel). Choosing another value restarts any running action that uses
it. The dashboard holds the value until it restarts or the scenario is
reset; `smoke.py` runs with the default, or with `--set <name>=<value>`.
Each choice needs hover text on the node's sublabel and the panel's buttons
(both done by the dashboard), and a smoke run of its own.

```yaml
settings:
  pace:
    label: Pace
    node: orders               # the node whose panel shows it
    default: clear
    choices:
      - value: clear
        label: Clear
        sublabel: "clear: 6 orders, 2/s"
        description: Six orders in flight, two updates a second.
        args: [--in-flight, "6", --rate, "2"]
      - value: busy
        label: Busy
        sublabel: "busy: 10 orders, 5/s"
        description: Ten orders in flight, five updates a second.
        args: [--in-flight, "10", --rate, "5"]
actions:
  - id: orders
    cmd: [bash, ../../apps/run.sh, partition, orders, "{setting:pace}"]
```

#### Partitioned queues: lanes and real events

A box inside the broker can be drawn as one lane per partition of a
partitioned queue, and a diagram can put one dot on the page for each real
update its consumers report handling. Partitioning uses both.

```yaml
diagram:
  events:                    # each line a consumer prints for an update it handled
    pattern: '\] (?P<order>O-\d+) #(?P<seq>\d+) (?P<status>\w+) \((?:key (?P<key>[^,)]+)|no key)(?:, msg (?P<msg>\d+))?\)'
    flags:                   # optional; gap, order, picked, redelivered
      gap: 'GAP: '
      order: 'OUT OF ORDER: '
  nodes:
    - id: broker
      contains:
        - id: partitioned
          label: q.part.orders
          partitions: { queue: q.part.orders, count: 6 }
```

- `count:` must equal the queue's `partition_count` in the scenario's
  terraform (the checker compares them), because the lanes are laid out
  before the broker is asked.
- Each lane shows the partition's owner (`partitionClientName`), the
  messages it holds now and how many it has taken in, read by
  `cockpit/app/partitions.py` over SEMP several times a second while the
  page is open. A line from each lane runs to its owner's node, and swings
  when the broker moves the partition. Lines `via:` the box are not drawn.
- Every node with a `flowOn` on that queue is a consumer: its sublabel says
  which partitions it owns, and a row shows the last updates it handled.
- `events:` reads the stdout of those consumers' actions. Named groups:
  `order` and `seq` (required), `status`, `key` and `msg` (optional). `msg`
  is the broker's message id (`solace.SpoolID`), which is how an update is
  placed in a partition: the dashboard matches it against the ids it saw in
  each partition. With `events:`, the diagram draws no looping packets at
  all: only real updates move.
- `GET /api/scenarios/<id>/partitions?since=<time>` returns the lanes,
  recent ownership moves and the parsed events, for the page and for
  `smoke.py`.

A flow out of the broker can name a queue with `traffic: <queue>`. It then
moves only while messages are arriving on that queue, judged from the
queue's spooled count on each poll (`TRAFFIC_SECONDS` in
`cockpit/app/main.py`), for a line whose traffic no app on the diagram
sends: across a bridge, or out of a REST delivery point. The queue must be
in the scenario's `tf/imports.tsv`.

A flow that carries only the odd message (from a queue to its dead message
queue's reader, say) can set `animate: false`. It is drawn, but never shows
packets, so the picture does not suggest a steady stream that is not there.

A node's `sublabel` may be at most 30 characters: a processor's hexagon cuts
text at about that length. The checker enforces it.

Rules the checker enforces, because they are what the workshop teaches:

- Exactly **one** `kind: broker` node per diagram.
- Every flow starts or ends at the broker. Apps never talk to each other
  directly. An app that consumes and publishes has a flow in from the broker
  and a flow back to it, and should be drawn with `shape: processor`.
- A node's `flowOn` has both `queue:` and `client:`.

`{vpn}` in any `semp:` path is replaced with the message VPN.

### Failure modes

Each entry becomes a card under **Break it**. Aim for two per scenario, each
showing a real error from the Go API or the broker. The cards show as a row
of tiles; clicking a tile opens its card underneath, one at a time. A card
stays open after Reset, on the recovery, until **Done** closes it and puts
a tick on its tile.

```yaml
failure_modes:
  - id: consumer-offline
    title: The consumer goes offline
    needs: running            # running (default) or applied (config-only scenarios)
    breaks: One line on what goes wrong.
    watch:                    # what to look for once it has: four parts, all required
      normally: What the app does when all is well, and what is unaffected.
      broke: What is gone or failing, and the exact lines to find in the output.
      impact: What stops, what keeps working, and any risk left behind.
      recover: What Reset does, what you see when it worked, whether anything was lost.
    # watch: can still be one paragraph of plain text (older scenarios);
    # new and reworked scenarios use the four parts.
    why: Why it breaks. Shown in a closed shade.
    real_world: Where you meet this in production. Same shade.
    docs:                     # https links, Go API page first, then broker docs
      - label: "Go API: consuming guaranteed messages"
        url: https://docs.solace.com/API/API-Developer-Guide-Go/Go-PM-Subscribe.htm
    node: consumer            # diagram node marked while it is in effect
    logs: consumer            # action whose output is streamed into the card
    trigger:                  # start: and stop: return at once, run: waits for it to finish
      - stop: consumer
    reset:
      - start: consumer
```

When several instances of one app share a job and the broker decides which
does what, a card acts on the role, not a named instance. `node:` and `logs:`
then name a role, resolved from the broker when Break it is pressed among the
nodes with `flowOn`, and kept for that failure through Reset:

Three roles exist. `active` and `standby` are what the broker reports for a
consumer's flow on the queue. `other` is an active consumer other than the
one the card's `node:` names, for a card on a shared queue that hits one
worker and watches another (`other` cannot be the `node:` itself). On a
shared queue several consumers are active at once; the one with the lowest
client name is taken, by the page, by `smoke.py` and by
`signal_consumer.sh` alike, so the box that is marked is the app that is hit.

```yaml
    node: {role: active}      # or standby; add queue: when there are several
    logs: {role: standby}     # the log of the app that holds that role
    trigger:
      - run: kill-active      # bash ../../scripts/signal_consumer.sh q.my.orders active KILL
    reset:
      - start: consumer-a     # start every instance; one already running is skipped
      - start: consumer-b
```

`cockpit/scripts/signal_consumer.sh <queue> <active|standby> <signal>` asks
the broker which client holds the role and signals the workshop app
connected under that client name. KILL is a crash. TERM is a stop, which
becomes a kill after 5 seconds, as the dashboard's own Stop does. Any other
signal is passed on for the app to handle (the exclusive consumer's ledger
freezes on USR1). The card text says "the active ledger", "the standby",
never "Ledger A": after one card the roles may have swapped, and a card must
not rearrange the apps behind the attendee's back to make a name true.

#### Expected outcomes: `expect:`

Each failure mode states what must happen, so `smoke.py` can fail on the
outcome and not only on whether processes started. The page ignores this
block.

```yaml
    expect:
      break:                       # checked after the trigger has run
        within: 15                 # seconds; optional, default 15
        logs:
          - log: {role: standby}   # an action id, or a role as for logs:
            has: ["ACTIVE: the broker is delivering", "applied posting"]
            lacks: ["GAP", "already applied"]
      reset:                       # checked after the reset has run
        logs:
          - log: {role: active}
            has: ["STANDBY until"]
```

- `has:` patterns must each appear in that log within `within` seconds.
  `lacks:` patterns must not appear at any point in that window. Patterns are
  Python regular expressions, matched against single log lines, so escape
  `(` and `)` (`"\\(redelivered\\)"` in YAML double quotes).
- Only output written after the step began counts: lines already in the log
  before Break it (or before Reset) are ignored, and a run started since then
  counts in full.
- Roles are frozen at Break it, together with the card's own `node:` and
  `logs:`, and stay the same for the reset checks. So after a crash, `{role:
  active}` in `reset:` still means the app that crashed.
- `settle: <seconds>` (optional) ignores output written in the first seconds
  after the steps, for something that legitimately lingers: messages already
  on the queue when a reset puts the cause right. The window starts after it.
- With any `lacks:` the check always waits the full window, since absence
  can only be shown by waiting. Without, it ends as soon as every `has:`
  pattern has been seen.
- A diagram with partition lanes can also check the dashboard's
  `/partitions` answer, alone or beside `logs:`:

  ```yaml
        partitions:
          orders: one-consumer     # or split: some order reached two consumers
          moved: true              # some partition changed owner
          picked: true             # some consumer picked an order up part way
          owned: all               # every partition has an owner by the end
          owns:                    # partitions a node owns by the end
            - { node: {role: active}, max: 0 }
            - { node: fulfil4, min: 1, max: 1 }
  ```

  `orders: one-consumer` waits the full window, like `lacks:`.
- A box with `arrivals:` can check how much each of its counters rose in
  the window, which is what labels its hops:

  ```yaml
        boxes:
          dmq: { expired: 3, failed 3 attempts: 0 }
  ```

  A rise above the count fails at once; a count of 0 waits the full window.
  The checker also requires each `arrivals:` label to name a different
  counter, and only counters that count moves to a dead message queue.
- A node with `holds:` can check how many rows it reports, at some point
  in the window:

  ```yaml
        holds:
          - { node: shipping, max: 2, missing: { min: 6 } }
  ```

  `matched:` and `missing:` need a node with `copyOf:`.

- A scenario can also say what must hold once Play has finished, before any
  card, with the same checks under a top-level `expect: play:`.
- `check_scenarios.py` validates the block: known phases, a `logs:` list or
  `partitions:` checks, a valid `log:` reference, at least one of `has:` or
  `lacks:`, and patterns that compile.

Write expectations from the card's own `watch:` text, so the words a card
promises are the words the test checks.

Let the app print the Go API's error as it is. Do not wrap it in your own
explanation: the card's `watch`, `why` and `real_world` text is where the
explanation goes.

### Inspect views

```yaml
inspect:
  - label: Queues
    kind: semp
    api: monitor              # config (default) for objects, monitor for live counters
    path: /msgVpns/{vpn}/queues?count=100
    columns: [queueName, msgSpoolUsage, lastSpooledMsgId, highestAckedMsgId]
    ui_hint: Manager → Queues # where to find the same thing in Broker Manager
```

Add `empty:` to say what an empty table means and what fills it ("No ledger
is bound right now. Press Play."), since a queue is empty until configuration
is applied but its consumers are empty until apps run. The views are read
again every 5 seconds while the page is open.

For a live backlog use `msgSpoolUsage`, not `spooledMsgCount`, which is
cumulative and never falls.

### Terraform and imports.tsv

Name every broker object with a prefix unique to the scenario (`q.my.`,
`svc-my-`, `acl-my-`, `cp-my-`). Scenarios must not share objects: Cleanup on
one would break another.

`tf/imports.tsv` lists every object the scenario owns, one per line, the
terraform address and the import id separated by a **tab**:

```
solacebroker_msg_vpn_queue.orders	{vpn}/q.my.orders
solacebroker_msg_vpn_client_username.publisher	{vpn}/svc-my-publisher
```

It drives two things: **Reconcile with broker** (imports existing objects
back into state after state is lost) and **Clear broker config** (deletes every
queue, client username, ACL profile and client profile any scenario lists).
An object missing from the file is invisible to both.

### Go apps

All roles compile into one binary so attendees wait for one build.

1. Add a package under `cockpit/apps/scenarios/<pkg>/`.
2. Write each role as `func Role(args []string)` that parses its own flags.
3. Register it in `cockpit/apps/cmd/workshop/main.go` as `"<pkg> <role>"`.
4. Use the helpers in `cockpit/apps/internal/solace/`: `Connect`,
   `StartDirectPublisher`, `StartDirectReceiver`, `Subscribe`,
   `StartPersistentPublisher`, `BindQueue`, `PublishKeyed`, `Logf`, `Errf`,
   `Eventf`, `WaitForStop`. They read connection details from the environment
   and keep log output consistent across scenarios.

`run.sh` rebuilds the binary automatically when any `.go` file is newer than
it. To build by hand inside the container:

```bash
bash cockpit/apps/build.sh
```

### The guide

Add `guides/<NNN>-<id>.md`, where `NNN` matches the scenario's `order`. Follow
the existing guides: what the scenario shows, what to look at while it runs,
a **Break it** section matching the failure modes, a short "Try this" list,
and a `Next:` link to the following guide. Add a row to the sections table in
the root `README.md`.

### Audio overview

A scenario can have a spoken overview of its use case, played from
**Listen: the use case** in the top bar, which opens a column on the right
with the controls and the transcript.

- `cockpit/scenarios/<id>/narration.md` is the script: plain paragraphs,
  about 1,500 characters (100 seconds). Lines starting with `#` are notes
  and are not spoken. Open with an accurate definition of the pattern, in
  the terms its usual sources use (the pattern guide, Fowler, the
  technology's own docs), then move into the use case: why this kind of
  system needs it, the usual alternative and what it costs, what the
  scenario does, the pattern's price, and which Break it cards show it.
  Never open on a formula such as "Picture a ...", and vary the wording
  from one script to the next. Use the page's own words (card titles,
  table names, **Play**).
- `python3 cockpit/scripts/narrate.py <id>` voices it with ElevenLabs and
  writes `cockpit/static/audio/<id>.mp3` and its entry in
  `cockpit/static/audio/index.json` (length, and when each sentence starts,
  shown as the caption). `--dry` counts characters without calling
  anything. It needs a key in `ELEVENLABS_API_KEY`, set in `.env.dev` (see
  `.env.dev.example`) or in the environment; attendees never do, because the
  audio is committed. Takes are cached in `~/.cache/eda-patterns-narration`.
- `check_scenarios.py` fails when a `narration.md` has no audio, or has
  changed since it was voiced.

## Checking your work

```bash
python3 cockpit/scripts/check_scenarios.py
```

It checks that `run`, `cleanup`, diagram and failure mode references name real
actions and nodes, that each diagram has one broker and no flow bypasses it,
that scripts and terraform folders exist, and that a scenario with Reconcile
has an `imports.tsv`. Run it before every commit.

Then drive the scenario the way an attendee would:

```bash
${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py <scenario-id>
```

It runs Play, every Break it and Reset with the same step rules as the page,
Cleanup, Clear broker config, and Play again from an empty broker. It fails
if a step does not reach running or succeeded, and prints each card's log so
you can read whether the failure and the recovery look the way the card says.

Give it the dashboard to itself:

- **Close the dashboard's tabs** (or leave them alone). A Break it, Reset,
  Play or Stop pressed in a tab while smoke runs changes what it is
  measuring. The dashboard logs every change and who sent it
  (`GET /api/activity`), and smoke fails the run if another client changed
  a scenario it is testing, cleared the broker or stopped everything.
  Changes to other scenarios do not count: scenarios share no broker
  objects.
- **Keep the machine awake.** On a laptop, run it under `caffeinate -i`
  (macOS). If the machine sleeps, the dashboard's timers stop while the
  clock smoke waits on runs on, and a Play can look hung for as long as the
  sleep. Smoke compares the wall clock with the monotonic clock, which stops
  during sleep, and fails the run if they drift apart by more than 5
  seconds.
- **Do not save Python under `cockpit/app/` or `cockpit/scripts/` during a
  run**, `config.py` and `smoke.py` included. In development mode the
  dashboard watches both, and a save, even with no change, restarts it,
  which stops every app and cuts a smoke run short.
- **Do not edit Go under `cockpit/apps/` during a run either.** `run.sh`
  rebuilds the binary whenever an app starts and the source is newer, so a
  half-written change makes every Play fail at "Build the apps", and a
  compiling change swaps the binary under a scenario mid-run. Write new Go
  elsewhere and move it in between runs.
- The sleep check has not been tested against a real sleep: only the
  clocks it relies on were checked (`time.monotonic` is
  `mach_absolute_time()` on macOS, which stops during sleep).

```bash
caffeinate -i ${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py partitioning
```

`--set <name>=<value>` runs with a setting other than its default, for
example `--set pace=busy`; smoke puts it back afterwards. Run a scenario with
settings once per choice.

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

To restart the dashboard by hand: `./dev.sh restart` on a host machine, or
inside a container:

```bash
pkill -TERM -f "[u]vicorn cockpit.app"; sleep 4; bash cockpit/start_cockpit.sh
```

Then look at the page in a browser: Play, each Break it and
Reset, Cleanup, and Play again from a clean broker (**Clear broker config**
gets you there).

## Changing the set of scenarios

The sidebar is whatever folders are in `cockpit/scenarios/`, grouped by
`eyebrow:` (the pattern guide's category) and sorted by `order:`.

### Remove a scenario

Delete `cockpit/scenarios/<id>/` and `guides/<NNN>-<id>.md`, remove its row
from `README.md`, fix the `Next:` link in the guide before it, and remove its
Go package and its roles in `cockpit/apps/cmd/workshop/main.go`.

### Reorder scenarios

Change `order:` in each `scenario.yaml`. The numbers only need to sort. The
convention here is the pattern guide's category number times 100 plus the
pattern's position in it times 10, so Exclusive Consumer (category 3, fifth)
is 350. Keep the guide file prefixes, the README table and the `Next:` links
in the same order. Renaming guide files is
optional but keeps the folder readable.

### Rename a scenario

- **Display name only:** change `title:`, `eyebrow:` and `summary:`, plus the
  guide's heading and the README row. This is the usual case and the safest,
  because nothing else refers to them.
- **The id as well:** rename the folder, change `id:`, and update every place
  the id appears as a string: `reconcile.sh <id>` and `forget_state.sh <id>`
  arguments in its actions, the guide file name and links, and the README.
  Search for the old id before committing:

  ```bash
  grep -rn "old-id" cockpit guides README.md
  ```

  Terraform state lives under `cockpit/.state/<id>/`, so anyone with the
  scenario already applied should run Cleanup before switching to the
  renamed version, or use Reconcile afterwards.

### Change the story

A scenario's story lives in three places: the text in `scenario.yaml`, the
topic names and payloads in the Go package, and the object names in `tf/`.
Change all three together, and update `imports.tsv` for any object you
rename.

### Shared code and the core workshop

This dashboard started as a copy of the core workshop's. A fix to shared code
(`cockpit/app/`, `cockpit/static/`, `cockpit/apps/internal/`,
`.devcontainer/`, the scripts) may be needed in both repositories until a
shared base is decided (`docs/requirements.md`, section 6).

## Writing style

- Plain, direct sentences, written for someone new to event-driven design.
- No em dashes, en dashes or double hyphens in anything attendees read. Use commas,
  colons or parentheses, or split the sentence.
- Write "Solace Agent Mesh" in full, never an abbreviation.
- Say "Solace Broker Manager" for the broker UI and "Solace Workshop
  Dashboard" (or "the dashboard") for the control surface.

## Before opening a pull request

- `python3 cockpit/scripts/check_scenarios.py` passes.
- `smoke.py` passes for the scenario on a Standard edition broker of the
  pinned version, and its logs read the way the cards say.
- Play, every Break it and Reset, and Cleanup work in a fresh devcontainer or
  Codespace.
- No credentials beyond the local broker's `admin` / `admin` and the shared
  workshop client password.
- The guide, the README table and the `Next:` links match the sidebar.
