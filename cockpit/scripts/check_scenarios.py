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
STATIC_JS = COCKPIT / "static" / "js"
READING_GUIDE = COCKPIT.parent / "guides" / "000-reading-the-dashboard.md"

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

    bad.extend(check_partitions(spec, here, actions))

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
        if not isinstance(part, dict) or not (part.get("logs") or part.get("partitions")):
            bad.append(f"{where} {phase} needs a non-empty logs: list, partitions: checks, or both")
            continue
        if "partitions" in part:
            bad.extend(check_partition_expect(f"{where} {phase}", part["partitions"], spec))
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

    for p in problems:
        print(p)
    print(f"\nChecked {len(found)} scenarios, {len(problems)} problem(s).")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
