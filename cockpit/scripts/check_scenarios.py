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

import math
import pathlib
import re
import sys

import yaml

COCKPIT = pathlib.Path(__file__).resolve().parent.parent
SCENARIOS = COCKPIT / "scenarios"
STATIC_JS = COCKPIT / "static" / "js"
READING_GUIDE = COCKPIT.parent / "guides" / "000-reading-the-dashboard.md"

# Queue monitor counters that count messages a queue moved to its dead
# message queue, each for one reason. An arrivals: label must name one of
# these, and no two labels the same one, or a hop would carry the wrong
# reason.
DMQ_COUNTERS = {"maxRedeliveryExceededToDmqMsgCount", "maxTtlExpiredToDmqMsgCount"}

# Flags a diagram's events: block may define, each drawn by diagram.js.
EVENT_FLAGS = {"gap", "order", "picked", "redelivered"}


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
        # A node is drawn at a fixed width; a processor's hexagon cuts text
        # at about this length, and an app box not much later.
        if len(node.get("sublabel", "")) > SUBLABEL_MAX:
            bad.append(f"diagram node {node['id']!r} sublabel is {len(node['sublabel'])} characters; "
                       f"the box shows {SUBLABEL_MAX}")

    # traffic: names a queue the scenario owns, read from its imports.
    owned = (here / "tf" / "imports.tsv").read_text() if (here / "tf" / "imports.tsv").exists() else ""
    for flow in spec.get("diagram", {}).get("flows", []):
        if flow.get("traffic") and f"/{flow['traffic']}\n" not in owned + "\n":
            bad.append(f"diagram flow {flow.get('from')!r} -> {flow.get('to')!r} traffic: "
                       f"{flow['traffic']!r} is not a queue in tf/imports.tsv")

    for a in actions.values():
        if a.get("stands_for") and a["stands_for"] not in actions:
            bad.append(f"action {a['id']!r} stands_for missing action {a['stands_for']!r}")

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
                if ref.get("role") not in ("active", "standby", "other"):
                    bad.append(f"failure mode {mid!r} {key} role must be active, standby or other")
                if ref.get("role") == "other" and key == "node":
                    bad.append(f"failure mode {mid!r} node cannot be role other; it is what other is measured against")
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
        elif flow_on:
            # signal_consumer.sh finds the app by --client-name on its command
            # line, so the node's action has to pass the same name.
            cmd = actions.get(node.get("action"), {}).get("cmd", [])
            want = ["--client-name", flow_on["client"]]
            if not any(cmd[i:i + 2] == want for i in range(len(cmd))):
                bad.append(f"diagram node {node['id']!r} has flowOn client {flow_on['client']!r}, "
                           f"but its action does not pass --client-name {flow_on['client']}")

    bad.extend(check_pattern(spec, here.parent))
    bad.extend(check_partitions(spec, here, actions))
    bad.extend(check_settings(spec, actions, node_ids))
    bad.extend(check_in_flight(spec))

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

    top = spec.get("expect")
    if top is not None:
        if not isinstance(top, dict) or set(top) - {"play"}:
            bad.append("expect: at the top level takes only play:, checked once Play has finished")
        else:
            bad.extend(check_expect("(play)", top, actions, spec))

    if "reconcile" in actions and not (here / "tf" / "imports.tsv").exists():
        bad.append("offers reconcile but has no tf/imports.tsv")

    return [f"{sid}: {m}" for m in bad]


SUBLABEL_MAX = 30
PATTERN_FIELDS = ("what", "when", "cost", "solace")
PATTERN_GUIDE = "https://solace.com/event-driven-architecture-patterns/"
PATTERN_ANCHORS = {
    "ecst", "cqrs", "cdc", "event-sourcing", "publish-subscribe", "point-to-point", "request-reply",
    "streaming", "hierarchical-topics", "event-filtering", "guaranteed-delivery", "backpressure-push",
    "exclusive-consumer", "competing-consumers", "consumer-groups", "shock-absorber", "partitioning",
    "event-bridge", "event-mesh", "event-gateway", "dead-letter-queue", "discard-pause-retry", "saga",
    "event-catalog", "event-apis", "intermediated", "disintermediated", "access-control-authorization",
    "strangler", "data-synchronization",
}


