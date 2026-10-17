"""What a partitioned queue is doing, for the diagram's lanes and event dots.

Two sources, both real. The broker, over SEMP: each partition is an internal
queue (`#pq/<id>/0000N`) that reports the client that owns it
(`partitionClientName`), how many messages it holds now (the `msgs`
collection's count) and how many it has taken in (`spooledMsgCount`). And the
consumers' own output: each update a consumer handles is a log line naming
the order, its sequence number and the broker's number for the message.

The Go API does not tell a consumer which partition a message came from, so
the dashboard works it out from the broker. A sampler polls the partitions
several times a second and remembers which message ids it saw in which
partition (in its `msgs` list, or as its last spooled or acknowledged id).
A logged update whose message id was seen is placed exactly. A keyed update
whose message id was missed is placed by its key, once another update with
the same key has been placed: the broker sends one key to one partition.
Anything else is returned with no partition, and drawn as such. Nothing is
computed from the key itself: the broker's hash is not documented.
"""

import asyncio
import re
import time
from collections import OrderedDict, deque
from urllib.parse import quote

import httpx

from .config import MSG_VPN, SEMP_MONITOR_BASE_URL, SEMP_PASSWORD, SEMP_USER
from .processes import manager

# How often the sampler reads the partition list (owners, last ids) and each
# partition's message list. Fast enough that most messages are seen before
# they are acknowledged and gone.
LIST_EVERY = 0.25
MSGS_EVERY = 0.5
# The sampler stops when nobody has asked for a while.
IDLE_STOP = 20.0
# Log lines this recent are held back one more poll, so the sampler has had
# its chance to see their message ids.
HOLD = 0.3
# How far back a first request (since=0) and the move history reach.
WINDOW = 30.0
MSG_MEMORY = 20000

FLAGS = ("gap", "order", "picked", "redelivered")


class Tracker:
    """One partitioned queue, sampled while a page is looking at it."""

    def __init__(self, queue: str):
        self.queue = queue
        self.task: asyncio.Task | None = None
        self.last_asked = 0.0
        self.parent = None          # the "#pq/<id>" prefix: changes if the queue is recreated
        self.present = False
        self.status = None
        self.lanes: dict = {}       # partition -> {owner, held, spooled, unacked}
        self.moves: deque = deque(maxlen=200)
        self.msg_part: OrderedDict = OrderedDict()
        self.key_part: dict = {}

    def _forget(self) -> None:
        self.lanes, self.msg_part, self.key_part = {}, OrderedDict(), {}

    def _saw(self, msg_id, partition: int) -> None:
        if not msg_id:
            return
        self.msg_part[int(msg_id)] = partition
        self.msg_part.move_to_end(int(msg_id))
        while len(self.msg_part) > MSG_MEMORY:
            self.msg_part.popitem(last=False)

    def ensure_running(self) -> None:
        self.last_asked = time.monotonic()
        if self.task is None or self.task.done():
            self.task = asyncio.create_task(self._run())

    async def _get(self, client, path: str) -> dict:
        resp = await client.get(SEMP_MONITOR_BASE_URL + path, auth=(SEMP_USER, SEMP_PASSWORD))
        resp.raise_for_status()
        return resp.json()

    async def _run(self) -> None:
        vpn = quote(MSG_VPN, safe="")
        q = quote(self.queue, safe="")
        list_path = (f"/msgVpns/{vpn}/queues?count=100&where=partitionQueueName=={q}"
                     "&select=queueName,partitionNumber,partitionClientName,spooledMsgCount,"
                     "txUnackedMsgCount,lastSpooledMsgId,highestAckedMsgId,lowestAckedMsgId")
        parent_path = f"/msgVpns/{vpn}/queues/{q}?select=partitionCount,partitionRebalanceStatus"
        names: dict = {}
        next_msgs = 0.0
        async with httpx.AsyncClient(timeout=httpx.Timeout(3.0, connect=1.0)) as client:
            while time.monotonic() - self.last_asked < IDLE_STOP:
                try:
                    await self._sample_list(client, list_path, parent_path, names)
                    if time.monotonic() >= next_msgs and names:
                        next_msgs = time.monotonic() + MSGS_EVERY
                        await asyncio.gather(*(self._sample_msgs(client, vpn, n, name)
                                               for n, name in names.items()))
                except (httpx.HTTPError, ValueError):
                    # Broker restarting, or the queue going away mid-poll.
                    pass
                await asyncio.sleep(LIST_EVERY)

    async def _sample_list(self, client, list_path, parent_path, names) -> None:
        try:
            parent = (await self._get(client, parent_path)).get("data", {})
        except httpx.HTTPStatusError:
            parent = None
        if not parent:
            if self.present:
                self._forget()
            self.present, self.status = False, None
            names.clear()
            return
        self.present = True
        self.status = parent.get("partitionRebalanceStatus")

        rows = (await self._get(client, list_path)).get("data", [])
        prefix = rows[0]["queueName"].rsplit("/", 1)[0] if rows else None
        if prefix != self.parent or len(rows) != len(self.lanes):
            # A new queue (Cleanup then Play) or a new partition count: the
            # old message ids and key placements mean nothing any more.
            self._forget()
            self.parent = prefix
        now = time.time()
        names.clear()
        for r in rows:
            n = r["partitionNumber"]
            names[n] = r["queueName"]
            owner = r.get("partitionClientName") or None
            lane = self.lanes.setdefault(n, {"owner": owner, "held": 0})
            if lane["owner"] != owner:
                self.moves.append({"ts": now, "partition": n, "from": lane["owner"], "to": owner})
            lane.update(owner=owner, spooled=r.get("spooledMsgCount", 0),
                        unacked=r.get("txUnackedMsgCount", 0))
            for key in ("lastSpooledMsgId", "highestAckedMsgId", "lowestAckedMsgId"):
                self._saw(r.get(key), n)

    async def _sample_msgs(self, client, vpn, n: int, name: str) -> None:
        data = await self._get(
            client, f"/msgVpns/{vpn}/queues/{quote(name, safe='')}/msgs?count=100&select=msgId")
        for m in data.get("data", []):
            self._saw(m.get("msgId"), n)
        if n in self.lanes:
            self.lanes[n]["held"] = data.get("meta", {}).get("count", len(data.get("data", [])))


