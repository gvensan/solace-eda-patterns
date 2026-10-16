# Solace Workshop Dashboard

The control surface attendees use to drive the workshop. It applies broker
configuration, runs sample applications, streams their output back live, and
shows what actually landed on the broker.

## Architecture

```
Browser (port 3000)          static HTML/CSS/JS, no build step
     |
     |  REST (control)  +  WebSocket (log streams)
     v
FastAPI dashboard            one uvicorn process
     |
     +-- ProcessManager       asyncio subprocesses, one per action
     +-- SEMP client          httpx, proxies broker reads
     +-- Scenario registry    YAML descriptors under scenarios/
     |
     v
terraform / sample apps  -->  Solace broker (docker, SEMP 8080)
```

Three decisions shape everything else:

**Subprocesses, not threads.** Terraform and the Go sample apps are separate
binaries, so both have to be subprocesses regardless. Treating any other tool the
same way means one code path for every language, real cancellation via signals,
and crash isolation, so a hung consumer can never take the dashboard down with it.

**No frontend framework.** The dashboard is cards, forms, and log panes. A build
step would cost container boot time and add a failure mode during a live
workshop without buying anything. Attendees who open devtools see the code we
wrote.

**Scenarios are data.** Adding a workshop module means dropping in a folder with
a `scenario.yaml`. The UI, process wiring, and teardown come for free.

## Running it

In the devcontainer this starts automatically on attach. To run it by hand:

```bash
bash cockpit/start_cockpit.sh
```

On a host machine, `./dev.sh start` in the repo root starts the broker and the
dashboard together, in development mode (`COCKPIT_DEV=1`): the page reloads
itself when a static file or scenario changes, and the server restarts when
its Python changes. A changed `scenario.yaml` is picked up without a restart
in either mode.

Then open http://localhost:3000. The script is idempotent, so running it twice
is harmless.

Configuration comes from the environment, with defaults matching
`setup_broker.sh`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `COCKPIT_PORT` | `3000` | Port the dashboard listens on |
| `SOLACE_HOST` | `localhost` | Broker host |
| `SOLACE_SEMP_PORT` | `8080` | SEMP / Manager port |
| `SOLACE_SMF_PORT` | `55555` | Messaging port the Go apps connect to |
| `SOLACE_SEMP_USER` | `admin` | Management username |
| `SOLACE_SEMP_PASSWORD` | `admin` | Management password |
| `SOLACE_MSG_VPN` | `default` | Message VPN to configure |
| `SOLACE_CLIENT_PASSWORD` | `solace-workshop` | Password of every scenario's client usernames, for terraform and the apps |
| `COCKPIT_STATE_DIR` | `cockpit/.state` | Terraform state and logs |
| `COCKPIT_DEV` | unset | `1` turns on development mode (page and server reload on edits) |

## Adding a scenario

Create `cockpit/scenarios/<id>/scenario.yaml`. This is an excerpt of
`exclusive-consumer`; CONTRIBUTING.md has the full schema.

```yaml
id: exclusive-consumer
order: 350                     # category 3 of the pattern guide, fifth pattern
eyebrow: Consumption Patterns  # the guide's category; the sidebar groups by it
title: Exclusive Consumer
summary: >
  A ledger has to apply payment postings one at a time, in order. ...

objectives:
  - See two consumers bound to one exclusive queue, with only one receiving

run: [apply, build, ledger-a, ledger-b, publisher]   # Play, consumers first
cleanup: [destroy]

actions:
  - id: apply
    label: Apply configuration
    kind: terraform            # runs terraform in tf_dir
    command: apply             # apply | plan | destroy
    tf_dir: tf
    variant: primary           # primary | secondary | danger

  - id: ledger-a
    label: Start ledger A
    kind: process              # runs a command; usually one Go role
    cmd: [bash, ../../apps/run.sh, exclusive, ledger, --role, ledger-a, --client-name, excl-ledger-a]
    cwd: .                     # relative to the scenario folder
    long_running: true         # keeps running until stopped

inspect:                       # read-only SEMP views, {vpn} is substituted
  - label: Consumers on q.excl.postings
    kind: semp
    api: monitor               # live counters; config is the default
    path: /msgVpns/{vpn}/queues/q.excl.postings/txFlows?count=10
    columns: [clientName, activityState, ackedMsgCount, unackedMsgCount]
    ui_hint: Manager → Queues → q.excl.postings → Consumers
```