def check_pattern(spec, scenarios_dir) -> list:
    """Every scenario says what its pattern is, beside its own story: the
    page shows the two side by side so the idea and the example are told
    apart. related: names other scenarios, linked from the panel."""
    p = spec.get("pattern")
    if not isinstance(p, dict):
        return ["no pattern: block (what, when, cost, solace, related, guide)"]
    bad = [f"pattern {k}: missing or not plain text" for k in PATTERN_FIELDS
           if not isinstance(p.get(k), str) or not p[k].strip()]
    for rid in p.get("related", []) or []:
        if not (scenarios_dir / rid / "scenario.yaml").exists():
            bad.append(f"pattern related: {rid!r} is not a scenario")
    # The writing rule, enforced where it is easiest to slip: no em dashes,
    # en dashes or double hyphens in the pattern text.
    for k in PATTERN_FIELDS:
        text = p.get(k) if isinstance(p.get(k), str) else ""
        if "\u2014" in text or "\u2013" in text or "--" in text:
            bad.append(f"pattern {k}: has an em dash, en dash or double hyphen; use a comma, colon or parentheses")
    # One link to this pattern's section of the pattern guide. The anchors
    # were read off the page on 2026-10-18; a new one must be checked there.
    guide = p.get("guide", "")
    if not guide.startswith(PATTERN_GUIDE + "#") or guide[len(PATTERN_GUIDE) + 1:] not in PATTERN_ANCHORS:
        bad.append(f"pattern guide: {guide!r} is not {PATTERN_GUIDE}#<one of the guide's section anchors>")
    return bad