_trackers: dict = {}


def _box(scenario):
    """The diagram box drawn as partitions, and the events spec, or Nones."""
    diagram = scenario.diagram or {}
    for node in diagram.get("nodes", []):
        for child in node.get("contains", []) or []:
            if child.get("partitions"):
                return child, diagram.get("events")
    return None, None


def _events(scenario, spec, queue, tracker, since: float, until: float) -> list:
    pattern = re.compile(spec["pattern"])
    flags = {name: re.compile(p) for name, p in (spec.get("flags") or {}).items()}
    out = []
    for node in scenario.diagram.get("nodes", []):
        flow_on = node.get("flowOn") or {}
        if flow_on.get("queue") != queue or not node.get("action"):
            continue
        run = manager.get(f"{scenario.id}:{node['action']}")
        if run is None:
            continue
        for line in run.history():
            if line["stream"] != "stdout" or not (since < line["ts"] <= until):
                continue
            m = pattern.search(line["text"])
            if not m:
                continue
            g = m.groupdict()
            msg_id = int(g["msg"]) if g.get("msg") else None
            key = g.get("key")
            partition = tracker.msg_part.get(msg_id) if msg_id is not None else None
            if partition is not None and key:
                tracker.key_part[key] = partition
            elif partition is None and key:
                partition = tracker.key_part.get(key)
            out.append({
                "ts": line["ts"],
                "node": node["id"],
                "order": g.get("order"),
                "seq": int(g["seq"]) if g.get("seq") else None,
                "status": g.get("status"),
                "key": key,
                "msg": msg_id,
                "partition": partition,
                "flags": [f for f, p in flags.items() if p.search(line["text"])],
                "text": line["text"],
            })
    out.sort(key=lambda e: e["ts"])
    return out


def snapshot(scenario, since: float) -> dict | None:
    """Lanes, recent moves and the events logged after `since`, or None when
    the scenario draws no partitioned queue."""
    box, spec = _box(scenario)
    if box is None:
        return None
    queue = box["partitions"]["queue"]
    tracker = _trackers.setdefault((scenario.id, queue), Tracker(queue))
    tracker.ensure_running()

    nodes = scenario.diagram.get("nodes", [])
    node_of = {n["flowOn"]["client"]: n["id"] for n in nodes
               if (n.get("flowOn") or {}).get("queue") == queue}
    now = time.time()
    until = now - HOLD
    since = max(since, now - WINDOW)
    lanes = [{"partition": n, "owner": l["owner"], "node": node_of.get(l["owner"]),
              "held": l.get("held", 0), "spooled": l.get("spooled", 0),
              "unacked": l.get("unacked", 0)}
             for n, l in sorted(tracker.lanes.items())]
    return {
        "box": box["id"],
        "queue": queue,
        "present": tracker.present,
        "status": tracker.status,
        "lanes": lanes,
        "moves": [dict(m, fromNode=node_of.get(m["from"]), toNode=node_of.get(m["to"]))
                  for m in tracker.moves if m["ts"] > now - WINDOW],
        "events": _events(scenario, spec, queue, tracker, since, until) if spec else [],
        "until": until,
    }