### The diagram is the interface

A scenario with a `diagram:` block renders it as the main surface, below the
transport controls. Nodes are not illustrations: each carries live status, a
checklist of the configuration it depends on, and a way into its own logs.

- **Status dot:** idle, running, done, failed, or missing configuration.
  Driven by the real process state and by SEMP, not by a timer.
- **`3/5` badge:** how much of that node's configuration actually exists on
  the broker right now.
- **`logs` button:** opens that node's log stream in the side panel.
- **Click the node:** apps start or stop; every node opens its detail panel.
- **Packets:** animate only while the process named by `liveWhen` is running,
  so a stopped publisher cannot leave the picture implying traffic. They are
  representative rather than per-message.
- **Active or standby:** a consumer node with `flowOn: {queue, client}` is
  labelled with what the broker reports for its flow on that queue, and a
  standby gets no packets. On an exclusive queue this shows which consumer the
  broker chose.

```yaml
diagram:
  caption: What the picture shows.
  duration: 2.4                    # seconds for a packet to cross
  liveWhen: payments               # packets move only while this node runs
  nodes:
    - id: payments
      label: Payments Service
      sublabel: two postings a second
      kind: app                    # app | broker | config
      col: 0                       # column, left to right
      action: publisher            # what a click toggles, and whose logs open
      requires:                    # checked against SEMP every poll
        - label: svc-excl-payments
          semp: /msgVpns/{vpn}/clientUsernames/svc-excl-payments
    - id: broker
      label: Solace Broker
      kind: broker
      col: 1
      action: apply
      contains:                    # dotted boxes drawn inside the broker
        - id: postings
          label: q.excl.postings
      provides:                    # same checks, phrased as what it creates
        - label: q.excl.postings
          group: postings          # counted in that box
          semp: /msgVpns/{vpn}/queues/q.excl.postings
    - id: ledgerA
      label: Ledger A
      kind: app
      col: 2
      action: ledger-a
      flowOn:                      # ask the broker: active or standby?
        queue: q.excl.postings
        client: excl-ledger-a
  flows:
    - from: payments
      to: broker
      label: postings
      delay: 0                     # stagger a burst or a fan-out
    - from: broker
      to: ledgerA
      via: postings                # leaves from that box
      delay: 0.8
```

Nodes sharing a `col` stack and centre against each other, so one broker sits
level with two ledgers. `kind` picks the treatment: `broker` is filled
green (the subject of the workshop), `config` is dashed (broker state rather
than a running process), `app` is the default.

`{vpn}` in a `semp:` path is substituted with the configured message VPN. A
missing object is reported by SEMP as HTTP 400 with `NOT_FOUND`, which the
dashboard reads as absent rather than as an error.

Logs live in the node panel rather than in a section further down the page: every
action's output is reachable from the thing that produces it.

### Play, Pause and Cleanup

Every scenario is driven by three buttons, with the steps behind each listed
beneath them so nothing is a mystery box:

- **Play** runs the scenario's `run:` sequence in order.
- **Pause** stops this scenario's processes and leaves broker configuration
  alone, so Play can start them again without re-applying anything.
- **Cleanup** stops everything, then runs the `cleanup:` sequence to remove the
  configuration from the broker. It asks for confirmation first.

Below the diagram, the Actions section draws every action as a flowchart: the
`run:` sequence under Play, the `cleanup:` sequence under Cleanup, and anything
in neither under Optional. Each step's dot shows its run state. Clicking a step
opens its description with Run and Stop for that step alone.

```yaml
run:                      # Play, in order
  - apply
  - publish

cleanup:                  # Cleanup, in order. Defaults to the destructive
  - destroy               # actions, which is usually just the destroy step.
```

Order cannot be inferred (`exclusive-consumer` has to bind ledger A before
ledger B so A is the active one, and both before it
publishes), so it is stated explicitly. Without a `run:` block the default is
every non-destructive action in declaration order.

A `long_running` step is given a few seconds to connect and then left running
while the sequence moves on, since a consumer never exits on its own. Play stops
long-running steps before restarting them, so a second press produces the same
end state as the first rather than publishing into subscribers left over from a
previous run. A step that fails stops the sequence and the reason is shown next
to the buttons, rather than the controls silently returning to rest.

Cleanup stops processes before its teardown steps so terraform is not deleting
configuration out from under a live client, and it continues past a failing step
rather than leaving a scenario half removed.

