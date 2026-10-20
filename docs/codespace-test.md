# Codespace test checklist

A checklist to run in a real GitHub Codespace before the workshop is used.
Everything so far was tested on macOS with Docker Desktop; these are the
things only a Codespace can confirm. Write the numbers you measure next to
each item.

Machine: the devcontainer asks for 4 cores, 16 GB memory, 32 GB storage.

## 1. Start

- [ ] Open the repository in a new Codespace (the README badge; it still
      points at the placeholder `SolaceDev/eda-patterns-workshop` until the
      final repository name is set).
- [ ] Note the time you press **Create codespace**: ________
- [ ] Note the time the dashboard tab opens on its own: ________
      (port 3000 is set to open the browser when it is forwarded)
- [ ] Startup breakdown, from a terminal once it is up:

  ```bash
  # The broker step: configureEnv.sh prints its own duration.
  grep -h "Total execution time" /workspaces/.codespaces/.persistedshare/creation.log 2>/dev/null
  # When the broker container started, and when the dashboard was ready.
  docker inspect -f '{{.State.StartedAt}}' solace_10.26.0
  grep -m1 "Application startup complete" cockpit/.state/cockpit.log
  ```

- [ ] Go build time (the image builds the apps in `updateContentCommand`;
      a rebuild from a cold cache is what an attendee waits for if the
      prebuild is missing):

  ```bash
  go clean -cache && time bash cockpit/apps/build.sh
  ```

  Cold build: ________ s. Second run (warm): ________ s.

## 2. Ports

Open the **Ports** panel.

- [ ] 3000 "Solace Workshop Dashboard": forwarded, visibility **Private**,
      opened in the browser.
- [ ] 8080 "Solace Broker Manager": forwarded, **Private**. Opens the broker
      UI (admin / admin).
- [ ] 1883, 9000, 55555, if listed: **Private**, and no pop-up asked to open
      them. Nothing needs them forwarded: the apps run inside the Codespace.
- [ ] Nothing is **Public**. Visibility cannot be set in devcontainer.json;
      it is private unless someone changes it in this panel.

## 3. Memory

With nothing running, then with one scenario playing:

```bash
free -h
docker stats --no-stream --format '{{.Name}} {{.MemUsage}}' solace_10.26.0
ps -o rss=,args= -C python3 | grep -i uvicorn      # the dashboard (RSS in KB)
ps -o rss=,args= -e | grep "bin/workshop" | grep -v grep | awk '{s+=$1} END {print s/1024 " MB in apps"}'
```

| | Idle | One scenario playing |
| --- | --- | --- |
| Total used (`free -h`) | | |
| Broker container | | |
| Dashboard | | |
| Workshop apps | | |

## 4. One scenario from each category

For each: Play, every **Break it** and **Reset**, **Cleanup**, in the
browser. Check that the page lands on the diagram after Break it, the bar at
the top shows the card, and the logs read the way the card says.

- [ ] Event Generation: **Event Sourcing** (replay log on this broker)
- [ ] Communication: **Request-Reply**
- [ ] Consumption: **Event Filtering**
- [ ] Consumer Scalability: **Partitioning** (lanes and moving dots)
- [ ] Deployment: **Event Gateway** (section 5) and **Event Bridge** (the
      line to the store stops while the bridge is down)
- [ ] Error Handling: **Dead Message Queue**
- [ ] Governance: **Intermediated Governance**
- [ ] Migration: **Strangler**

The same, unattended (dashboard on, no dashboard tab open):

```bash
for s in event-sourcing request-reply event-filtering partitioning event-gateway event-bridge dead-letter-queue intermediated-governance strangler; do
  ${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py $s | tail -1
done
```

## 5. Event Gateway: the REST delivery point's route

The broker runs in Docker inside the Codespace (docker-in-docker), and posts
to billing's webhook, which runs in the Codespace itself, at
`host.docker.internal:18099`. `setup_broker.sh` adds that name with
`--add-host host.docker.internal:host-gateway`.

- [ ] The broker container has the route:

  ```bash
  docker inspect -f '{{.HostConfig.ExtraHosts}}' solace_10.26.0   # [host.docker.internal:host-gateway]
  docker exec solace_10.26.0 getent hosts host.docker.internal     # an address, not empty
  ```

  If the container predates this change (no ExtraHosts), recreate it:
  `docker rm -f solace_10.26.0 && bash setup_broker.sh`, then Play again.

- [ ] Play **Event Gateway**. Billing's log shows
      "POST /billing/orders from the broker: O-... billed".
- [ ] On the broker, the REST consumer is up:

  ```bash
  curl -s -u admin:admin "http://localhost:8080/SEMP/v2/monitor/msgVpns/default/restDeliveryPoints/rdp-gw-billing/restConsumers" \
    | python3 -c 'import json,sys; [print(c["restConsumerName"], "up:", c["up"], c.get("lastFailureReason")) for c in json.load(sys.stdin)["data"]]'
  ```

- [ ] If it is not up: note `lastFailureReason`, and check the webhook is
      listening on all interfaces (`ss -ltnp | grep 18099`).

## 6. Whole workshop

- [ ] **Clear broker config** from the top bar: it reports objects deleted
      and nothing failed, and Solace Broker Manager shows only the `default`
      VPN, no bridges, no replay logs.
- [ ] Optional, about two hours: every scenario's smoke test.

  ```bash
  for s in $(ls cockpit/scenarios); do
    ${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py $s | tail -1
  done
  ```

## 7. What to send back

The times from section 1, the memory table from section 3, any scenario
that failed in section 4 with its smoke output, and the result of section 5.