def check_expect(mid, expect, actions, spec) -> list:
    """A failure mode's optional expect: block, which smoke.py checks:

        expect:
          break:                 # after the trigger; reset: after the reset
            within: 15           # seconds, optional
            settle: 5            # seconds of output to ignore first, optional
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
        if phase not in (("play",) if mid == "(play)" else ("break", "reset")):
            bad.append(f"{where} has {phase!r}; only break and reset are checked")
            continue
        allowed = ("play",) if mid == "(play)" else ("break", "reset")
        if phase not in allowed:
            continue
        if not isinstance(part, dict) or not (part.get("logs") or part.get("partitions") or part.get("boxes")):
            bad.append(f"{where} {phase} needs a non-empty logs: list, partitions: or boxes: checks")
            continue
        if "partitions" in part:
            bad.extend(check_partition_expect(f"{where} {phase}", part["partitions"], spec))
        if "boxes" in part:
            bad.extend(check_boxes_expect(f"{where} {phase}", part["boxes"], spec))
        if not part.get("logs"):
            continue
        if not isinstance(part["logs"], list):
            bad.append(f"{where} {phase} logs: must be a list")
            continue
        within = part.get("within", 15)
        if not isinstance(within, (int, float)) or within <= 0:
            bad.append(f"{where} {phase} within must be a positive number of seconds")
        settle = part.get("settle", 0)
        if not isinstance(settle, (int, float)) or settle < 0:
            bad.append(f"{where} {phase} settle must be a number of seconds")
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
                if ref.get("role") not in ("active", "standby", "other"):
                    bad.append(f"{at} log role must be active, standby or other")
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


def partition_box(spec):
    """The (node, child) whose child is drawn as partitions, or None."""
    for node in spec.get("diagram", {}).get("nodes", []) or []:
        for child in node.get("contains", []) or []:
            if child.get("partitions") is not None:
                return node, child
    return None


def check_settings(spec, actions, node_ids) -> list:
    """settings: choices an attendee makes about how an app runs.

        settings:
          pace:
            label: Pace
            node: <diagram node whose panel shows it>
            default: <a choice value>
            choices:
              - {value: clear, label: Clear, sublabel: ..., description: ..., args: [--rate, "2"]}

    An action uses one by putting "{setting:pace}" in its cmd."""
    bad = []
    settings = spec.get("settings") or {}
    if not isinstance(settings, dict):
        return ["settings: must map names to settings"]
    used = {part[len("{setting:"):-1] for a in actions.values() for part in a.get("cmd", []) or []
            if isinstance(part, str) and part.startswith("{setting:") and part.endswith("}")}
    for name in sorted(used - set(settings)):
        bad.append(f"an action's cmd uses {{setting:{name}}}, which is not under settings:")
    nodes = {n["id"]: n for n in spec.get("diagram", {}).get("nodes", []) or []}
    for name, st in settings.items():
        where = f"setting {name!r}"
        if not isinstance(st, dict):
            bad.append(f"{where} must be a mapping")
            continue
        if name not in used:
            bad.append(f"{where} is used by no action's cmd")
        for key in ("label", "node", "default"):
            if not isinstance(st.get(key), str):
                bad.append(f"{where} needs {key}:")
        if st.get("node") not in node_ids:
            bad.append(f"{where} node {st.get('node')!r} is not a diagram node")
        else:
            action = actions.get(nodes[st["node"]].get("action"), {})
            if f"{{setting:{name}}}" not in (action.get("cmd") or []):
                bad.append(f"{where} is shown on {st['node']!r}, whose action does not use it")
        choices = st.get("choices") or []
        values = [c.get("value") for c in choices if isinstance(c, dict)]
        if len(choices) < 2 or len(values) != len(choices):
            bad.append(f"{where} needs two or more choices")
        if st.get("default") not in values:
            bad.append(f"{where} default {st.get('default')!r} is not one of its choices")
        for c in choices:
            if not isinstance(c, dict):
                continue
            for key in ("value", "label", "sublabel", "description"):
                if not isinstance(c.get(key), str):
                    bad.append(f"{where} choice {c.get('value')!r} needs {key}: as plain text")
            if not isinstance(c.get("args"), list) or not all(isinstance(a, str) for a in c["args"]):
                bad.append(f"{where} choice {c.get('value')!r} args: must be a list of strings")
    return bad


def check_partitions(spec, here, actions) -> list:
    """A box drawn as partition lanes, and the events: that put dots on them.

        contains:
          - id: partitioned
            partitions: {queue: <name>, count: <lanes>}
        events:
          pattern: <regex with named groups order, seq; optional key, msg, status>
          flags: {gap|order|picked|redelivered: <regex>}
    """
    bad = []
    diagram = spec.get("diagram", {}) or {}
    boxes = [c for n in diagram.get("nodes", []) or [] for c in n.get("contains", []) or []
             if c.get("partitions") is not None]
    if len(boxes) > 1:
        bad.append("diagram draws more than one partitioned queue; the lanes support one")
    for box in boxes:
        part = box["partitions"]
        if not isinstance(part, dict) or not part.get("queue") or not isinstance(part.get("count"), int) \
                or part["count"] < 1:
            bad.append(f"diagram box {box.get('id')!r} partitions: needs queue: and a count: of 1 or more")
            continue
        consumers = [n for n in diagram.get("nodes", []) if (n.get("flowOn") or {}).get("queue") == part["queue"]]
        if not consumers:
            bad.append(f"diagram box {box.get('id')!r} draws {part['queue']} as lanes, but no node has a flowOn on it")
        # The lanes are laid out from count:, so it must match what terraform creates.
        for tf in sorted((here / "tf").glob("*.tf")):
            for block in re.findall(r'resource\s+"solacebroker_msg_vpn_queue"\s+"[^"]+"\s*\{(.*?)\n\}', tf.read_text(), re.S):
                name = re.search(r'queue_name\s*=\s*"([^"]+)"', block)
                count = re.search(r'partition_count\s*=\s*(\d+)', block)
                if name and name.group(1) == part["queue"]:
                    if not count:
                        bad.append(f"{tf.name}: {part['queue']} has no partition_count, but the diagram draws {part['count']} lanes")
                    elif int(count.group(1)) != part["count"]:
                        bad.append(f"diagram draws {part['count']} lanes for {part['queue']}, "
                                   f"but {tf.name} creates {count.group(1)} partitions")
    # depth: and arrivals: on a queue box, read from the broker on each poll.
    all_boxes = {c.get("id"): c for n in diagram.get("nodes", []) or [] for c in n.get("contains", []) or []}
    for box in all_boxes.values():
        depth = box.get("depth")
        if depth is not None and not (isinstance(depth, dict) and depth.get("queue") and depth.get("unit")):
            bad.append(f"diagram box {box.get('id')!r} depth: needs queue: and unit:")
        arr = box.get("arrivals")
        if arr is None:
            continue
        if not isinstance(arr, dict) or not arr.get("queue") or not isinstance(arr.get("reasons"), dict) \
                or not arr["reasons"]:
            bad.append(f"diagram box {box.get('id')!r} arrivals: needs queue: and reasons: (label: counter)")
            continue
        if arr.get("from") not in all_boxes or arr.get("from") == box.get("id"):
            bad.append(f"diagram box {box.get('id')!r} arrivals from: must name another box")
        for label, field in arr["reasons"].items():
            if field not in DMQ_COUNTERS:
                bad.append(f"diagram box {box.get('id')!r} arrivals reason {label!r} names {field!r}, "
                           f"not one of the DMQ counters {sorted(DMQ_COUNTERS)}")
        fields = list(arr["reasons"].values())
        for field in sorted({f for f in fields if fields.count(f) > 1}):
            bad.append(f"diagram box {box.get('id')!r} arrivals has two labels on {field}; a hop would carry either")

    events = diagram.get("events")
    if events is not None:
        if not boxes:
            bad.append("diagram events: needs a box drawn as partitions to put the dots on")
        if not isinstance(events, dict) or not isinstance(events.get("pattern"), str):
            bad.append("diagram events: needs a pattern:")
        else:
            try:
                groups = set(re.compile(events["pattern"]).groupindex)
                for g in ("order", "seq"):
                    if g not in groups:
                        bad.append(f"diagram events pattern has no (?P<{g}>...) group")
            except re.error as exc:
                bad.append(f"diagram events pattern is not a valid regex: {exc}")
            for name, pat in (events.get("flags") or {}).items():
                if name not in EVENT_FLAGS:
                    bad.append(f"diagram events flag {name!r} is not one of {sorted(EVENT_FLAGS)}")
                try:
                    re.compile(pat)
                except (re.error, TypeError) as exc:
                    bad.append(f"diagram events flag {name!r} is not a valid regex: {exc}")
    return bad


def check_boxes_expect(where, checks, spec) -> list:
    """boxes: in an expect: phase: how much each arrivals: counter on a box
    must rise during the window, which smoke.py reads from /state.

        boxes:
          dmq: {expired: 3, failed 3 attempts: 0}
          dmq: {expired: {min: 100}}     # when the count depends on timing
    """
    bad = []
    arrivals = {c.get("id"): c.get("arrivals") for n in spec.get("diagram", {}).get("nodes", []) or []
                for c in n.get("contains", []) or []}
    if not isinstance(checks, dict) or not checks:
        return [f"{where} boxes: must map box ids to counts"]
    for box, want in checks.items():
        spec_arr = arrivals.get(box)
        if not spec_arr:
            bad.append(f"{where} boxes: {box!r} is not a box with arrivals:")
            continue
        if not isinstance(want, dict) or not want:
            bad.append(f"{where} boxes: {box!r} must map reasons to counts")
            continue
        for reason, n in want.items():
            if reason not in spec_arr.get("reasons", {}):
                bad.append(f"{where} boxes: {box!r} has no reason {reason!r}")
            if isinstance(n, dict):
                if not n or set(n) - {"min", "max"} or not all(isinstance(v, int) and v >= 0 for v in n.values()):
                    bad.append(f"{where} boxes: {box!r} {reason!r} must be a count, or min: and max: counts")
            elif not isinstance(n, int) or n < 0:
                bad.append(f"{where} boxes: {box!r} {reason!r} must be a count of 0 or more")
    return bad


def check_partition_expect(where, checks, spec) -> list:
    """partitions: in an expect: phase, which smoke.py checks against the
    dashboard's /partitions answer:

        partitions:
          orders: one-consumer | split   # every order seen reached one consumer, or one reached several
          moved: true                    # some partition changed owner
          picked: true                   # some consumer picked an order up part way
          owned: all                     # every partition has an owner at the end
          owns:                          # how many partitions a node owns at the end
            - {node: <node id> | {role: active|standby|other}, min: 1, max: 2}
    """
    bad = []
    found = partition_box(spec)
    if not found:
        return [f"{where} has partitions: checks, but the diagram draws no partitioned queue"]
    queue = found[1]["partitions"].get("queue")
    if not isinstance(checks, dict) or not checks:
        return [f"{where} partitions: must map checks to values"]
    for key, value in checks.items():
        if key == "orders":
            if value not in ("one-consumer", "split"):
                bad.append(f"{where} partitions orders: must be one-consumer or split")
        elif key in ("moved", "picked"):
            if value is not True:
                bad.append(f"{where} partitions {key}: can only be true")
        elif key == "owned":
            if value != "all":
                bad.append(f"{where} partitions owned: can only be all")
        elif key == "owns":
            if not isinstance(value, list) or not value:
                bad.append(f"{where} partitions owns: must be a list")
                continue
            ids = {n["id"] for n in spec["diagram"]["nodes"] if (n.get("flowOn") or {}).get("queue") == queue}
            for entry in value:
                ref = entry.get("node") if isinstance(entry, dict) else None
                if isinstance(ref, dict):
                    if ref.get("role") not in ("active", "standby", "other"):
                        bad.append(f"{where} partitions owns node role must be active, standby or other")
                elif ref not in ids:
                    bad.append(f"{where} partitions owns node {ref!r} is not a consumer of {queue}")
                if not any(isinstance(entry.get(k), int) for k in ("min", "max")):
                    bad.append(f"{where} partitions owns entry needs min: or max:")
        else:
            bad.append(f"{where} partitions: has unknown check {key!r}")
    return bad


# Event dots and chips are told apart by colour (and a letter). Every pair
# of colours must be at least this far apart in CIEDE2000, with lightness at
# half weight so two shades of one hue do not count as different, and each
# colour this far from the dashboard's dark backgrounds.
MIN_COLOUR_GAP = 20.0
MIN_BACKGROUND_GAP = 45.0
DARK_BACKGROUNDS = ("#03213B", "#0B3A5C", "#061A2B")


def _lab(colour: str) -> tuple:
    """CIE L*a*b* (D65) of a #RRGGBB colour."""
    def linear(c):
        c /= 255
        return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4
    r, g, b = (linear(int(colour[i:i + 2], 16)) for i in (1, 3, 5))
    x = (0.4124 * r + 0.3576 * g + 0.1805 * b) / 0.95047
    y = 0.2126 * r + 0.7152 * g + 0.0722 * b
    z = (0.0193 * r + 0.1192 * g + 0.9505 * b) / 1.08883
    f = lambda t: t ** (1 / 3) if t > 0.008856 else 7.787 * t + 16 / 116
    return 116 * f(y) - 16, 500 * (f(x) - f(y)), 200 * (f(y) - f(z))


