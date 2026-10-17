#!/usr/bin/env python3
"""Drive scenarios through the dashboard API the way an attendee would.

For each scenario: Play, every failure mode's Break it and Reset, Cleanup,
Clear broker config, then Play again from an empty broker and Cleanup. Steps
in a trigger or reset run with the same rules as the page (cockpit.js
runSteps): start: and stop: return at once, run: waits for the step to finish,
and a start of something already running is skipped.

It checks what can be checked mechanically (every Play step reaches running
or succeeded, Cleanup's steps succeed, Clear reports no failures), checks
the scenario's expect: play: block once Play has finished, checks each
failure mode's expect: block (log lines that must, and must not, appear
after Break it and after Reset, and for a partitioned queue what the
dashboard's /partitions answer shows), and prints the tail of each card's log so a
person can also read whether the failure and the recovery looked right. Any
failed check makes it exit non-zero.

Run with the dashboard's virtualenv, which has httpx and websockets:

    ${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/smoke.py [scenario-id ...]

COCKPIT_URL overrides http://localhost:3000. --set <name>=<value> chooses a
scenario setting (for example --set pace=busy) before Play; every scenario
that has that setting runs with it, and it is put back to its default after.

Two things make a run meaningless, and fail it: another client changing
the scenarios under test meanwhile (a Break it pressed in an open tab), or
clearing the broker or stopping everything,
and the machine sleeping, which stops the dashboard's timers while the
clock this script waits on keeps going. Close the dashboard's tabs, and on
a laptop run it under `caffeinate -i`.
"""

import asyncio
import json
import os
import re
import sys
import time

import httpx
import websockets

BASE = os.environ.get("COCKPIT_URL", "http://localhost:3000")
WS = BASE.replace("http", "ws", 1)

# How long to let a failure, and then its recovery, play out before reading
# the log. Long enough for a handover and a few messages after it.
BREAK_SECONDS = float(os.environ.get("SMOKE_BREAK_SECONDS", "12"))
RESET_SECONDS = float(os.environ.get("SMOKE_RESET_SECONDS", "12"))
LOG_TAIL = int(os.environ.get("SMOKE_LOG_TAIL", "12"))

problems: list = []

# Sent with every request, so the dashboard's change log tells this run's
# requests from anyone else's.
CLIENT = f"smoke-{os.getpid()}"
# More than this many seconds of sleep anywhere in the run fails it.
SLEEP_TOLERANCE = 5.0


def say(msg: str) -> None:
    print(msg, flush=True)


def fail(msg: str) -> None:
    problems.append(msg)
    say(f"  FAIL {msg}")


async def runs(client) -> dict:
    return (await client.get("/api/runs")).json()


async def run_of(client, sid: str, aid: str):
    return next((r for r in (await runs(client))["runs"] if r["key"] == f"{sid}:{aid}"), None)


async def wait_sequence(client, sid: str, timeout: float = 900) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if sid not in (await runs(client))["sequences"]:
            return
        await asyncio.sleep(1)
    fail(f"{sid}: sequence still running after {timeout}s")


async def wait_step(client, sid: str, aid: str, timeout: float = 300) -> dict:
    deadline = time.time() + timeout
    while time.time() < deadline:
        await asyncio.sleep(1)
        run = await run_of(client, sid, aid)
        if run and run["state"] != "running":
            return run
    fail(f"{sid}: {aid} did not finish within {timeout}s")
    return {}


async def run_steps(client, sid: str, steps) -> None:
    for step in steps or []:
        verb, aid = next(iter(step.items()))
        if verb == "stop":
            await client.post(f"/api/scenarios/{sid}/actions/{aid}/stop")
            continue
        current = await run_of(client, sid, aid)
        if not current or current["state"] != "running":
            resp = await client.post(f"/api/scenarios/{sid}/actions/{aid}/start")
            if resp.status_code >= 400:
                fail(f"{sid}: could not start {aid}: {resp.text[:200]}")
                continue
        if verb == "run":
            await wait_step(client, sid, aid)


