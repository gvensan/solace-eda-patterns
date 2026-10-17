# CLAUDE.md

A hands-on Solace workshop on event-driven architecture patterns. Each pattern
from the Solace guide to event-driven architecture patterns is one scenario:
a small working system on a local Solace broker that attendees run, break and
inspect. Attendees open it in a Codespace, read `guides/`, and drive scenarios
from the **Solace Workshop Dashboard** on port 3000.

The dashboard, devcontainer and tooling are a copy of the Solace Core
Developer Workshop
(https://github.com/SolaceDev/solace-core-developer-workshops). Keep the
structure, flow and usage the same as that one. `docs/requirements.md` is the
goal and scope of this work; `docs/spikes.md` records what was tested on the
broker and what was found.

[CONTRIBUTING.md](CONTRIBUTING.md) is the full reference for how everything
fits together and for the `scenario.yaml` schema. Read it before making a
structural change. This file is the short version plus the rules to follow.

## Layout

| Path | What it is |
| --- | --- |
| `cockpit/scenarios/<id>/scenario.yaml` | One scenario: page text, actions, diagram, failure modes, inspect views |
| `cockpit/scenarios/<id>/tf/` | Terraform for the broker objects that scenario owns, plus `imports.tsv` |
| `cockpit/apps/scenarios/<pkg>/` | Go roles for that scenario's apps |
| `cockpit/apps/cmd/workshop/main.go` | Registers every role as `"<pkg> <role>"` in one binary |
| `cockpit/apps/internal/solace/` | Shared Go helpers: connect, publish, subscribe, bind queue, logging, freeze on SIGUSR1, payload fields |
| `cockpit/app/` | FastAPI dashboard backend (registry, process manager, SEMP proxy) |
| `cockpit/static/` | Dashboard UI, plain JS and CSS, no build step |
| `cockpit/scripts/` | `check_scenarios.py`, `smoke.py`, `reconcile.sh`, failure mode helpers |
| `guides/NNN-<id>.md` | Attendee guide per scenario, `NNN` matches `order:` |
| `.devcontainer/`, `setup_broker.sh` | Codespace setup and the broker container |
| `docs/` | Requirements and spike findings, not attendee-facing |

The `cockpit/` folder name, the `cockpit.app` module and `COCKPIT_*` variables
are internal names. Attendee-facing text always says "Solace Workshop
Dashboard" or "the dashboard".

## Commands

Run these from the repo root, inside the devcontainer or Codespace, or on a
host with Go, a C toolchain, Terraform and Python (see "Local development").

```bash
python3 cockpit/scripts/check_scenarios.py        # validate every scenario.yaml; run after any change
${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py <id>   # Play, each Break it and Reset, Cleanup, Clear, Play again
bash cockpit/apps/build.sh                        # rebuild the Go binary (run.sh also rebuilds when stale)
./dev.sh start | stop | restart | status | logs | check <id>   # on a host machine (see Local development)
pkill -TERM -f "[u]vicorn cockpit.app"; sleep 4; bash cockpit/start_cockpit.sh   # restart inside a container
bash cockpit/apps/run.sh <pkg> <role> [flags]     # run one app by hand
tail -f cockpit/.state/cockpit.log                # dashboard log
```

The dashboard reloads a changed `scenario.yaml` on its next request, without
a restart and without stopping running apps. Static files are uncached. In
development mode (`COCKPIT_DEV=1`, set by `./dev.sh`) the open page reloads
itself on either, and the server restarts itself when `cockpit/app/` Python
changes, which stops the apps it runs. Go code rebuilds when an app next
starts.

Driving a scenario without the browser (dashboard on localhost:3000):

```bash
curl -X POST localhost:3000/api/scenarios/<id>/run       # Play
curl -X POST localhost:3000/api/scenarios/<id>/pause
curl -X POST localhost:3000/api/scenarios/<id>/cleanup
curl localhost:3000/api/runs                               # state of every action
curl localhost:3000/api/scenarios/<id>/state               # per-node process, config and flow state
curl -X POST localhost:3000/api/broker/clear               # delete every scenario's broker objects
```

If Claude is running on a host machine rather than in the container, wrap the
commands in `docker exec -u vscode <container> bash -c 'cd /workspaces/<repo> && ...'`,
or run the dashboard on the host against a broker in Docker (below).

## Local development

Every part reads the broker from the same `SOLACE_*` variables
(`SOLACE_HOST`, `SOLACE_SEMP_PORT`, `SOLACE_SMF_PORT`, `SOLACE_SEMP_USER`,
`SOLACE_SEMP_PASSWORD`, `SOLACE_MSG_VPN`, `SOLACE_CLIENT_PASSWORD`), with
defaults that match `setup_broker.sh`. On a host machine use `./dev.sh`,
which reads them from `.env.dev` (git-ignored; copy `.env.dev.example`),
starts the broker and the dashboard in development mode, and fetches a pinned
Terraform into `.tools/` if none is on PATH. On a machine where 8080 or 55555
is taken, set `SOLACE_SEMP_PORT=18080`, `SOLACE_SMF_PORT=15555` and
`SOLACE_MAP_EXTRA_PORTS=false` in `.env.dev`.

Never point the dashboard at a broker someone else is using: Clear broker
config deletes every object any scenario lists in `imports.tsv`.

## Rules that must hold

These are what the workshop teaches, and `check_scenarios.py` enforces most
of them.

- **One broker per diagram, and every flow goes through it.** Apps never
  connect to each other. An app that consumes and publishes gets a flow in
  from the broker, a flow back to it, and `shape: processor` (a hexagon). Do
  not build multi-broker scenarios yet; that needs a deliberate checker change.
- **One pattern per scenario.** Where two patterns overlap (Competing
  Consumers and Consumer Groups, say), each keeps its own scenario and its
  guide states the one idea that separates it from its neighbour.
- **Scenarios never share broker objects.** Prefix every name with something
  unique to the scenario (`q.<prefix>.`, `svc-<prefix>-`, `acl-<prefix>`,
  `cp-<prefix>`). Topics start with the same prefix (`excl/...`), so no other
  scenario's queue subscription matches them.
- **`tf/imports.tsv` lists every object the scenario owns**, terraform address
  and import id separated by a tab, `{vpn}` for the VPN. Reconcile and Clear
  broker config depend on it; a missing line means that object is never cleaned
  up.
- **`run:` starts consumers before publishers.** Direct messaging keeps
  nothing for a subscriber that is not connected yet, and on an exclusive
  queue the first to bind is the one the broker makes active.
- **Every scenario offers apply, build (if it has apps), reconcile and
  destroy actions**, following the shapes in existing scenarios. Declare
  `cleanup:` explicitly: the default is every `danger` action, which would
  also run a failure-only helper marked `danger`.
- **Failure modes show real errors.** Apps print the Go API's error as it is,
  with `solace.Errf(role, "%s", err)`. Explanations belong in the card's
  `watch`, `why` and `real_world` text, not in the app's output. Each mode has
  `docs` links: the Go API page first, then the broker documentation. Only use
  URLs you have confirmed exist.
- **Name roles, not instances.** When several instances of one app share a
  job and the broker decides who does what (the active consumer and the
  standby on an exclusive queue, and the like), a failure card acts on the
  role, never on a named instance: "the active ledger crashes", not "Ledger A
  crashes". Its trigger targets whoever holds the role now, through
  `cockpit/scripts/signal_consumer.sh <queue> <active|standby> <SIGNAL>`, and
  its `node:` and `logs:` are `{role: active}`, `{role: standby}` or, for the
  log, `{role: other}` (another active consumer, on a shared queue), which
  the page resolves from the broker when Break it is pressed. Where several
  apps hold a role, the lowest client name is the one hit and marked. Every
  app that takes part passes `--client-name` matching its node's `flowOn`
  (the checker enforces it); `signal_consumer.sh` finds it that way. The card text,
  the guide and the Reset steps use the same role words. Never rearrange the
  apps behind the attendee's back to make a fixed name true: what they see in
  the diagram before pressing Break it is what the card acts on. Reset brings
  back whatever stopped (start every instance; starting one that runs is
  skipped), and it rejoins in whatever role the broker gives it.
- **A crash is a kill, not a stop.** Stop sends SIGTERM, and the apps shut down
  cleanly, which finishes and acknowledges in-flight messages. To show what a
  crash does, send SIGKILL (`signal_consumer.sh ... KILL`, or `pkill -f`
  anchored on the binary path, `bin/workshop <pkg> <role> `, so it cannot
  match anything else). Apps that take part in a role connect with a fixed
  client name (`solace.ConnectAs`) and pass it as `--client-name`.
- **An action used only by a failure mode** is marked `failure_only: true`.
- **Every indicator explains itself.** Anything the page shows as a symbol, a
  count, a colour or a short label (a dot, `1/1`, a badge, a border, a node
  sublabel) gets hover text that says what it means and what to do about it,
  updated with the value it describes. Add it in the same change that adds the
  indicator (`indicator(node, "<name>", text)` in `diagram.js` or
  `cockpit.js`), and add a row to `guides/000-reading-the-dashboard.md` tagged
  `<!-- indicator: <name> -->`. The checker fails if either is missing.
  Scenario text uses the same words the indicator shows.
- **The page never shows something the system is not doing.** If a label or
  an animation could be read as work happening when it is not (packets into a
  stuck consumer, "active" on a frozen one), fix the picture from broker data
  or say so in the card text.
- **Check Solace behaviour on the broker or on docs.solace.com.** The pattern
  guide is thin on broker detail. Do not invent features. Record anything you
  test in `docs/spikes.md`.
- **Credentials stay local**: the broker's `admin` / `admin` and the shared
  client password from `cockpit/app/config.py`. Never add real secrets.

## Common tasks

**Add a scenario.** Copy the closest existing one (`exclusive-consumer` for
queues and consumers). Then: rename the folder and `id:`, pick an `order:`
(category number times 100 plus the pattern's position times 10, following
the pattern guide), rewrite the text, change the terraform object names, topic
prefix and `imports.tsv`, write the Go package and register its roles in
`main.go`, add two failure modes, write `guides/NNN-<id>.md`, add a README
row, and fix the `Next:` links in the guides either side. Check, restart,
run `smoke.py`, then look at the page in a browser.

**Remove a scenario.** Delete its scenario folder and guide, remove its README
row, fix the neighbouring `Next:` link, and remove its Go package and roles.

**Reorder.** Change `order:` values. Keep guide prefixes, the README table and
`Next:` links in the same order.

**Rename.** Prefer changing only `title:`, `eyebrow:`, `summary:`, the guide
heading and the README row. Changing the id means renaming the folder and
every string use of the id (`reconcile.sh <id>`, `forget_state.sh <id>`, guide
names and links); `grep -rn "<old-id>" cockpit guides README.md` finds them.

## Verifying a change

1. `python3 cockpit/scripts/check_scenarios.py` reports 0 problems.
2. If you changed `cockpit/app/` Python outside development mode, restart the
   dashboard (`./dev.sh restart`). Yaml and static changes need no restart.
3. Every failure mode has an `expect:` block (CONTRIBUTING.md) that encodes
   what its `watch:` text promises.
4. `${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py <id>` reports 0
   problems. It runs Play, every Break it and Reset, Cleanup, Clear broker
   config and Play again from an empty broker, checks every `expect:` block,
   and prints each card's log. CI runs the same on Linux for every push.
5. Read those logs too: `expect:` checks the words, a person checks that the
   failure and the recovery read the way the card describes.
6. Look at the page in a browser. The diagram, the flowchart and the Break it
   cards are the product; a passing check is not the same as a page that reads
   well.
7. Do this on a Standard edition broker of the pinned version
   (`setup_broker.sh`), not only on whatever broker you have locally.

## Writing style

Attendee-facing text is `scenario.yaml` text fields, `guides/`, the README and
UI strings.

- Short, plain sentences for someone new to event-driven design. Say what
  happens and why, then what to look at.
- Never use em dashes, en dashes or double hyphens. Use commas, colons,
  parentheses, or split the sentence.
- Write "Solace Agent Mesh" in full, never abbreviated.
- "Solace Broker Manager" for the broker UI, "Solace Workshop Dashboard" or
  "the dashboard" for this app.
- Match the existing comment density in code: comments explain why, not what.