def _de2000(c1: str, c2: str, kL: float = 2.0) -> float:
    """CIEDE2000 colour difference. kL above 1 gives lightness less weight."""
    L1, a1, b1 = _lab(c1)
    L2, a2, b2 = _lab(c2)
    cbar = (math.hypot(a1, b1) + math.hypot(a2, b2)) / 2
    g = 0.5 * (1 - math.sqrt(cbar ** 7 / (cbar ** 7 + 25 ** 7)))
    a1, a2 = (1 + g) * a1, (1 + g) * a2
    C1, C2 = math.hypot(a1, b1), math.hypot(a2, b2)
    h1 = math.degrees(math.atan2(b1, a1)) % 360
    h2 = math.degrees(math.atan2(b2, a2)) % 360
    dh = 0.0 if C1 * C2 == 0 else (h2 - h1 + 180) % 360 - 180
    dH = 2 * math.sqrt(C1 * C2) * math.sin(math.radians(dh / 2))
    if C1 * C2 == 0:
        hbar = h1 + h2
    elif abs(h1 - h2) <= 180:
        hbar = (h1 + h2) / 2
    else:
        hbar = (h1 + h2 + 360) / 2 if h1 + h2 < 360 else (h1 + h2 - 360) / 2
    lbar, cbarp = (L1 + L2) / 2, (C1 + C2) / 2
    t = (1 - 0.17 * math.cos(math.radians(hbar - 30)) + 0.24 * math.cos(math.radians(2 * hbar))
         + 0.32 * math.cos(math.radians(3 * hbar + 6)) - 0.20 * math.cos(math.radians(4 * hbar - 63)))
    sl = 1 + 0.015 * (lbar - 50) ** 2 / math.sqrt(20 + (lbar - 50) ** 2)
    sc = 1 + 0.045 * cbarp
    sh = 1 + 0.015 * cbarp * t
    rt = (-math.sin(math.radians(60 * math.exp(-((hbar - 275) / 25) ** 2)))
          * 2 * math.sqrt(cbarp ** 7 / (cbarp ** 7 + 25 ** 7)))
    dl, dc, dhh = (L2 - L1) / (kL * sl), (C2 - C1) / sc, dH / sh
    return math.sqrt(dl ** 2 + dc ** 2 + dhh ** 2 + rt * dc * dhh)