async def history(sid: str, aid: str) -> list:
    """The buffered output of one action's current run, read the way the page
    reads it, as {seq, stream, text} lines."""
    async with websockets.connect(f"{WS}/ws/runs/{sid}/{aid}") as ws:
        deadline = time.time() + 3
        while time.time() < deadline:
            msg = json.loads(await asyncio.wait_for(ws.recv(), timeout=3))
            if msg.get("type") == "history":
                return msg["lines"]
            if msg.get("type") == "status" and msg.get("status") is None:
                return []
    return []


async def log_tail(sid: str, aid: str, n: int = LOG_TAIL) -> list:
    return [line["text"] for line in await history(sid, aid)][-n:]


async def check_play(client, scenario) -> bool:
    sid = scenario["id"]
    actions = {a["id"]: a for a in scenario["actions"]}
    resp = await client.post(f"/api/scenarios/{sid}/run")
    if resp.status_code >= 400:
        fail(f"{sid}: Play refused: {resp.text[:200]}")
        return False
    await wait_sequence(client, sid)
    err = (await runs(client))["sequenceErrors"].get(sid)
    if err:
        fail(f"{sid}: Play stopped: {err}")
        return False
    ok = True
    for aid in scenario["runSequence"]:
        run = await run_of(client, sid, aid)
        want = "running" if actions[aid]["longRunning"] else "succeeded"
        state = run["state"] if run else None
        if state != want:
            fail(f"{sid}: after Play {aid} is {state}, expected {want}")
            ok = False
    say(f"  Play: {'every step ' + 'running or succeeded' if ok else 'see failures above'}")
    return ok


async def check_cleanup(client, scenario) -> None:
    sid = scenario["id"]
    await client.post(f"/api/scenarios/{sid}/cleanup")
    await wait_sequence(client, sid)
    for aid in scenario["cleanupSequence"]:
        run = await run_of(client, sid, aid)
        if not run or run["state"] != "succeeded":
            fail(f"{sid}: cleanup step {aid} is {run['state'] if run else None}")
    active = [r["key"] for r in (await runs(client))["runs"]
              if r["key"].startswith(sid + ":") and r["state"] == "running"]
    if active:
        fail(f"{sid}: still running after Cleanup: {active}")
    say("  Cleanup: done")


async def roles_now(client, scenario):
    """A resolver for node and log references, frozen at this moment, as the
    page does when Break it is pressed: a plain name stays as it is, and
    {role: active|standby} becomes whichever node with a flowOn the broker
    reports in that role now. Returns (node_for, action_for)."""
    nodes = scenario.get("diagram", {}).get("nodes", [])
    live = (await client.get(f"/api/scenarios/{scenario['id']}/state")).json()["nodes"]

    def node_for(ref, exclude=None):
        # Same rule as the page: on a shared queue the lowest client name in
        # the role wins, and "other" is an active node other than `exclude`.
        if not isinstance(ref, dict):
            return ref
        want = ("standby",) if ref.get("role") == "standby" else ("active", "stalled")
        skip = exclude if ref.get("role") == "other" else None
        found = sorted((n for n in nodes if n.get("flowOn") and n["id"] != skip
                        and (not ref.get("queue") or n["flowOn"]["queue"] == ref["queue"])
                        and live.get(n["id"], {}).get("flow") in want),
                       key=lambda n: n["flowOn"]["client"])
        return found[0]["id"] if found else None

    def action_for(ref, exclude=None):
        if not isinstance(ref, dict):
            return ref
        node = node_for(ref, exclude)
        return next((n.get("action") for n in nodes if n["id"] == node), None)

    return node_for, action_for


async def snapshot(client, sid: str, aids) -> dict:
    """Where each action's log is now, so a check only reads what comes after:
    the run it belongs to (by start time) and the last line number."""
    out = {}
    for aid in aids:
        run = await run_of(client, sid, aid)
        lines = await history(sid, aid)
        out[aid] = (run or {}).get("startedAt"), max((l["seq"] for l in lines), default=0)
    # Asking also keeps the dashboard sampling the partitions from now on.
    if await partitions_since(client, sid, time.time()) is not None:
        out["__partitions__"] = time.time()
    return out


