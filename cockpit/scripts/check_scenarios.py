#!/usr/bin/env python3
"""Check every scenario descriptor for the mistakes that only show up mid-workshop.

A scenario.yaml is data, so a typo in it is not caught by any compiler and
usually surfaces as a button that does nothing while an attendee is watching.
This checks the references that have to line up: run and cleanup steps naming
real actions, diagram nodes pointing at real actions, diagram flows that go
through the one broker, failure modes naming real actions, process commands
whose scripts exist, terraform directories that are present, and an
imports.tsv for every scenario offering a reconcile action.

Run from anywhere:  python3 cockpit/scripts/check_scenarios.py
"""

import pathlib
import re
import sys

import yaml

COCKPIT = pathlib.Path(__file__).resolve().parent.parent
SCENARIOS = COCKPIT / "scenarios"


def check(path: pathlib.Path) -> list[str]:
    bad = []
    spec = yaml.safe_load(path.read_text())
    sid = spec.get("id", path.parent.name)
    here = path.parent

    if sid != here.name:
        bad.append(f"id {sid!r} does not match directory {here.name!r}")

    actions = {a["id"]: a for a in spec.get("actions", [])}

    # A colon followed by a space turns a line of plain text into a mapping,
    # which the page then shows as "[object Object]". Quote such lines.
    for key in ("title", "eyebrow", "summary"):
        if key in spec and not isinstance(spec[key], str):
            bad.append(f"{key} is not plain text; quote it if it contains ': '")
    for i, line in enumerate(spec.get("objectives", []) or []):
        if not isinstance(line, str):
            bad.append(f"objective {i + 1} is not plain text; quote it if it contains ': '")
    for mode in spec.get("failure_modes", []) or []:
        for key in ("title", "breaks", "watch", "why", "real_world"):
            if key in mode and not isinstance(mode[key], str):
                bad.append(f"failure mode {mode.get('id', '?')!r} {key} is not plain text")

    for step in spec.get("run", []) + spec.get("cleanup", []):
        if step not in actions:
            bad.append(f"run/cleanup step {step!r} is not an action")

    for node in spec.get("diagram", {}).get("nodes", []):
        if node.get("action") and node["action"] not in actions:
            bad.append(f"diagram node {node['id']!r} points at missing action {node['action']!r}")

    node_ids = {n["id"] for n in spec.get("diagram", {}).get("nodes", [])}
    for flow in spec.get("diagram", {}).get("flows", []):
        for end in ("from", "to"):
            if flow.get(end) not in node_ids:
                bad.append(f"diagram flow {end} {flow.get(end)!r} is not a node")

    # Events always go through the broker: one broker per diagram, and every
    # flow either arrives at it or leaves it. An app that consumes and then
    # publishes draws one flow in from the broker and one flow back.
    nodes = spec.get("diagram", {}).get("nodes", [])
    brokers = {n["id"] for n in nodes if n.get("kind") == "broker"}
    if nodes and len(brokers) != 1:
        bad.append(f"diagram has {len(brokers)} broker nodes, expected exactly 1")
    for flow in spec.get("diagram", {}).get("flows", []):
        if brokers and not ({flow.get("from"), flow.get("to")} & brokers):
            bad.append(f"diagram flow {flow.get('from')!r} -> {flow.get('to')!r} bypasses the broker")

    # A failure mode is steps on actions that must exist, plus an optional
    # diagram node to mark while it is in effect.
    for mode in spec.get("failure_modes", []):
        mid = mode.get("id", "?")
        for key in ("id", "title", "trigger", "reset"):
            if not mode.get(key):
                bad.append(f"failure mode {mid!r} has no {key}")
        for phase in ("trigger", "reset"):
            for op in mode.get(phase, []) or []:
                verb, target = next(iter(op.items())) if len(op) == 1 else (None, None)
                if verb not in ("start", "stop", "run"):
                    bad.append(f"failure mode {mid!r} {phase} step {op!r} is not start:, stop: or run:")
                elif target not in actions:
                    bad.append(f"failure mode {mid!r} {phase} names missing action {target!r}")
        for doc in mode.get("docs", []) or []:
            if not doc.get("label") or not str(doc.get("url", "")).startswith("https://"):
                bad.append(f"failure mode {mid!r} has a docs entry without a label and https url")
        # node: and logs: name a fixed node or action, or a role such as
        # {role: active}, resolved when Break it is pressed among the nodes
        # whose flowOn matches. A role needs at least one such node.
        for key in ("node", "logs"):
            ref = mode.get(key)
            if isinstance(ref, dict):
                if ref.get("role") not in ("active", "standby"):
                    bad.append(f"failure mode {mid!r} {key} role must be active or standby")
                candidates = [n for n in spec.get("diagram", {}).get("nodes", [])
                              if n.get("flowOn") and (not ref.get("queue")
                                                      or n["flowOn"].get("queue") == ref["queue"])]
                if not candidates:
                    bad.append(f"failure mode {mid!r} {key} names a role, but no diagram node has a matching flowOn")
                if key == "logs" and any(not n.get("action") for n in candidates):
                    bad.append(f"failure mode {mid!r} logs role includes a node with no action")
        bad.extend(check_expect(mid, mode.get("expect"), actions, spec))
        if isinstance(mode.get("logs"), str) and mode["logs"] not in actions:
            bad.append(f"failure mode {mid!r} logs names missing action {mode['logs']!r}")
        if mode.get("needs", "running") not in ("running", "applied"):
            bad.append(f"failure mode {mid!r} needs {mode['needs']!r}, expected running or applied")
        if isinstance(mode.get("node"), str) and mode["node"] not in node_ids:
            bad.append(f"failure mode {mid!r} marks missing diagram node {mode['node']!r}")

    boxes = {c.get("id") for n in spec.get("diagram", {}).get("nodes", [])
             for c in n.get("contains", []) or []}
    for flow in spec.get("diagram", {}).get("flows", []) or []:
        if flow.get("via") and flow["via"] not in boxes:
            bad.append(f"diagram flow via {flow['via']!r} is not a box the broker contains")
        src = flow.get("source")
        for s in ([src] if isinstance(src, str) else src or []):
            if s not in node_ids:
                bad.append(f"diagram flow source {s!r} is not a node")

    # flowOn names the queue a consumer node binds and the client name it
    # connects with, so the dashboard can ask the broker whether it is active.
    for node in spec.get("diagram", {}).get("nodes", []):
        flow_on = node.get("flowOn")
        if flow_on is not None and not (isinstance(flow_on, dict)
                                        and flow_on.get("queue") and flow_on.get("client")):
            bad.append(f"diagram node {node['id']!r} flowOn needs queue: and client:")

    live = spec.get("diagram", {}).get("liveWhen")
    if live and live not in node_ids:
        bad.append(f"diagram liveWhen {live!r} is not a node")

    for action in actions.values():
        if action.get("kind") == "terraform":
            tf = here / action.get("tf_dir", "tf")
            if not tf.is_dir():
                bad.append(f"action {action['id']!r} needs missing terraform dir {tf}")
        if action.get("kind") == "process":
            cmd = action.get("cmd", [])
            if not cmd:
                bad.append(f"action {action['id']!r} has no cmd")
                continue
            # Resolve the script a bash command runs, relative to cwd.
            if cmd[0] == "bash" and len(cmd) > 1:
                target = (here / action.get("cwd", ".") / cmd[1]).resolve()
                if not target.exists():
                    bad.append(f"action {action['id']!r} runs missing script {cmd[1]}")

    if "reconcile" in actions and not (here / "tf" / "imports.tsv").exists():
        bad.append("offers reconcile but has no tf/imports.tsv")

    return [f"{sid}: {m}" for m in bad]