def key_colours() -> list:
    text = (STATIC_JS / "diagram.js").read_text()
    m = re.search(r"const KEY_COLOURS = \[(.*?)\]", text, re.S)
    return re.findall(r'"(#[0-9A-Fa-f]{6})"', m.group(1)) if m else []


def check_palette() -> list:
    """The colours of event dots stay distinct: measured pairwise, and
    against the dashboard's dark backgrounds."""
    bad = []
    colours = key_colours()
    if not colours:
        return ["diagram.js: KEY_COLOURS not found"]
    for i, a in enumerate(colours):
        for b in colours[i + 1:]:
            gap = _de2000(a, b)
            if gap < MIN_COLOUR_GAP:
                bad.append(f"KEY_COLOURS {a} and {b} are only {gap:.1f} apart (need {MIN_COLOUR_GAP:g})")
        for bg in DARK_BACKGROUNDS:
            gap = _de2000(a, bg, kL=1.0)
            if gap < MIN_BACKGROUND_GAP:
                bad.append(f"KEY_COLOURS {a} is only {gap:.1f} from background {bg} (need {MIN_BACKGROUND_GAP:g})")
    return [f"palette: {m}" for m in bad]


def check_in_flight(spec) -> list:
    """A diagram that colours events by order can tell at most as many
    orders apart as it has colours, so no setting may keep more in flight."""
    if not (spec.get("diagram") or {}).get("events"):
        return []
    limit, bad = len(key_colours()), []
    for name, st in (spec.get("settings") or {}).items():
        for c in st.get("choices", []) if isinstance(st, dict) else []:
            args = c.get("args", []) if isinstance(c, dict) else []
            for i, a in enumerate(args[:-1]):
                if a == "--in-flight" and str(args[i + 1]).isdigit() and int(args[i + 1]) > limit:
                    bad.append(f"setting {name!r} choice {c.get('value')!r} keeps {args[i + 1]} orders "
                               f"in flight, more than the {limit} colours that tell them apart")
    return bad