async def lines_since(client, sid: str, aid: str, snap) -> list:
    """Lines written after the snapshot. A run started since then counts in
    full, since its line numbers start again."""
    started, last = snap.get(aid, (None, 0))
    run = await run_of(client, sid, aid)
    lines = await history(sid, aid)
    if (run or {}).get("startedAt") != started:
        return [l["text"] for l in lines]
    return [l["text"] for l in lines if l["seq"] > last]


async def settle(client, sid: str, part, watched, snap):
    """An expect: phase may say `settle: <seconds>`: lines written in that
    time after the steps ran do not count, for output that legitimately
    lingers (messages already in flight when a reset puts things right).
    Returns the snapshot the checks should read from."""
    if not part or not part.get("settle"):
        return snap
    await asyncio.sleep(float(part["settle"]))
    return await snapshot(client, sid, watched)


async def partitions_since(client, sid: str, since: float):
    resp = await client.get(f"/api/scenarios/{sid}/partitions", params={"since": since})
    return resp.json() if resp.status_code == 200 else None


def judge_partitions(spec, data, since, node_for) -> tuple:
    """(pending, failed) for a partitions: block against one /partitions
    answer. pending lists checks not met yet; failed lists checks that can
    no longer pass (an order reached two consumers under one-consumer)."""
    pending, failed = [], []
    if data is None:
        return ["the dashboard has no partition data"], []
    events = [e for e in data["events"] if e["ts"] > since]
    consumers = {}
    for e in events:
        consumers.setdefault(e["order"], set()).add(e["node"])
    shared = {o: sorted(n) for o, n in consumers.items() if len(n) > 1}
    want = spec.get("orders")
    if want == "one-consumer":
        if shared:
            o = next(iter(shared))
            failed.append(f"order {o} reached {', '.join(shared[o])}, expected one consumer per order")
        elif len(events) < 10:
            pending.append(f"only {len(events)} update(s) seen, too few to judge one consumer per order")
    elif want == "split" and not shared:
        pending.append(f"no order reached more than one consumer ({len(events)} update(s) seen)")
    if spec.get("moved") and not [m for m in data["moves"] if m["ts"] > since]:
        pending.append("no partition changed owner")
    if spec.get("picked") and not [e for e in events if "picked" in e["flags"]]:
        pending.append("no consumer picked an order up part way")
    lanes = data["lanes"]
    if spec.get("owned") == "all":
        unowned = [l["partition"] for l in lanes if not l["owner"]]
        if unowned or not lanes:
            pending.append(f"partition(s) {unowned or 'all'} have no owner")
    for entry in spec.get("owns", []):
        node = node_for(entry["node"])
        n = sum(1 for l in lanes if l["node"] == node)
        lo, hi = entry.get("min", 0), entry.get("max", len(lanes))
        if not lo <= n <= hi:
            pending.append(f"{node} owns {n} partition(s), expected {lo} to {hi}")
    return pending, failed