def check_expect(mid, expect, actions, spec) -> list:
    """A failure mode's optional expect: block, which smoke.py checks:

        expect:
          break:                 # after the trigger; reset: after the reset
            within: 15           # seconds, optional
            logs:
              - log: <action id> | {role: active|standby, queue: optional}
                has: [regex, ...]     # must appear within the window
                lacks: [regex, ...]   # must not appear during it
    """
    if expect is None:
        return []
    bad = []
    where = f"failure mode {mid!r} expect"
    if not isinstance(expect, dict) or not expect:
        return [f"{where} must map break: and/or reset: to checks"]
    flow_nodes = [n for n in spec.get("diagram", {}).get("nodes", []) if n.get("flowOn")]
    for phase, part in expect.items():
        if phase not in ("break", "reset"):
            bad.append(f"{where} has {phase!r}; only break and reset are checked")
            continue
        if not isinstance(part, dict) or not isinstance(part.get("logs"), list) or not part["logs"]:
            bad.append(f"{where} {phase} needs a non-empty logs: list")
            continue
        within = part.get("within", 15)
        if not isinstance(within, (int, float)) or within <= 0:
            bad.append(f"{where} {phase} within must be a positive number of seconds")
        for i, entry in enumerate(part["logs"], 1):
            at = f"{where} {phase} entry {i}"
            if not isinstance(entry, dict):
                bad.append(f"{at} must be a mapping")
                continue
            ref = entry.get("log")
            if isinstance(ref, str):
                if ref not in actions:
                    bad.append(f"{at} log names missing action {ref!r}")
            elif isinstance(ref, dict):
                if ref.get("role") not in ("active", "standby"):
                    bad.append(f"{at} log role must be active or standby")
                if not [n for n in flow_nodes
                        if not ref.get("queue") or n["flowOn"].get("queue") == ref["queue"]]:
                    bad.append(f"{at} log names a role, but no diagram node has a matching flowOn")
            else:
                bad.append(f"{at} needs log: (an action id or a role)")
            if not entry.get("has") and not entry.get("lacks"):
                bad.append(f"{at} needs has: or lacks:")
            for key in ("has", "lacks"):
                pats = entry.get(key, [])
                if not isinstance(pats, list) or not all(isinstance(p, str) for p in pats):
                    bad.append(f"{at} {key} must be a list of strings")
                    continue
                for pat in pats:
                    try:
                        re.compile(pat)
                    except re.error as exc:
                        bad.append(f"{at} {key} pattern {pat!r} is not a valid regex: {exc}")
    return bad


def main() -> int:
    problems = []
    found = sorted(SCENARIOS.glob("*/scenario.yaml"))
    for path in found:
        problems.extend(check(path))

    for p in problems:
        print(p)
    print(f"\nChecked {len(found)} scenarios, {len(problems)} problem(s).")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