def _calls(text: str, name: str):
    """(offset, [arguments as source text]) for each call of name(...) in
    JavaScript source, splitting top-level commas outside strings."""
    for m in re.finditer(rf"(?<![\w.]){name}\(", text):
        i, depth, quote, args, cur = m.end(), 1, None, [], ""
        while i < len(text) and depth:
            c = text[i]
            if quote:
                cur += c
                if c == "\\":
                    cur += text[i + 1]
                    i += 1
                elif c == quote:
                    quote = None
            elif c in "\"'`":
                quote = c
                cur += c
            elif c in "([{":
                depth += 1
                cur += c
            elif c in ")]}":
                depth -= 1
                if depth:
                    cur += c
            elif c == "," and depth == 1:
                args.append(cur.strip())
                cur = ""
            else:
                cur += c
            i += 1
        args.append(cur.strip())
        yield m.start(), args


def _body(text: str, signature: str):
    """The character range of a function's body, found by its signature."""
    start = text.find(signature)
    if start < 0:
        return None
    # The body's brace follows the parameter list, which may hold "= {}".
    i = re.compile(r"\)\s*\{").search(text, start).end() - 1
    depth = 0
    for j in range(i, len(text)):
        depth += {"{": 1, "}": -1}.get(text[j], 0)
        if depth == 0:
            return start, j
    return None