async def check_expect(client, sid: str, mode_id: str, phase: str, part, action_for, snap,
                       node_for=None) -> None:
    """One phase of an expect: block. Every has: pattern must appear in its
    log within the window, and no lacks: pattern may appear in it during the
    window; every partitions: check must hold by the end of it. Without any
    lacks: or orders: one-consumer, the check ends as soon as everything
    else has been seen."""
    checks = []
    for entry in part.get("logs", []):
        aid = action_for(entry["log"])
        if not aid:
            fail(f"{sid}: {mode_id} {phase}: no app holds {entry['log']}")
            continue
        checks.append((aid, [re.compile(p) for p in entry.get("has", [])],
                       [re.compile(p) for p in entry.get("lacks", [])]))
    window = float(part.get("within", 15))
    deadline = time.time() + window
    parts = part.get("partitions")
    since = snap.get("__partitions__", time.time())
    any_lacks = any(lacks for _, _, lacks in checks) or (parts or {}).get("orders") == "one-consumer"
    missing = {}
    pending = []
    while True:
        if parts:
            pending, broken = judge_partitions(parts, await partitions_since(client, sid, since),
                                               since, node_for or (lambda r: r))
            if broken:
                for b in broken:
                    fail(f"{sid}: {mode_id} {phase}: partitions: {b}")
                return
        missing, found = {}, {}
        for aid, has, lacks in checks:
            text = await lines_since(client, sid, aid, snap)
            missing[aid] = [p.pattern for p in has if not any(p.search(t) for t in text)]
            hits = [t for t in text for p in lacks if p.search(t)]
            if hits:
                found[aid] = hits
        if found:
            for aid, hits in found.items():
                fail(f"{sid}: {mode_id} {phase}: {aid} printed what it must not: {hits[0]!r}")
            return
        done = not any(missing.values()) and not pending
        if (done and not any_lacks) or time.time() >= deadline:
            break
        await asyncio.sleep(1)
    for aid, pats in missing.items():
        for pat in pats:
            fail(f"{sid}: {mode_id} {phase}: {aid} never printed /{pat}/ within {window:g}s")
    for p in pending:
        fail(f"{sid}: {mode_id} {phase}: partitions: {p} within {window:g}s")
    if not any(missing.values()) and not pending:
        say(f"     expect {phase}: passed")


async def show_logs(sid: str, scenario, bound) -> None:
    """The card's log, and the marked node's own log when that is a different
    action, since a failure often shows on one app and its effect on another."""
    node_action = next((n.get("action") for n in scenario.get("diagram", {}).get("nodes", [])
                        if n.get("id") == bound["node"]), None)
    for aid in dict.fromkeys(a for a in (bound["logs"], node_action) if a):
        say(f"     [{aid}]")
        for line in await log_tail(sid, aid):
            say(f"     | {line}")


async def smoke(client, scenario) -> None:
    sid = scenario["id"]
    say(f"\n=== {sid}")
    if not await check_play(client, scenario):
        return

    play = (scenario.get("expect") or {}).get("play")
    if play:
        node_for, action_for = await roles_now(client, scenario)
        watched = {action_for(e["log"]) for e in play.get("logs", [])} - {None}
        snap = await snapshot(client, sid, watched)
        await check_expect(client, sid, "Play", "after Play", play, action_for, snap, node_for)

    for mode in scenario.get("failureModes", []):
        say(f"\n  -- Break it: {mode['title']}")
        # Roles are frozen here, as the page freezes them at Break it, and
        # used for the card and for every expect: entry through Reset.
        node_for, action_for = await roles_now(client, scenario)
        card_node = node_for(mode.get("node"))
        resolve = lambda ref: action_for(ref, card_node)
        resolve_node = lambda ref: node_for(ref, card_node)
        bound = {"node": card_node, "logs": resolve(mode.get("logs"))}
        if (mode.get("node") and not bound["node"]) or (mode.get("logs") and not bound["logs"]):
            fail(f"{sid}: {mode['id']}: no app holds the role it names ({bound})")
            continue
        say(f"     (marks {bound['node']}, shows the log of {bound['logs']})")
        expect = mode.get("expect") or {}
        watched = {resolve(e["log"]) for part in expect.values()
                   for e in part.get("logs", [])} - {None}

        snap = await snapshot(client, sid, watched)
        await run_steps(client, sid, mode.get("trigger"))
        snap = await settle(client, sid, expect.get("break"), watched, snap)
        if "break" in expect:
            await check_expect(client, sid, mode["id"], "after Break it", expect["break"], resolve, snap,
                               resolve_node)
        else:
            await asyncio.sleep(BREAK_SECONDS)
        await show_logs(sid, scenario, bound)

        say(f"  -- Reset: {mode['title']}")
        snap = await snapshot(client, sid, watched)
        await run_steps(client, sid, mode.get("reset"))
        snap = await settle(client, sid, expect.get("reset"), watched, snap)
        if "reset" in expect:
            await check_expect(client, sid, mode["id"], "after Reset", expect["reset"], resolve, snap,
                               resolve_node)
        else:
            await asyncio.sleep(RESET_SECONDS)
        await show_logs(sid, scenario, bound)

    say("")
    await check_cleanup(client, scenario)

    resp = (await client.post("/api/broker/clear")).json()
    if resp.get("failed"):
        fail(f"Clear broker config failed for: {resp['failed']}")
    say(f"  Clear broker config: deleted {resp.get('deleted')} object(s)")

    say("  Play again from an empty broker")
    if await check_play(client, scenario):
        await check_cleanup(client, scenario)