Terraform state is written to `COCKPIT_STATE_DIR/<scenario-id>/` rather than
beside the `.tf` files, so resetting a scenario is a directory delete and the
repo stays clean for the next attendee. `terraform init` runs automatically
before the first apply.

### Failure modes

A scenario page is laid out in numbered parts: **Run it** (the transport and
diagram), **Step through it** (the Actions flowchart), **Break it**, and **On the
broker** (the Inspect views, each a closed shade until opened).

**Break it** holds one card per entry in `failure_modes:`. A card is locked until
the scenario is running, or until its configuration is applied for a mode marked
`needs: applied`. **Break it** runs the `trigger` steps, marks the diagram node,
and streams the `logs` step's output into the card and behind that node's logs
button. **Reset** runs the `reset` steps and leaves the card open on the
restarted app's log, so the recovery can be watched too. One failure is in effect
at a time.

```yaml
failure_modes:
  - id: active-crashes
    title: The active ledger crashes
    breaks: One line on what goes wrong.
    watch: What to look for once it has.
    why: Why it breaks. Shown in a closed shade on the card.
    real_world: Where you would meet this in production. Same shade.
    docs:                          # links at the bottom of the card
      - label: "Go API: consuming persistent messages"
        url: https://docs.solace.com/API/API-Developer-Guide-Go/Go-PM-Subscribe.htm
    node: {role: active}           # diagram node to mark: a node id, or a role
    logs: {role: standby}          # whose output tells the story: an action id, or a role
    needs: running                 # or applied, for a scenario with no apps
    trigger:                       # start: and stop: return at once;
      - run: kill-active           # run: waits for the step to finish
    reset:
      - start: ledger-a            # starting one that already runs is skipped
      - start: ledger-b
    expect:                        # checked by smoke.py, ignored by the page
      break:
        logs:
          - log: {role: standby}
            has: ["ACTIVE: the broker is delivering", "applied posting"]
            lacks: ["GAP"]
```

A start of a step that is already running is skipped, so a trigger can make sure
something is up first. `node:` and `logs:` can also name a role,
`{role: active}` or `{role: standby}`, for apps whose role the broker decides;
the page resolves it from the broker when Break it is pressed (see
CONTRIBUTING.md). An action that exists only to cause a failure is marked
`failure_only: true`, which keeps it out of the Actions flowchart.
`scripts/check_scenarios.py` checks that every step, node, `logs` action and docs
link in a failure mode is valid.

## Scenarios

### `exclusive-consumer`

A payments service publishes ledger postings to one exclusive queue. Two
instances of the ledger bind it; the broker delivers to the first and keeps
the second on standby. Each ledger prints ACTIVE or STANDBY from the Go API's
activation and passivation listener, and the diagram reads the same thing
from the broker (`flowOn`). One card kills the active ledger so the standby
takes over with the unacknowledged postings redelivered; the other stalls it
without disconnecting, so nothing takes over until it is stopped.

Every app is one Go binary, built by `apps/build.sh` when the container is
created and again by Play. The "Build the apps" action is only needed to
rebuild after changing the Go code.

## When state and broker drift apart

Deleting a queue by hand is not a problem: terraform compares the broker against
its state file and recreates whatever is missing on the next apply. That is the
demonstration, not the failure mode.

The failure mode is terraform losing its **state file** while the broker keeps
the objects, which happens on a fresh Codespace pointed at an already-configured
broker, or after `reset` is used without destroying first. Apply then fails with
`ALREADY_EXISTS`, because it tries to create things that are already there.

Every scenario ships a **Reconcile with broker** action for that: it imports the
existing objects back into state and leaves the broker untouched. Run it, then
apply again. It is safe to run repeatedly.


## Teardown

Attendees jump between sections out of order, and a stale process from an
earlier section is the most confusing failure mode in a workshop like this.
Three escape hatches exist:

- **Stop everything** in the top bar terminates every running process and
  cancels any Play still working through its steps.
- **Clear broker config** in the top bar stops everything, then deletes every
  queue, client profile, ACL profile and client username that any scenario
  lists in its `tf/imports.tsv`, and removes every scenario's terraform state.
  It works whether or not terraform still has its state, and leaves broker
  defaults and anything created by hand alone. Pressing Play on a scenario sets
  it up again from scratch.
- **Reset this scenario** stops that scenario's processes and deletes its
  terraform state. It does not remove configuration already on the broker; run
  the scenario's destroy action first if you want a clean broker.