def check_indicators() -> list:
    """Every indicator on the page explains itself.

    Hover text is set only through indicator(node, "<name>", text) in the
    dashboard's JavaScript (or the indicator: attribute of cockpit.js's h()),
    and each name needs a row in guides/000-reading-the-dashboard.md tagged
    <!-- indicator: <name> --> (one comment may list several names). Setting
    a title any other way, a name with no text anywhere, a name missing from
    the guide, or a guide tag for a name nothing uses, is a problem."""
    bad, names = [], {}
    for js in sorted(STATIC_JS.glob("*.js")):
        text = js.read_text()
        exempt = [r for r in (_body(text, "function indicator("), _body(text, "function tip("),
                              _body(text, "function h(")) if r]
        inside = lambda pos: any(a <= pos <= b for a, b in exempt)
        for pat, what in ((r"(?<![\w.])tip\(", "tip()"), (r"\.title\s*=(?!=)", ".title ="),
                          (r"[\"']title[\"']\s*\)", "a <title> element or title attribute"),
                          (r"\btitle:\s*[\"'`]", "a title: attribute")):
            for m in re.finditer(pat, text):
                if not inside(m.start()):
                    line = text.count("\n", 0, m.start()) + 1
                    bad.append(f"{js.name}:{line}: sets hover text with {what}; use indicator(node, name, text)")
        # The helpers themselves (h() passes its indicator: attribute on)
        # are not uses.
        calls = [(pos, args) for pos, args in _calls(text, "indicator")
                 if not inside(pos) and not text.startswith("function", max(0, pos - 9))]
        for m in re.finditer(r'indicator:\s*\[\s*"([a-z0-9-]+)"\s*,\s*([^\]]+)\]', text):
            calls.append((m.start(), ["node", f'"{m.group(1)}"', m.group(2).strip()]))
        for pos, args in calls:
            line = text.count("\n", 0, pos) + 1
            if len(args) != 3 or not re.fullmatch(r'"[a-z0-9-]+"', args[1]):
                bad.append(f"{js.name}:{line}: indicator() needs (node, \"literal-name\", text)")
                continue
            entry = names.setdefault(args[1].strip('"'), {"where": f"{js.name}:{line}", "text": False})
            if args[2] not in ("null", "undefined", '""', "''"):
                entry["text"] = True
    for name, entry in sorted(names.items()):
        if not entry["text"]:
            bad.append(f"indicator {name!r} ({entry['where']}) is never given hover text")
    guide = READING_GUIDE.read_text() if READING_GUIDE.exists() else ""
    tagged = set()
    for m in re.finditer(r"<!--\s*indicators?:\s*([a-z0-9,\s-]+?)\s*-->", guide):
        tagged.update(n.strip() for n in m.group(1).split(",") if n.strip())
    for name in sorted(set(names) - tagged):
        bad.append(f"indicator {name!r} ({names[name]['where']}) has no row in {READING_GUIDE.name} "
                   f"tagged <!-- indicator: {name} -->")
    for name in sorted(tagged - set(names)):
        bad.append(f"{READING_GUIDE.name} has a row for indicator {name!r}, which the page does not use")
    return [f"indicators: {m}" for m in bad]


def main() -> int:
    problems = []
    found = sorted(SCENARIOS.glob("*/scenario.yaml"))
    for path in found:
        problems.extend(check(path))
    problems.extend(check_indicators())
    problems.extend(check_palette())

    for p in problems:
        print(p)
    print(f"\nChecked {len(found)} scenarios, {len(problems)} problem(s).")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
