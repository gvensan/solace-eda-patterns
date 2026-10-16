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
| 350 | [Exclusive Consumer](guides/350-exclusive-consumer.md) | Consumption Patterns | One active consumer and a standby on an exclusive queue, failover, and the hang it cannot detect |
| 999 | [Cleanup](guides/999-cleanup.md) | | Tearing it down |

## What you get in the container

| Thing | Where |
| --- | --- |
| Solace Workshop Dashboard | Port 3000 (the **Ports** tab in a Codespace, <http://localhost:3000> locally) |
| Solace Broker Manager | Port 8080, `admin` / `admin`, or the link in the dashboard sidebar |
| Broker messaging (SMF) | `localhost:55555` |
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