async def watch_for_sleep(slept: list) -> None:
    """The wall clock runs on while the machine sleeps; the monotonic clock
    (on macOS) does not, and neither do the dashboard's timers. A gap
    between the two means the machine slept."""
    wall, mono = time.time(), time.monotonic()
    while True:
        await asyncio.sleep(1)
        gap = (time.time() - wall) - (time.monotonic() - mono)
        if gap > SLEEP_TOLERANCE:
            slept.append(gap)
        wall, mono = time.time(), time.monotonic()


# Changes anywhere that reach every scenario: these count wherever they came
# from. Scenarios never share broker objects, so a change to another
# scenario does not touch the one under test.
GLOBAL_CHANGES = ("/api/broker/clear", "/api/runs/stop-all")


async def check_interference(client, since: float, ids) -> None:
    changes = (await client.get("/api/activity", params={"since": since})).json()["changes"]
    mine = tuple(f"/api/scenarios/{sid}/" for sid in ids)
    others = [c for c in changes if c["client"] != CLIENT
              and (c["path"] in GLOBAL_CHANGES or c["path"].startswith(mine))]
    for c in others[:5]:
        fail(f"another client ({c['client']}) changed something during the run: {c['method']} {c['path']}")
    if len(others) > 5:
        fail(f"... and {len(others) - 5} more changes from other clients")


async def apply_settings(client, scenarios, chosen: dict, default: bool = False) -> None:
    for scenario in scenarios:
        for name, spec in (scenario.get("settings") or {}).items():
            if name in chosen:
                value = spec["default"] if default else chosen[name]
                resp = await client.put(f"/api/scenarios/{scenario['id']}/settings/{name}",
                                        json={"value": value})
                if resp.status_code >= 400:
                    fail(f"{scenario['id']}: setting {name}={value}: {resp.text[:200]}")
                elif not default:
                    say(f"  {scenario['id']}: {name} = {value}")


async def main(ids, chosen_settings) -> int:
    started = time.time()
    slept: list = []
    sleeper = asyncio.create_task(watch_for_sleep(slept))
    async with httpx.AsyncClient(base_url=BASE, timeout=30,
                                 headers={"X-Workshop-Client": CLIENT}) as client:
        scenarios = (await client.get("/api/scenarios")).json()["scenarios"]
        chosen = [s for s in scenarios if not ids or s["id"] in ids]
        missing = set(ids) - {s["id"] for s in chosen}
        for sid in sorted(missing):
            fail(f"no scenario {sid!r}")
        await apply_settings(client, chosen, chosen_settings)
        try:
            for scenario in chosen:
                await smoke(client, scenario)
        finally:
            await apply_settings(client, chosen, chosen_settings, default=True)
        await check_interference(client, started, [s["id"] for s in chosen])
    sleeper.cancel()
    if slept:
        fail(f"the machine slept for {sum(slept):.0f}s during the run, so timings are meaningless; "
             "run it again under caffeinate -i")

    say(f"\nSmoke tested {len(chosen)} scenario(s), {len(problems)} problem(s).")
    return 1 if problems else 0


if __name__ == "__main__":
    args, settings = [], {}
    argv = sys.argv[1:]
    while argv:
        arg = argv.pop(0)
        if arg == "--set" and argv:
            name, _, value = argv.pop(0).partition("=")
            settings[name] = value
        else:
            args.append(arg)
    sys.exit(asyncio.run(main(args, settings)))
