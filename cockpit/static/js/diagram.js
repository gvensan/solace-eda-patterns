/* Scenario diagram renderer.

   Turns the `diagram:` block in a scenario.yaml into an animated SVG that shows
   what the scenario does. Usually it is representative, not live: packets loop
   on a timer while the apps that send them run, rather than tracking real
   broker traffic. A diagram with `events:` is the exception: there every dot
   is one real update, drawn after a consumer reports handling it, through the
   partition the broker held it in (see "Partitioned queues" below).

   Scenarios declare nodes and flows; layout is computed here so every scenario
   looks like it belongs to the same workshop.
*/

const Diagram = (() => {
  const SVG_NS = "http://www.w3.org/2000/svg";

  /* Geometry. Columns are laid out left to right, nodes stacked within a
     column, which suits the publisher -> broker -> subscribers shape that most
     messaging scenarios take. */
  const NODE_W = 150;
  const NODE_H = 76;
  const COL_GAP = 136;
  const ROW_GAP = 26;
  const PAD_X = 18;
  const PAD_Y = 22;

  /* A node may contain configuration objects rather than point at them.
     Drawing a queue as a box inside the broker says "this lives on the
     broker" structurally, where an arrow to a box outside it says "messages
     travel here" -- which is wrong for something that was merely created. */
  const CHILD_H = 26;
  const CHILD_GAP = 6;
  const CHILD_INSET = 12;
  // Space above the first child: status dot, label and sublabel.
  const CHILD_TOP = 60;
  /* Bottom strip left clear below the last child, for the node's own count
     and its logs button. Without it the last box sits underneath both. */
  const CHILD_BOTTOM = 28;
  /* A node holding configuration needs more width than a plain one: each
     child carries a label and a count on the same line. */
  const NODE_W_WIDE = 210;

  function el(name, attrs = {}, ...children) {
    const node = document.createElementNS(SVG_NS, name);
    for (const [k, v] of Object.entries(attrs)) {
      if (v === null || v === undefined || v === false) continue;
      node.setAttribute(k, String(v));
    }
    for (const c of children.flat()) {
      if (c === null || c === undefined || c === false) continue;
      node.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return node;
  }

  /* Hover text for anything drawn on the diagram. Every badge, dot and label
     says what it means when pointed at, so nothing on the picture has to be
     explained by someone standing next to the attendee. An SVG <title> is the
     browser's own tooltip and is read out by screen readers too. Called again
     on each poll for live indicators, so the text tracks what is shown. */
  function tip(node, text) {
    if (!node) return;
    let t = node.querySelector(":scope > title");
    if (!text) { t?.remove(); return; }
    if (!t) { t = el("title"); node.prepend(t); }
    if (t.textContent !== text) t.textContent = text;
  }

  /* Every indicator goes through here, named. check_scenarios.py finds each
     name and fails unless guides/000-reading-the-dashboard.md has a row
     tagged with it, so nothing new appears on the page unexplained. */
  function indicator(node, name, text) {
    if (!node) return;
    node.setAttribute("data-indicator", name);
    tip(node, text);
  }

  /* Place every node on a grid: x from its column, y from its position within
     that column, with each column centred vertically against the tallest one so
     a single broker sits level with three subscribers rather than at the top. */
  // How tall one node needs to be, given whatever it contains.
  /* A processor consumes and publishes, and is drawn as a hexagon standing on
     a point: events arrive at the top point and the new events it publishes
     leave from the bottom one, so a chain of processors reads as work falling
     through the pipeline even though every hop goes back through the broker.
     A hexagon rather than a cylinder, which would read as a database. */
  const isProcessor = (n) => n.shape === "processor";
  // Height of the hexagon's top and bottom points above and below its sides.
  const HEX_POINT = 18;
  /* Space between two nodes in a column when either is a processor: room for
     one flow leaving a bottom and the next arriving at a top. */
  const PROC_GAP = 64;

  function rowGap(a, b) {
    return isProcessor(a) || isProcessor(b) ? PROC_GAP : ROW_GAP;
  }

  /* A partitioned queue is drawn as a box of lanes, one per partition, with
     a header line for the queue's name and its rebalance state. */
  const LANE_H = 19;
  const LANE_GAP = 3;
  const PART_HEAD = 24;
  const PART_PAD = 7;
  const NODE_W_LANES = 300;
  // A consumer of a partitioned queue has a row for the updates it handled.
  const SEQ_ROW = 22;

  function childHeight(c) {
    if (!c.partitions) return CHILD_H;
    const k = Number(c.partitions.count) || 1;
    return PART_HEAD + k * LANE_H + (k - 1) * LANE_GAP + PART_PAD;
  }

  /* A hop inside the broker (a flow with into:) runs between two of its
     boxes, so those two sit further apart, with room for the line and its
     label. hopPairs is set on a copy of the broker node by render. */
  const HOP_GAP = 30;
  const hopKey = (a, b) => [a, b].sort().join("|");
  function childGap(n, k) {
    const kids = n.contains || [];
    return kids[k + 1] && n.hopPairs?.has(hopKey(kids[k].id, kids[k + 1].id)) ? HOP_GAP : CHILD_GAP;
  }
  const isHop = (f) => f.into !== undefined;

  function nodeHeight(n) {
    // A point above and below the usual box, so the text keeps its room.
    if (isProcessor(n)) return NODE_H + HEX_POINT * 2;
    const kids = n.contains || [];
    if (!kids.length) return NODE_H + (n.seqRow ? SEQ_ROW : 0);
    return CHILD_TOP + kids.reduce((t, c, k) => t + childHeight(c) + (k ? childGap(n, k - 1) : 0), 0)
      + CHILD_BOTTOM;
  }

  /* A node holding configuration grows to fit its longest child label, so a
     queue name like q.shock.scans.partitioned never runs into its count.
     Child labels are 10px monospace, about 6.2px a character; the constant
     covers the insets, the label's padding and the count beside it. */
  const CHILD_CHAR_W = 6.2;
  const DEPTH_W = 64;
  const CHILD_CHROME_W = 76;

  function nodeWidth(n) {
    if (!n.contains?.length) return NODE_W;
    const longest = Math.max(...n.contains.map((c) => (c.label || "").length));
    const lanes = n.contains.some((c) => c.partitions) ? NODE_W_LANES : 0;
    // Room for a live depth ("12 waiting") beside the label and its count.
    const depth = n.contains.some((c) => c.depth) ? DEPTH_W : 0;
    return Math.max(NODE_W_WIDE, lanes, Math.ceil(CHILD_CHROME_W + depth + longest * CHILD_CHAR_W));
  }

  /* A flow label sits between two columns, so a long one (a full topic
     subscription) widens the gap after its column rather than running into
     the nodes either side. Edge labels are 9.5px monospace, a little under
     6px a character; the margin keeps clear of the arrowheads. */
  const LABEL_CHAR_W = 5.9;
  const LABEL_MARGIN = 16;

  function layout(nodes, flows = []) {
    const columns = new Map();
    for (const n of nodes) {
      const col = Number(n.col ?? 0);
      if (!columns.has(col)) columns.set(col, []);
      columns.get(col).push(n);
    }

    const colIndexes = [...columns.keys()].sort((a, b) => a - b);

    /* Column height is the sum of its nodes' own heights, not a count times a
       constant, because a node holding configuration is taller than one that
       does not. */
    const colHeight = (c) => {
      const m = columns.get(c);
      return m.reduce((t, n, i) => t + nodeHeight(n) + (i ? rowGap(m[i - 1], n) : 0), 0);
    };
    /* Flows enter a processor from above and leave below, so a diagram with
       one needs room over its top node and under its bottom one. */
    const hasProcessor = nodes.some(isProcessor);
    /* A queue box drawn level with the lane into the top processor has to
       clear the broker's own label, so that lane moves down to make room. */
    const viaTop = hasProcessor && flows.some((f) => f.via) ? QUEUE_HEADROOM : 0;
    const padY = PAD_Y + (hasProcessor ? PROC_HEADROOM : 0);
    const canvasH = padY * 2 + viaTop + Math.max(...colIndexes.map(colHeight));

    /* Column width is set by its widest member, so a column containing the
       broker and its configuration is wider than one holding subscribers,
       and every node in a column still lines up on the left. */
    const colWidth = (c) => Math.max(...columns.get(c).map(nodeWidth));

    const colOf = new Map(nodes.map((n) => [n.id, Number(n.col ?? 0)]));
    const gapAfter = (col) => Math.max(COL_GAP, ...flows
      .filter((f) => f.label && Math.min(colOf.get(f.from), colOf.get(f.to)) === col)
      .map((f) => Math.ceil(f.label.length * LABEL_CHAR_W + LABEL_MARGIN * 2)));

    // Left edge of each column, accumulated so widths can differ.
    const colX = new Map();
    let x = PAD_X;
    let lastGap = COL_GAP;
    colIndexes.forEach((col) => {
      colX.set(col, x);
      lastGap = gapAfter(col);
      x += colWidth(col) + lastGap;
    });

    const placed = new Map();
    colIndexes.forEach((col) => {
      const members = columns.get(col);
      const top = viaTop + (canvasH - viaTop - colHeight(col)) / 2;
      let y = top;
      members.forEach((n, i) => {
        if (i) y += rowGap(members[i - 1], n);
        const h = nodeHeight(n);
        const w = nodeWidth(n);
        placed.set(n.id, {
          ...n,
          // Centre a narrow node within a wide column so a column of mixed
          // widths reads as a column rather than a ragged edge.
          x: colX.get(col) + (colWidth(col) - w) / 2,
          y,
          w,
          h,
        });
        y += h;
      });
    });

    /* With processors, the broker runs the full height of the diagram. Each
       flow to or from a processor travels along the gap above or below it,
       and a broker that spans every gap can take each one head on. */
    if (hasProcessor) {
      for (const n of placed.values()) {
        if (n.kind !== "broker") continue;
        n.y = PAD_Y;
        n.h = Math.max(n.h, canvasH - PAD_Y * 2);
      }
    }

    const canvasW = x - lastGap + PAD_X;
    return { placed, canvasW, canvasH };
  }

  function drawNode(n, handlers) {
    /* The broker is the one element drawn in Classic Green: it is the subject of
       the workshop, and colouring everything would flatten that. */
    const isBroker = n.kind === "broker";
    /* Anything with something to say is clickable: an action to toggle, or a
       checklist worth reading. A config node has no process but its list of
       what does and does not exist is the most useful thing on the page. */
    const hasChecklist = Boolean(
      (n.requires && n.requires.length) || (n.provides && n.provides.length)
    );
    const clickable = Boolean(n.action) || isBroker || hasChecklist;

    const g = el("g", {
      class: `dg-node dg-node--${n.kind || "app"}${clickable ? " is-clickable" : ""}`,
      "data-node": n.id,
    });
    if (isBroker) g.classList.add("is-broker");

    // Content sits below a hexagon's top point, so everything shifts down by it.
    const shift = isProcessor(n) ? HEX_POINT : 0;

    if (isProcessor(n)) {
      const { x, y, w, h } = n;
      const cx = x + w / 2;
      g.append(el("path", {
        class: "dg-node__box",
        d: `M ${cx} ${y} L ${x + w} ${y + HEX_POINT} L ${x + w} ${y + h - HEX_POINT} ` +
           `L ${cx} ${y + h} L ${x} ${y + h - HEX_POINT} L ${x} ${y + HEX_POINT} Z`,
        "stroke-linejoin": "round",
      }));
    } else {
      g.append(el("rect", {
        x: n.x, y: n.y, width: n.w, height: n.h,
        rx: 10, class: "dg-node__box",
      }));
    }

    /* Status ring: a small dot rather than a border change, so a node's state
       is readable without competing with the broker's green fill. */
    g.append(el("circle", {
      cx: n.x + 14, cy: n.y + 15 + shift, r: 4.5, class: "dg-node__dot",
    }));

    const cx = n.x + n.w / 2;
    g.append(el("text", {
      x: cx, y: n.y + 32 + shift, class: "dg-node__label", "text-anchor": "middle",
    }, n.label));

    if (n.sublabel) {
      g.append(el("text", {
        x: cx, y: n.y + 48 + shift, class: "dg-node__sub", "text-anchor": "middle",
      }, n.sublabel));
    }

    /* Configuration this node holds, drawn as dotted boxes inside it. Nesting
       is the whole point: an object on the broker is part of the broker, not
       a separate destination something is sent to. */
    let slot = n.y + CHILD_TOP;
    (n.contains || []).forEach((child, k) => {
      const box = n.childBoxes?.get(child.id);
      const cyTop = box ? box.y : slot;
      slot += childHeight(child) + childGap(n, k);
      const kid = el("g", { class: "dg-node__child", "data-child": child.id || "" });
      const textY = box?.grown ? cyTop + box.h / 2 + 4 : cyTop + 17;

      kid.append(el("rect", {
        x: n.x + CHILD_INSET, y: cyTop,
        width: n.w - CHILD_INSET * 2, height: box ? box.h : childHeight(child),
        rx: 5, class: "dg-node__child-box",
      }));
      if (child.partitions) drawLanes(kid, n, child, cyTop);
      kid.append(el("text", {
        x: n.x + CHILD_INSET + 8, y: textY,
        class: "dg-node__child-label", "text-anchor": "start",
      }, child.label));

      // How many messages the queue holds now, read from the broker.
      if (child.depth) {
        kid.append(el("text", {
          x: n.x + n.w - CHILD_INSET - 34, y: textY,
          class: "dg-node__child-depth", "text-anchor": "end",
          "data-child-depth": child.id || "",
        }, ""));
      }

      /* The count is filled in from SEMP on each poll, like the node-level
         check, so a box reads as present or missing rather than assumed. */
      kid.append(el("text", {
        x: n.x + n.w - CHILD_INSET - 8, y: textY,
        class: "dg-node__child-count", "text-anchor": "end",
        "data-child-count": child.id || "",
      }, ""));

      g.append(kid);
    });

    /* A consumer of a partitioned queue keeps a row of the last updates it
       handled, filled from its log (see paintPartitions). */
    if (n.seqRow) {
      g.setAttribute("data-partitioned", "true");
      g.append(el("g", { class: "dg-seq", "data-seq": n.id }));
    }

    /* Bottom row: how much of this node's config exists, and a way into its
       logs. Both are per-node so nothing has to live further down the page. */
    // A hexagon narrows to its bottom point, so its bottom row sits higher.
    const base = isProcessor(n) ? HEX_POINT : 0;
    g.append(el("text", {
      x: n.x + 12, y: n.y + n.h - 10 - base,
      class: "dg-node__check", "text-anchor": "start",
    }, ""));

    // How many rows of what it holds (holds: count:), between the check and
    // the logs button. The table under the diagram has the rows themselves.
    if (n.holds?.count) {
      g.append(el("text", {
        x: n.x + (n.w - 8) / 2, y: n.y + n.h - 10 - base,
        class: "dg-node__holds", "text-anchor": "middle",
      }, ""));
    }

    if (n.action) {
      const btn = el("g", { class: "dg-node__logbtn", "data-log": n.id,
        role: "button", tabindex: "0" });
      btn.append(el("rect", {
        x: n.x + n.w - 42, y: n.y + n.h - 22 - base, width: 32, height: 16,
        rx: 8, class: "dg-node__logbtn-box",
      }));
      btn.append(el("text", {
        x: n.x + n.w - 26, y: n.y + n.h - 10 - base,
        class: "dg-node__logbtn-text", "text-anchor": "middle",
      }, "logs"));
      indicator(btn, "logs-button", "Show what this step printed, in the side panel");
      btn.addEventListener("click", (e) => {
        // Never let the log button also trigger the node's start/stop.
        e.stopPropagation();
        handlers.onLogs?.(n.id);
      });
      g.append(btn);
    }

    if (clickable) {
      g.addEventListener("click", () => handlers.onSelect?.(n.id));
      g.setAttribute("role", "button");
      g.setAttribute("tabindex", "0");
      g.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handlers.onSelect?.(n.id);
        }
      });
    }

    return g;
  }

  /* A flow is drawn as a path between two nodes, with a packet animated along
     it. Curved rather than straight so fan-out from one broker to three
     subscribers stays readable.

     Most flows run left to right, from one node's right edge to the next
     one's left. A flow back to a node on the left is an app publishing to the
     broker it also consumes from, so it leaves the app's left edge and lands
     on the broker's right edge. A processor is the exception: flows arrive at
     its top and leave from its bottom. */
  const isBack = (from, to) => to.x + to.w <= from.x;

  /* Which edge of `node` a flow uses, and its outward direction, which the
     curve follows so a line always leaves and enters square to its edge. */
  function sideOf(node, other) {
    const otherIsLeft = other.x + other.w <= node.x;
    return otherIsLeft ? { side: "L", nx: -1, ny: 0 } : { side: "R", nx: 1, ny: 0 };
  }

  function flowPath(p1, p2) {
    const k = Math.max(28, Math.abs(p2.x - p1.x) / 2);
    return `M ${p1.x} ${p1.y} C ${p1.x + p1.nx * k} ${p1.y}, ` +
      `${p2.x + p2.nx * k} ${p2.y}, ${p2.x} ${p2.y}`;
  }

  /* Flows to and from a processor are routed square rather than curved: along
     the gap above it and down onto its top point, or out of its bottom point
     and along the gap below it. Stacked processors then read as one stream falling from
     each into the next, with the broker between every hop. */
  // Distance of the lane above or below a processor from its edge.
  const PROC_LANE = 22;
  // Radius of the rounded corner where a lane turns into a processor.
  const PROC_CORNER = 10;
  // Room above the top processor and below the bottom one for their lanes.
  const PROC_HEADROOM = 36;
  /* Extra room above the top processor when a queue box sits on its lane:
     enough to put that lane below the broker's label. */
  const QUEUE_HEADROOM = CHILD_TOP + CHILD_H / 2 + PROC_LANE - PROC_HEADROOM;

  function processorPath(from, to) {
    const r = PROC_CORNER;
    /* Lines leave and arrive on the side facing the other end, so a
       processor left of the broker is not reached by a line drawn across
       the whole broker (and through the queues in it). */
    if (isProcessor(to)) {
      const cx = to.x + to.w / 2;
      const right = cx > from.x + from.w / 2;
      const x1 = right ? from.x + from.w : from.x;
      const s = right ? 1 : -1;
      const lane = to.y - PROC_LANE;
      return {
        d: `M ${x1} ${lane} H ${cx - s * r} Q ${cx} ${lane} ${cx} ${lane + r} V ${to.y}`,
        label: { x: (x1 + cx) / 2, y: lane - 6 },
      };
    }
    const cx = from.x + from.w / 2;
    const base = from.y + from.h;
    const lane = base + PROC_LANE;
    const right = to.x + to.w / 2 > cx;
    const x2 = right ? to.x : to.x + to.w;
    const s = right ? 1 : -1;
    return {
      d: `M ${cx} ${base} V ${lane - r} Q ${cx} ${lane} ${cx + s * r} ${lane} H ${x2}`,
      label: { x: (cx + x2) / 2, y: lane - 6 },
    };
  }

  /* Where a queue box sits inside the broker: its placed position when the
     broker moved its boxes (placeQueueBoxes), otherwise its stacked slot. */
  function boxRectIn(broker, childId) {
    const kids = broker.contains || [];
    if (!kids.some((c) => c.id === childId)) return null;
    const x = broker.x + CHILD_INSET;
    const w = broker.w - CHILD_INSET * 2;
    const placedBox = broker.childBoxes?.get(childId);
    if (placedBox) return { x, w, y: placedBox.y, h: placedBox.h };
    let top = broker.y + CHILD_TOP;
    for (const [k, c] of kids.entries()) {
      if (c.id === childId) return { x, w, y: top, h: childHeight(c) };
      top += childHeight(c) + childGap(broker, k);
    }
    return null;
  }

  /* A processor publishing into a queue box: out of its bottom point and
     along the lane below it, as processorPath, then up outside the broker's
     edge, and level into the side of the box. Outside rather than up the
     gap inside the broker, so the line reads as arriving from outside and
     never runs past another queue on the way: that would read as going
     into that queue too. It lands a little off the box's middle, on the
     side it comes from (below the middle coming up, above it coming down),
     so it stops short of a line leaving the box level and never crosses
     it. */
  /* One drawn size per layout unit for every diagram, so text and boxes
     look the same on every page. Event Sourcing's diagram set the look: at
     1.3 its text reads comfortably. Where the panel is too narrow for that,
     every diagram shrinks by the same factor, as if it were as wide as
     DIAGRAM_REF_W (about the widest diagram, Hierarchical Topics, 1006). */
  const DIAGRAM_SCALE = 1.3;
  const DIAGRAM_REF_W = 1010;
  const OUTSIDE_RUN = 16;
  const FORK_GAP = 14;
  function processorIntoBox(from, broker, box, landing, run = OUTSIDE_RUN) {
    const r = PROC_CORNER;
    const cx = from.x + from.w / 2;
    const base = from.y + from.h;
    const lane = base + PROC_LANE;
    const right = broker.x > cx;
    const xv = right ? broker.x - run : broker.x + broker.w + run;
    const xb = right ? box.x : box.x + box.w;
    const mid = landing ?? box.y + box.h / 2;
    const up = mid < lane;
    const ye = landing ?? mid + (up ? 5 : -5);
    const s = right ? 1 : -1;
    // A landing within a few pixels of the lane would leave a small hook
    // beside the box; the line runs straight in along the lane instead.
    if (Math.abs(ye - lane) < 8) {
      return {
        d: `M ${cx} ${base} V ${lane - r} Q ${cx} ${lane} ${cx + s * r} ${lane} H ${xb}`,
        label: { x: (cx + xv) / 2, y: lane - 6 },
        badge: { x: (xv + xb) / 2, y: lane },
      };
    }
    return {
      d: `M ${cx} ${base} V ${lane - r} Q ${cx} ${lane} ${cx + s * r} ${lane} H ${xv - s * r} `
        + `Q ${xv} ${lane} ${xv} ${up ? lane - r : lane + r} V ${up ? ye + 3 : ye - 3} `
        + `Q ${xv} ${ye} ${xv + s * 3} ${ye} H ${xb}`,
      label: { x: (cx + xv) / 2, y: lane - 6 },
      // On the line's own upright, where a fork has split from its twin.
      badge: { x: xv, y: (lane + ye) / 2 },
    };
  }

  /* A direct line (no via:) meets the broker level with the pin at its
     other end where the wall allows, so it runs straight across, and never
     level with a queue box: there it reads as coming out of that queue.
     The lines on one side keep their order and spacing. Repeats of one
     line (a burst) stay one. */
  function settleBrokerEnds(broker, flows, placed, ports) {
    const boxes = (broker.contains || []).map((c) => boxRectIn(broker, c.id)).filter(Boolean);
    const lo = broker.y + 12;
    const hi = broker.y + broker.h - 12;
    const clear = (y) => !boxes.some((b) => y > b.y - BOX_CLEAR && y < b.y + b.h + BOX_CLEAR);
    const sides = new Map();
    flows.forEach((f, i) => {
      if (f.via || !ports[i]) return;
      const k = f.from === broker.id ? 0 : f.to === broker.id ? 1 : -1;
      if (k < 0 || !ports[i][k] || !ports[i][1 - k]) return;
      const p = ports[i][k];
      const key = `${p.x}`;
      if (!sides.has(key)) sides.set(key, new Map());
      const lines = sides.get(key);
      const id = `${p.y}|${ports[i][1 - k].y}`;
      if (!lines.has(id)) lines.set(id, { want: ports[i][1 - k].y, was: p.y, points: [] });
      lines.get(id).points.push(p);
    });
    for (const lines of sides.values()) {
      const ordered = [...lines.values()].sort((a, b) => a.want - b.want || a.was - b.was);
      const spots = [];
      for (let y = lo; y <= hi; y += 2) if (clear(y)) spots.push(y);
      /* The closest fit that keeps every line in order and apart: a small
         search over the clear heights. Tighter spacing if the wall is
         short; if nothing fits, the lines stay where they were. */
      for (const gap of [PORT_GAP, 10, 8]) {
        const fit = fitInOrder(ordered.map((l) => Math.min(Math.max(l.want, lo), hi)), spots, gap);
        if (!fit) continue;
        ordered.forEach((l, k) => { for (const p of l.points) p.y = fit[k]; });
        break;
      }
    }
  }

  /* Heights for wants (ascending) chosen from spots (ascending), in order,
     at least gap apart, with the least total movement. Null if they do not
     fit. Sizes here are a handful of lines and a few dozen spots. */
  function fitInOrder(wants, spots, gap) {
    const n = wants.length;
    const m = spots.length;
    let prev = spots.map((y) => Math.abs(y - wants[0]));
    const from = [spots.map(() => -1)];
    for (let j = 1; j < n; j++) {
      const cur = new Array(m).fill(Infinity);
      const back = new Array(m).fill(-1);
      let best = Infinity;
      let bestAt = -1;
      let q = 0;
      for (let c = 0; c < m; c++) {
        // Best earlier spot at least gap above this one.
        while (q < m && spots[q] <= spots[c] - gap) {
          if (prev[q] < best) { best = prev[q]; bestAt = q; }
          q += 1;
        }
        if (bestAt >= 0) {
          cur[c] = best + Math.abs(spots[c] - wants[j]);
          back[c] = bestAt;
        }
      }
      prev = cur;
      from.push(back);
    }
    let end = -1;
    for (let c = 0; c < m; c++) if (prev[c] < Infinity && (end < 0 || prev[c] < prev[end])) end = c;
    if (end < 0) return null;
    const out = new Array(n);
    for (let j = n - 1, c = end; j >= 0; j--) { out[j] = spots[c]; c = from[j][c]; }
    return out;
  }

  // Spacing between connection points on one edge of a node.
  const PORT_GAP = 14;
  // How much of an app's side its lines spread over, when it has several.
  const PIN_SPREAD = 0.4;
  // Spacing between publishes that end in the same queue box.
  const LANDING_GAP = 12;
  // How far a direct line onto the broker stays from a queue box's edge.
  const BOX_CLEAR = 8;
  // Most space left between boxes spread over a tall broker (placeQueueBoxes).
  const MAX_SPREAD = 60;
  // How far a publish lands from a delivery leaving the same side of a box,
  // and how far a grown box reaches past its outermost line.
  const OUT_GAP = 30;
  const LANDING_PAD = 9;

  /* Where each flow meets each node. Every edge of a node spreads its flows
     over separate points, ordered by where the other end sits, so a broker
     consuming to four apps and hearing back from two of them shows six
     distinct lines rather than one knot. Repeated flows between the same two
     nodes in the same direction (a burst drawn as three packets) share a
     point, because they are the same line. Returns [start, end] per flow,
     each a point with its outward direction. */
  function assignPorts(flows, placed) {
    const sides = new Map();
    const attach = (node, other, leaving, i) => {
      const { side, nx, ny } = sideOf(node, other);
      const key = `${node.id}:${side}`;
      if (!sides.has(key)) sides.set(key, { node, side, nx, ny, lanes: new Map() });
      const lanes = sides.get(key).lanes;
      /* Consumers of one queue leave the broker from one point, the queue's
         box, so competing consumers read as a fan out of a single queue. A
         publish with via: lands on the box it goes into, the same way, so a
         topic two queues subscribe to forks into both. Only the broker's end
         is split by box: the publisher's end stays one point. */
      const via = node.kind === "broker" && flows[i].via;
      const lane = via ? `via:${via}${leaving ? ">" : "<"}` : `${other.id}${leaving ? ">" : "<"}`;
      if (!lanes.has(lane)) {
        lanes.set(lane, {
          otherY: 0,
          n: 0,
          back: leaving ? isBack(node, other) : isBack(other, node),
          uses: [],
        });
      }
      const l = lanes.get(lane);
      // Aim a shared point at the middle of everything it feeds.
      l.otherY = (l.otherY * l.n + other.y + other.h / 2) / (l.n + 1);
      l.n += 1;
      l.uses.push([i, leaving ? 0 : 1]);
    };

    flows.forEach((f, i) => {
      const from = placed.get(f.from);
      const to = placed.get(f.to);
      // Processor flows run along their own lanes; see processorPath.
      if (!from || !to || isProcessor(from) || isProcessor(to)) return;
      attach(from, to, true, i);
      attach(to, from, false, i);
    });

    const ends = flows.map(() => [null, null]);
    for (const { node, side, nx, ny, lanes } of sides.values()) {
      // Down the edge by the far end, and on a tie the outbound lane above its
      // return so a consume and the publish back never cross each other.
      const ordered = [...lanes.values()].sort(
        (a, b) => a.otherY - b.otherY || Number(a.back) - Number(b.back)
      );
      /* An app's lines are pinned across its whole side, not bunched in the
         middle: one in the middle, two at either end of the side (30% and
         70% down), more spread evenly between those, so each line is easy
         to tell apart and to click. The broker's side is tall and meets
         many apps, so there the lines stay close, aimed at the far ends. */
      const pinned = node.kind !== "broker";
      const span = pinned
        ? (ordered.length > 1 ? Math.min(node.h - 24, Math.max(node.h * PIN_SPREAD, (ordered.length - 1) * PORT_GAP)) : 0)
        : Math.min(node.h - 24, (ordered.length - 1) * PORT_GAP);
      const step = ordered.length > 1 ? span / (ordered.length - 1) : 0;
      // Centred on the far ends rather than on this node, so a tall broker
      // meets a short app level with it instead of from far above or below.
      const aim = pinned ? node.y + node.h / 2 : ordered.reduce((t, l) => t + l.otherY, 0) / ordered.length;
      const centre = Math.min(Math.max(aim, node.y + 12 + span / 2), node.y + node.h - 12 - span / 2);
      ordered.forEach((lane, k) => {
        const point = { x: side === "L" ? node.x : node.x + node.w, y: centre - span / 2 + k * step };
        for (const [i, end] of lane.uses) ends[i][end] = { ...point, nx, ny };
      });
    }
    return ends;
  }

  /* Where each queue box sits inside the broker, so a line to a consumer
     visibly starts at the queue it reads (`via:` on the flow).

     A broker sized to its boxes has no spare room, so there the boxes keep
     their stacked slots and the lines leave from them. A broker running the
     height of a processor diagram has room, and its lanes into processors
     sit at fixed heights, so there each box moves level with its lines,
     spanning them if they leave at different heights, and the boxes nothing
     leaves through fill the space left over. */
  function placeQueueBoxes(broker, flows, placed, ports) {
    const kids = broker.contains || [];
    if (!kids.some((c) => flows.some((f) => f.via === c.id))) return null;

    if (![...placed.values()].some(isProcessor)) {
      let top = broker.y + CHILD_TOP;
      kids.forEach((c, k) => {
        const mid = top + childHeight(c) / 2;
        top += childHeight(c) + childGap(broker, k);
        flows.forEach((f, i) => {
          if (f.via === c.id && f.from === broker.id && ports[i][0]) ports[i][0].y = mid;
          if (f.via === c.id && f.to === broker.id && ports[i][1]) ports[i][1].y = mid;
        });
      });
      return null;
    }

    const top = broker.y + CHILD_TOP;
    const bottom = broker.y + broker.h - CHILD_BOTTOM;
    const half = CHILD_H / 2;
    const clamp = (y) => Math.min(Math.max(y, top + half), bottom - half);

    const boxes = new Map();
    const displaced = new Set();
    for (const c of kids) {
      const ys = [];
      flows.forEach((f, i) => {
        if (f.via !== c.id || f.from !== broker.id) return;
        const to = placed.get(f.to);
        if (!to) return;
        if (isProcessor(to)) {
          ys.push(to.y - PROC_LANE);
        } else if (ports[i][0]) {
          ports[i][0].y = clamp(ports[i][0].y);
          ys.push(ports[i][0].y);
        }
      });
      if (!ys.length) continue;
      const y1 = clamp(Math.min(...ys)) - half;
      const y2 = clamp(Math.max(...ys)) + half;
      /* Two processors level with each other (one each side of the broker)
         put their lanes at the same height. The second box would sit on the
         first, so it takes a free slot instead, and its lines are drawn
         from there as ordinary curves (see render). */
      const overlaps = [...boxes.values()].some((b) => y1 < b.y + b.h + CHILD_GAP && y2 + CHILD_GAP > b.y);
      if (overlaps) {
        displaced.add(c.id);
        continue;
      }
      boxes.set(c.id, { y: y1, h: y2 - y1 });
    }

    const gapTo = (a, b) => (broker.hopPairs?.has(hopKey(a, b)) ? HOP_GAP : CHILD_GAP);
    const brokerMid = broker.x + broker.w / 2;
    const sideOf = (n) => (n.x + n.w / 2 < brokerMid ? "left" : "right");
    const intoOf = (id) => {
      const into = [];
      flows.forEach((f, i) => {
        if (f.to !== broker.id || f.via !== id) return;
        const from = placed.get(f.from);
        if (!from) return;
        // A processor's publish comes along the lane below it (processorIntoBox).
        const fromY = isProcessor(from) ? from.y + from.h + PROC_LANE : ports[i][0]?.y ?? from.y;
        into.push({ i, fromY, side: sideOf(from) });
      });
      return into;
    };
    /* A publish landing on the side of a box that a delivery leaves from
       would arrive level with it, and the two read as one line. It lands
       clear of the delivery instead, above it if it comes from above and
       below if from below, and the box grows to hold both. Publishes on a
       side with no delivery spread about the box's middle as before.
       Returns false, changing nothing, when no publish shares a side with a
       delivery or the taller box would not fit. */
    const sideClearOfOuts = (id, box, into) => {
      const outs = [];
      flows.forEach((f, i) => {
        if (f.from !== broker.id || f.via !== id) return;
        const to = placed.get(f.to);
        if (!to) return;
        const y = isProcessor(to) ? clamp(to.y - PROC_LANE) : ports[i][0]?.y;
        if (y !== undefined) outs.push({ side: sideOf(to), y });
      });
      if (!into.some((n) => outs.some((o) => o.side === n.side))) return false;
      const landings = new Map();
      const loose = [];
      for (const side of ["left", "right"]) {
        const ys = outs.filter((o) => o.side === side).map((o) => o.y);
        const mine = into.filter((n) => n.side === side).sort((a, b) => a.fromY - b.fromY);
        if (!ys.length) { loose.push(...mine); continue; }
        const lo = Math.min(...ys);
        const hi = Math.max(...ys);
        const above = mine.filter((n) => n.fromY < (lo + hi) / 2);
        const below = mine.filter((n) => n.fromY >= (lo + hi) / 2);
        above.reverse().forEach((n, k) => landings.set(n.i, lo - OUT_GAP - k * LANDING_GAP));
        below.forEach((n, k) => landings.set(n.i, hi + OUT_GAP + k * LANDING_GAP));
      }
      const ys = [...landings.values(), ...outs.map((o) => o.y)];
      const y1 = Math.min(box.y, Math.min(...ys) - LANDING_PAD);
      const y2 = Math.max(box.y + box.h, Math.max(...ys) + LANDING_PAD);
      const fits = y1 >= top && y2 <= bottom
        && [...boxes].every(([other, b]) => other === id
          || y1 >= b.y + b.h + gapTo(other, id) || y2 + gapTo(other, id) <= b.y);
      if (!fits) return false;
      const span = (loose.length - 1) * LANDING_GAP;
      loose.forEach((n, k) => landings.set(n.i, (y1 + y2) / 2 - span / 2 + k * LANDING_GAP));
      box.y = y1;
      box.h = y2 - y1;
      box.grown = true;
      box.landings = landings;
      for (const [i, ly] of landings) if (ports[i][1]) ports[i][1].y = ly;
      return true;
    };
    /* Boxes with deliveries leaving them grow first, so the boxes placed
       freely below (fill) keep clear of the grown size. */
    for (const [id, box] of [...boxes]) sideClearOfOuts(id, box, intoOf(id));

    /* The boxes nothing leaves through go in the gap their place in
       `contains:` puts them, between the placed boxes listed before and
       after them, spread evenly there rather than packed at the top of a
       tall broker. A gap too small for them falls back to the packing
       below. */
    const free = kids.filter((c) => !boxes.has(c.id));
    const runs = [];
    let run = null;
    kids.forEach((c, k) => {
      if (!free.includes(c)) { run = null; return; }
      if (!run) { run = { before: kids[k - 1]?.id, ids: [] }; runs.push(run); }
      run.ids.push(c.id);
      run.after = kids[k + 1]?.id;
    });
    for (const r of runs) {
      const lo = r.before && boxes.has(r.before) ? boxes.get(r.before) : null;
      const hi = r.after && boxes.has(r.after) ? boxes.get(r.after) : null;
      const from = lo ? lo.y + lo.h : top;
      const to = hi ? hi.y : bottom;
      const n = r.ids.length;
      const room = to - from - n * CHILD_H;
      if (room < (n + 1) * CHILD_GAP) continue;
      // Equal space around each box, capped so a few boxes do not drift apart.
      const gap = Math.min(room / (n + 1), MAX_SPREAD);
      // A hop between two of them needs room for its line and label.
      if (r.ids.some((id, k) => k && gap < gapTo(r.ids[k - 1], id))) continue;
      let y = from + (room - gap * (n - 1)) / 2;
      const placedHere = r.ids.map((id) => {
        const box = { y, h: CHILD_H };
        y += CHILD_H + gap;
        return [id, box];
      });
      const ok = placedHere.every(([id, b]) => [...boxes].every(([other, o]) => other === id
        || b.y >= o.y + o.h + gapTo(other, id) || b.y + b.h + gapTo(other, id) <= o.y));
      if (ok) for (const [id, b] of placedHere) boxes.set(id, b);
    }

    // Any box still unplaced fills the gaps from the top, as close as fits.
    const taken = [...boxes.entries()];
    for (const c of kids) {
      if (boxes.has(c.id)) continue;
      const clash = (y) => taken.some(([id, b]) => y < b.y + b.h + gapTo(id, c.id) && y + CHILD_H + gapTo(id, c.id) > b.y);
      let y = top;
      while (clash(y) && y + CHILD_H < bottom) y += 2;
      const box = { y, h: CHILD_H };
      boxes.set(c.id, box);
      taken.push([c.id, box]);
    }
    // Publishes into a box arrive level with it.
    flows.forEach((f, i) => {
      const box = f.to === broker.id && boxes.get(f.via);
      if (box && !box.grown && ports[i][1]) ports[i][1].y = box.y + box.h / 2;
    });


    /* Two or more publishes ending in one box would arrive at almost the
       same point and read as one line. The box grows, about its middle, so
       each lands at its own height, ordered by where it comes from so they
       do not cross on the way in. Only where the taller box still clears
       the other boxes and the broker's edges; otherwise it stays as it was. */
    for (const [id, box] of boxes) {
      if (displaced.has(id) || box.grown) continue;
      const into = intoOf(id);
      if (into.length < 2) continue;
      const span = (into.length - 1) * LANDING_GAP;
      const h = Math.max(box.h, span + CHILD_H);
      const y = box.y + box.h / 2 - h / 2;
      const clear = y >= top && y + h <= bottom
        && [...boxes].every(([other, b]) => other === id
          || y >= b.y + b.h + gapTo(other, id) || y + h + gapTo(other, id) <= b.y);
      if (!clear) continue;
      box.y = y;
      box.h = h;
      box.grown = true;
      box.landings = new Map();
      into.sort((a, b) => a.fromY - b.fromY).forEach(({ i }, k) => {
        const ly = y + h / 2 - span / 2 + k * LANDING_GAP;
        box.landings.set(i, ly);
        if (ports[i][1]) ports[i][1].y = ly;
      });
    }

    /* A displaced box's lines leave from the box's own height, on the side
       facing their consumer, and curve to the consumer's near side. Flows
       into a processor have no ports of their own (they normally run on its
       lane), so the points are made here. */
    flows.forEach((f, i) => {
      if (!displaced.has(f.via) || f.from !== broker.id) return;
      const box = boxes.get(f.via);
      const to = placed.get(f.to);
      if (!to) return;
      const right = to.x > broker.x;
      ports[i] = [
        { x: right ? broker.x + broker.w : broker.x, y: box.y + box.h / 2, nx: right ? 1 : -1, ny: 0 },
        { x: right ? to.x : to.x + to.w, y: to.y + to.h / 2, nx: right ? -1 : 1, ny: 0 },
      ];
      ports[i].free = true;
    });
    return boxes;
  }

  function render(host, diagram, handlers = {}) {
    host.replaceChildren();
    const flows = diagram.flows || [];
    /* A consumer of a partitioned queue is a little taller, for the row of
       updates it handled. Copies, so the scenario's own data is untouched. */
    const lanesOf = partitionBox(diagram);
    /* Hops inside the broker (into:) take no part in placing nodes or
       routing lines between them; they are drawn between two boxes once
       the boxes are placed. The routing code sees an empty stand-in, so
       flow indexes stay the same everywhere. */
    const hops = flows.filter(isHop);
    const laid = flows.map((f) => (isHop(f) ? { id: f.id } : f));
    const nodes = (diagram.nodes || []).map((n) => (lanesOf
      && n.flowOn?.queue === lanesOf.child.partitions.queue ? { ...n, seqRow: true } : n))
      .map((n) => {
        const mine = hops.filter((f) => f.from === n.id);
        return mine.length ? { ...n, hopPairs: new Set(mine.map((f) => hopKey(f.via, f.into))) } : n;
      });
    if (!nodes.length) return;

    const { placed, canvasW, canvasH } = layout(nodes, laid);

    const svg = el("svg", {
      viewBox: `0 0 ${canvasW} ${canvasH}`,
      class: "dg",
      role: "img",
      "aria-label": diagram.caption || "Scenario data flow",
      preserveAspectRatio: "xMidYMid meet",
      style: `--dg-w: ${Math.round(canvasW * DIAGRAM_SCALE)}px; `
        + `--dg-f: ${Math.min(1, canvasW / DIAGRAM_REF_W).toFixed(4)}`,
    });

    const defs = el("defs");
    defs.append(el("marker", {
      id: "dg-arrow", viewBox: "0 0 10 10", refX: "9", refY: "5",
      markerWidth: "5", markerHeight: "5", orient: "auto-start-reverse",
    }, el("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "dg-arrowhead" })));
    // The same arrowhead, bright, for a line on a highlighted path.
    defs.append(el("marker", {
      id: "dg-arrow-on", viewBox: "0 0 10 10", refX: "9", refY: "5",
      markerWidth: "5", markerHeight: "5", orient: "auto-start-reverse",
    }, el("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "dg-arrowhead dg-arrowhead--on" })));
    svg.append(defs);

    const edges = el("g", { class: "dg-edges" });
    const packets = el("g", { class: "dg-packets" });
    const reduceMotion = window.matchMedia
      && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    const ports = assignPorts(laid, placed);
    for (const n of placed.values()) {
      if (n.kind === "broker") n.childBoxes = placeQueueBoxes(n, laid, placed, ports);
    }

    /* A direct message into a processor whose top lane already carries a
       queue's messages would run along that lane from the queue's box, and
       read as coming out of it (a request delivered to the service that
       also reads the queue). It goes into the processor's near side
       instead, level with its middle, from bare broker wall. */
    flows.forEach((f, i) => {
      const from = placed.get(f.from);
      const to = placed.get(f.to);
      if (!from || !to || from.kind !== "broker" || !isProcessor(to) || f.via) return;
      if (!flows.some((g) => g.from === f.from && g.to === f.to && g.via)) return;
      const right = to.x > from.x;
      const y = to.y + to.h / 2;
      ports[i] = [
        { x: right ? from.x + from.w : from.x, y, nx: right ? 1 : -1, ny: 0 },
        { x: right ? to.x : to.x + to.w, y, nx: right ? -1 : 1, ny: 0 },
      ];
      ports[i].free = true;
    });
    for (const n of placed.values()) {
      if (n.kind === "broker") settleBrokerEnds(n, laid, placed, ports);
    }

    /* Paths (`paths:`): the routes one message takes, as numbered steps.
       Numbers run on from one path to the next, so every number on the
       picture is different; the list under the diagram groups them. */
    const stepOf = new Map();
    const flowIndex = new Map(flows.map((f, i) => [f.id, i]));
    let number = 0;
    (diagram.paths || []).forEach((path, k) => {
      for (const st of path.steps || []) {
        const i = flowIndex.get(st.flow);
        if (i === undefined) continue;
        number += 1;
        stepOf.set(i, { path: k, number, text: st.text, label: path.label, at: st.at });
        // `also:` names the other branches of the same publish (a fork into
        // several boxes): one step, one number, every branch on the path.
        for (const id of [].concat(st.also || [])) {
          const j = flowIndex.get(id);
          if (j !== undefined) stepOf.set(j, { path: k, number, text: st.text, label: path.label, branch: true });
        }
      }
    });
    const steps = el("g", { class: "dg-steps" });

    /* A line on a path gets its step number, and a wide invisible twin
       that takes the click, since a 1px dashed line is hard to hit. */
    const addStep = (i, d, f, labelBox) => {
      const step = stepOf.get(i);
      if (!step) return false;
      const say = `Step ${step.number}, "${step.label}": ${step.text}. `
        + "Click to highlight this message's whole path. Click again, or press Esc, to clear";
      const hit = el("path", { d, class: "dg-edge__hit", "data-flow": i, "data-path": step.path });
      indicator(hit, "flow-path", say);
      edges.append(hit);
      // A branch of a step's publish: on the path, numbered once, elsewhere.
      if (step.branch) return true;
      /* Beside the label rather than on it: before it on a level line,
         and a little back along a curve from where the label sits. */
      const probe = el("path", { d });
      const len = probe.getTotalLength();
      let pt;
      if (step.at !== undefined) pt = probe.getPointAtLength(len * step.at);
      else if (labelBox?.badge) pt = labelBox.badge;
      else if (labelBox?.x !== undefined) {
        pt = { x: labelBox.x - (f.label.length * LABEL_CHAR_W) / 2 - 12, y: labelBox.y };
      } else if (labelBox) {
        const t = labelBox.level ? labelBox.at : labelBox.at > 0.4 ? labelBox.at - 0.3 : labelBox.at + 0.3;
        pt = probe.getPointAtLength(len * t);
        if (labelBox.level) pt = { x: pt.x - (f.label.length * LABEL_CHAR_W) / 2 - 12, y: pt.y };
      } else pt = probe.getPointAtLength(len * 0.5);
      const badge = el("g", { class: "dg-step", "data-flow": i, "data-path": step.path },
        el("circle", { cx: pt.x, cy: pt.y, r: 7.5 }),
        el("text", { x: pt.x, y: pt.y + 3.2, "text-anchor": "middle" }, String(step.number)));
      indicator(badge, "flow-step", say);
      steps.append(badge);
      return true;
    };

    /* Publishes from one processor into several boxes share its lane, then
       fork. Each turns up its own upright, the one going furthest outermost,
       so the uprights run side by side and never cross. */
    const runOf = new Map();
    const forks = new Map();
    flows.forEach((f, i) => {
      const from = placed.get(f.from);
      const to = placed.get(f.to);
      if (!from || !to || !isProcessor(from) || !f.via || to.kind !== "broker" || ports[i]?.free) return;
      const box = boxRectIn(to, f.via);
      if (!box) return;
      const lane = from.y + from.h + PROC_LANE;
      const y = to.childBoxes?.get(f.via)?.landings?.get(i) ?? box.y + box.h / 2;
      if (!forks.has(from.id)) forks.set(from.id, []);
      forks.get(from.id).push({ i, reach: Math.abs(lane - y) });
    });
    for (const list of forks.values()) {
      if (list.length < 2) continue;
      list.sort((a, b) => a.reach - b.reach).forEach(({ i }, k) => runOf.set(i, OUTSIDE_RUN + k * FORK_GAP));
    }

    /* A hop inside the broker: a straight line across the gap between two
       of its boxes, labelled beside it. Nothing animates on it, because it
       happens only at the moment its label names, not all the time. */
    const drawHop = (f, i) => {
      const broker = placed.get(f.from);
      const a = broker && boxRectIn(broker, f.via);
      const b = broker && boxRectIn(broker, f.into);
      if (!a || !b) return;
      const name = (id) => (broker.contains || []).find((c) => c.id === id)?.label || id;
      const x = a.x + a.w * 0.45;
      const down = b.y > a.y;
      const y0 = down ? a.y + a.h : a.y;
      const y1 = down ? b.y : b.y + b.h;
      const d = `M ${x} ${y0} V ${y1}`;
      const say = `${f.label}: the broker moves messages from ${name(f.via)} into ${name(f.into)}. `
        + "No app sends them, and nothing moves on this line, because it happens only then.";
      const line = el("path", { d, class: "dg-edge dg-edge--hop", "marker-end": "url(#dg-arrow)", "data-flow": i });
      const text = el("text", {
        x: x + 12, y: (y0 + y1) / 2 + 3, class: "dg-edge__label", "data-flow": i,
      }, f.label);
      indicator(text, "box-hop", say);
      edges.append(line, text);
      // The number beside the line, not on it: the line is short.
      if (!addStep(i, d, f, { badge: { x: x - 14, y: (y0 + y1) / 2 } })) {
        const hit = el("path", { d, class: "dg-edge__hit", "data-flow": i });
        indicator(hit, "box-hop", say);
        edges.append(hit);
      }
    };

    flows.forEach((f, i) => {
      if (isHop(f)) {
        drawHop(f, i);
        return;
      }
      const from = placed.get(f.from);
      const to = placed.get(f.to);
      // A flow naming a node that does not exist is a typo in the scenario, not
      // something to crash the page over; skip it and draw the rest.
      if (!from || !to) return;
      // Lines out of a partitioned queue are drawn per lane, from broker
      // data, by paintPartitions.
      if (lanesOf && f.via === lanesOf.child.id && from.kind === "broker") return;

      const back = isBack(from, to);
      const intoBox = f.via && to.kind === "broker" ? boxRectIn(to, f.via) : null;
      const routed = (isProcessor(from) || isProcessor(to)) && !ports[i].free
        ? (intoBox && isProcessor(from)
          ? processorIntoBox(from, to, intoBox, to.childBoxes?.get(f.via)?.landings?.get(i), runOf.get(i))
          : processorPath(from, to))
        : null;
      let d = routed ? routed.d : flowPath(...ports[i]);
      // On from the broker's edge to the queue box the publish goes into.
      if (intoBox && !routed) d += ` H ${ports[i][1].x <= to.x + to.w / 2 ? intoBox.x : intoBox.x + intoBox.w}`;
      /* A delivery starts at the queue box it comes out of, as a publish ends
         at the box it goes into: the queue sends it, not the broker as a
         whole. Only when the line leaves level with the box. */
      const outBox = f.via && from.kind === "broker" && to.kind !== "broker" ? boxRectIn(from, f.via) : null;
      const start = outBox && d.match(/^M\s*(-?[\d.]+)[\s,]+(-?[\d.]+)/);
      if (start && +start[2] >= outBox.y && +start[2] <= outBox.y + outBox.h) {
        const edge = +start[1] >= from.x + from.w / 2 ? outBox.x + outBox.w : outBox.x;
        if (Math.abs(edge - +start[1]) > 0.5) d = `M ${edge} ${start[2]} H ${start[1]}` + d.slice(start[0].length);
      }
      const pathId = `dg-flow-${i}`;

      edges.append(el("path", {
        id: pathId, d, class: "dg-edge", "marker-end": "url(#dg-arrow)", "data-flow": i,
      }));

      let labelBox = null;
      // A fork's number goes on its own upright: the lane is shared.
      if (routed?.badge && runOf.has(i)) labelBox = { badge: routed.badge };
      else if (f.label && routed) labelBox = { x: routed.label.x, y: routed.label.y + 6, level: true };
      if (f.label && routed) {
        // A processor lane is level, so its label sits flat along it.
        edges.append(el("text", {
          x: routed.label.x, y: routed.label.y, class: "dg-edge__label", "text-anchor": "middle",
          "data-flow": i,
        }, f.label));
      } else if (f.label) {
        /* Set flat rather than along the path: a textPath rotates with the
           curve, and on a steep fan-out the labels end up near-vertical and
           unreadable.

           Position is sampled from the curve itself at a point that varies with
           how far the flow rises or falls, which spreads a fan-out's labels
           along their own paths instead of stacking them at one x. */
        /* Anchor the label to the destination end of the curve. A fan-out's
           paths all leave the broker from the same point but arrive at well
           separated ones, so labelling near the arrival keeps them apart;
           a level flow has no such crowding and reads best labelled midway. */
        const rise = Math.abs(ports[i][1].y - ports[i][0].y);
        /* A flow back to the broker is labelled near the app that sends it,
           and below its line: the app end is where it is distinct from the
           other flows arriving at the broker, and below keeps it clear of the
           label on the consume running the other way above it. */
        // labelAt: on the flow overrides it, for a long label near a node.
        const at = f.labelAt ?? (rise < 4 ? 0.5 : back ? 0.35 : 0.78);
        labelBox = { at, level: rise < 4 };

        // Path geometry is measurable without being in the document, so this
        // works while the svg is still being assembled.
        const probe = el("path", { d });
        const pt = probe.getPointAtLength(probe.getTotalLength() * at);

        edges.append(el("text", {
          x: pt.x, y: back ? pt.y + 16 : pt.y - 9, class: "dg-edge__label", "text-anchor": "middle",
          "data-flow": i,
        }, f.label));
      }

      addStep(i, d, f, labelBox);

      /* The moving part. Each flow's packet is offset in time so a fan-out
         reads as one message being copied outward rather than three unrelated
         dots, and `dur` is uniform so speed stays consistent across scenarios. */
      // In a diagram drawn from real events, only real events move.
      if (diagram.events) return;
      const packet = el("circle", { r: 5, class: "dg-packet", "data-flow": i });

      /* SMIL animation is not controlled by CSS, so honouring reduced motion
         means not creating the animation at all. The packet is parked midway
         along its path instead, which keeps the picture readable. */
      if (reduceMotion) {
        const probe = el("path", { d });
        const rest = probe.getPointAtLength(probe.getTotalLength() * 0.5);
        packet.setAttribute("cx", rest.x);
        packet.setAttribute("cy", rest.y);
        packets.append(packet);
        return;
      }

      /* Hidden until its first run starts: before `begin` an animated circle
         with no position of its own sits at the canvas origin. */
      const begin = `${f.delay ?? i * 0.35}s`;
      packet.setAttribute("visibility", "hidden");
      packet.append(el("set", { attributeName: "visibility", to: "visible", begin }));
      packet.append(el("animateMotion", {
        dur: `${diagram.duration || 2.4}s`,
        repeatCount: "indefinite",
        begin,
        path: d,
        keyPoints: "0;1",
        keyTimes: "0;1",
        calcMode: "linear",
      }));
      packets.append(packet);
    });

    const nodeLayer = el("g", { class: "dg-nodes" });
    for (const n of placed.values()) nodeLayer.append(drawNode(n, handlers));

    if (lanesOf) {
      const pub = flows.findIndex((f) => f.to === lanesOf.node.id && !isBack(placed.get(f.from) || {}, placed.get(f.to) || {}));
      svg.__parts = partsContext(placed, lanesOf, edges, packets,
        pub >= 0 ? ports[pub][0] : null, reduceMotion);
    }

    // Edges beneath nodes, packets above, so a packet passing a box is visible
    // but a line never crosses a label. Step numbers sit on top of both.
    svg.append(edges, nodeLayer, packets, steps);

    if (stepOf.size) {
      svg.classList.add("has-paths");
      svg.__paths = { diagram, flows, onPath: handlers.onPath };
      svg.addEventListener("click", (e) => {
        const on = e.target.closest?.("[data-path]");
        if (on) {
          const k = Number(on.dataset.path);
          selectPath(host, svg.__paths.current === k ? null : k);
        } else if (!e.target.closest?.(".dg-node")) {
          selectPath(host, null);
        }
      });
    }

    const figure = document.createElement("figure");
    figure.className = "dg-figure";
    figure.append(svg);
    if (diagram.caption) {
      const cap = document.createElement("figcaption");
      cap.className = "dg-caption";
      cap.textContent = diagram.caption;
      figure.append(cap);
    }
    host.append(figure);
  }

  /* Highlight one path (index into diagram.paths), or none. Everything on
     it stays bright: its lines, labels, packets and numbers, the nodes it
     passes and the queue boxes it goes through. The rest dims, so the
     message's whole route reads at a glance. */
  function selectPath(host, k) {
    const svg = host.querySelector(".dg");
    const ctx = svg?.__paths;
    if (!ctx) return;
    const path = k === null || k === undefined ? null : ctx.diagram.paths?.[k];
    ctx.current = path ? k : null;
    svg.classList.toggle("has-path", Boolean(path));
    for (const x of svg.querySelectorAll(".on-path")) x.classList.remove("on-path");
    for (const x of svg.querySelectorAll(".dg-edge[marker-end]")) x.setAttribute("marker-end", "url(#dg-arrow)");
    if (path) {
      const index = new Map(ctx.flows.map((f, i) => [f.id, i]));
      for (const id of (path.steps || []).flatMap((st) => [st.flow, ...[].concat(st.also || [])])) {
        const i = index.get(id);
        if (i === undefined) continue;
        const f = ctx.flows[i];
        for (const x of svg.querySelectorAll(`[data-flow="${i}"]`)) x.classList.add("on-path");
        svg.querySelector(`.dg-edge[data-flow="${i}"]`)?.setAttribute("marker-end", "url(#dg-arrow-on)");
        for (const id of [f.from, f.to]) svg.querySelector(`.dg-node[data-node="${id}"]`)?.classList.add("on-path");
        if (f.via) svg.querySelector(`[data-child="${f.via}"]`)?.classList.add("on-path");
        if (f.into) svg.querySelector(`[data-child="${f.into}"]`)?.classList.add("on-path");
      }
    }
    ctx.onPath?.(ctx.current);
  }

  /* Apply live state to an already-rendered diagram.

     Called on every poll, so it only ever updates attributes -- re-rendering
     would restart the SMIL animations and make the packets stutter. */
  /* Which flows carry traffic right now, as a Set of flow indexes.

     A node with a long-running action is live only while that process runs;
     anything else (the broker, a terraform node) is always live. An app
     publishes when it is live and either is a source (nothing flows into it)
     or one of the flows into it is carrying traffic. A flow from the broker
     to an app needs that app running and something feeding it: the nodes in
     the flow's `source:` if given, otherwise any source publishing to the
     broker. `source:` is how a consumer at the end of a processor chain says
     it only sees what the processor before it republished. */
  function liveFlows(diagram, state, gated) {
    const flows = diagram.flows || [];
    const running = (id) => !gated.has(id) || Boolean(state.nodes?.[id]?.running);
    const brokerId = (diagram.nodes || []).find((n) => n.kind === "broker")?.id;
    const into = (id) => flows.filter((f) => f.to === id);
    const isSource = (id) => !into(id).some((f) => f.from === brokerId);
    const sources = [...new Set(flows
      .filter((f) => f.to === brokerId && isSource(f.from)).map((f) => f.from))];

    const memo = new Map();
    const publishing = (id, seen = new Set()) => {
      if (memo.has(id)) return memo.get(id);
      if (seen.has(id)) return false;
      seen.add(id);
      const on = running(id) && (isSource(id)
        || into(id).some((f) => flowLive(f, seen)));
      memo.set(id, on);
      return on;
    };
    const flowLive = (f, seen = new Set()) => {
      // A flow that only carries the odd message (to a dead message queue,
      // say) is drawn but never animated, so it never looks busy.
      if (f.animate === false || isHop(f)) return false;
      if (f.from !== brokerId) return publishing(f.from, seen);
      if (!running(f.to)) return false;
      // Tied to a queue: moves only while messages are arriving on it, as the
      // broker counts them (across a bridge, say, which no app here runs).
      if (f.traffic && state.traffic?.[f.traffic] === false) return false;
      // Bound but not delivered to: a standby on an exclusive queue. A
      // stalled consumer is at its unacknowledged limit, so the broker is
      // not sending it anything either.
      const flow = state.nodes?.[f.to]?.flow;
      if (flow === "standby" || flow === "stalled") return false;
      const feed = f.source ? [].concat(f.source) : sources;
      return feed.some((s) => publishing(s, new Set(seen)));
    };

    const live = new Set();
    flows.forEach((f, i) => { if (flowLive(f)) live.add(i); });
    return live;
  }

  /* "2/3" spelled out, with the objects behind it, so the count explains
     itself and says what to do when something is missing. */
  function configTip(present, list, holder = false) {
    const name = (c) => c.label;
    const have = list.filter((c) => c.present).map(name);
    const lack = list.filter((c) => c.present === false).map(name);
    const unknown = list.filter((c) => c.present !== true && c.present !== false).map(name);
    const lines = [holder
      ? `${present} of ${list.length} objects this scenario creates exist on the broker right now.`
      : `${present} of ${list.length} broker objects this app needs exist on the broker right now.`];
    if (have.length) lines.push(`Present: ${have.join(", ")}`);
    if (lack.length) lines.push(`Missing: ${lack.join(", ")}. Press Play, or Apply configuration.`);
    if (unknown.length) lines.push(`Not checked yet (broker not answering): ${unknown.join(", ")}`);
    return lines.join("\n");
  }

  /* `gated` lists the node ids whose action is a long-running process, so
     their packets follow whether that process is running. */
  function paint(host, diagram, state, gated = new Set()) {
    const svg = host.querySelector(".dg");
    if (!svg || !state) return;

    for (const node of svg.querySelectorAll(".dg-node")) {
      const id = node.dataset.node;
      const s = state.nodes?.[id];
      if (!s) continue;

      const run = s.run;
      const status = s.running
        ? "running"
        : run && run.state === "failed"
        ? "failed"
        : run && run.state === "succeeded"
        ? "done"
        : s.ready === false
        ? "missing"
        : "idle";
      node.setAttribute("data-status", status);

      const holdsEl = node.querySelector(".dg-node__holds");
      if (holdsEl) {
        const spec = (diagram.nodes || []).find((x) => x.id === id);
        const unit = spec?.holds?.count || "";
        const rows = s.running && s.holds ? (s.holds.rows || []).length : null;
        holdsEl.textContent = rows === null ? "" : rows === 1 ? `1 ${unit.replace(/s$/, "")}` : `${rows} ${unit}`;
        holdsEl.setAttribute("data-empty", String(rows === 0));
        indicator(holdsEl, "hold-count", rows === null ? null
          : `${spec.label} holds ${holdsEl.textContent} right now, as it last reported. The table under the diagram lists them`);
      }
      // The broker's action is applying configuration, so its states read
      // as that rather than as an app's.
      const holder = node.classList.contains("is-broker");
      indicator(node.querySelector(".dg-node__dot"), "status-dot", {
        running: holder ? "Applying configuration" : "Running",
        failed: "Stopped with an error. Open its logs to see why",
        done: holder ? "Configuration applied" : "Finished successfully",
        missing: "Not running, and some broker configuration it needs is missing. Press Play, or Apply configuration",
        idle: "Not running",
      }[status]);

      /* A node with `flowOn` says what the broker decided about its flow:
         the active consumer, or a standby the queue is not delivering to.
         The sublabel carries it while the app runs, and goes back to the
         yaml's own text when it stops. */
      const flow = s.running ? s.flow : null;
      node.setAttribute("data-flow", flow || "");
      const sub = node.querySelector(".dg-node__sub");
      // Every consumer of a partitioned queue is "active" to the broker;
      // what it owns is the useful part, and paintPartitions says that.
      if (sub && !node.hasAttribute("data-partitioned")) {
        if (sub.dataset.text === undefined) sub.dataset.text = sub.textContent;
        sub.textContent = flow === "active" ? "active consumer"
          : flow === "stalled" ? "active, not acking"
          : flow === "standby" ? "standby"
          : sub.dataset.text;
        indicator(sub, "flow-role", flow === "active"
          ? "The broker is delivering this queue's messages to this app"
          : flow === "stalled"
          ? "The broker still treats this app as an active consumer, but it is holding messages and has acknowledged none for several seconds. The broker sends it nothing more, and the messages it holds wait until it disconnects"
          : flow === "standby"
          ? "Bound to the queue, but the broker is delivering to another consumer. This app gets nothing until that one goes away"
          : null);
      }

      // "3/5" reads faster than a list, and the panel has the detail.
      const list = s.checklist || [];
      const check = node.querySelector(".dg-node__check");
      if (check) {
        const present = list.filter((c) => c.present).length;
        check.textContent = list.length ? `${present}/${list.length}` : "";
        check.setAttribute("data-complete", String(list.length > 0 && present === list.length));
        indicator(check, "config-count", list.length ? configTip(present, list, holder) : null);
      }

      /* Contained configuration gets its own count, so each dotted box says
         whether the objects it stands for are actually on the broker. A child
         claims the checklist entries whose group matches its id, which is how
         one node's flat checklist is split across several boxes. */
      for (const countEl of node.querySelectorAll("[data-child-count]")) {
        const childId = countEl.getAttribute("data-child-count");
        if (!childId) continue;
        const mine = list.filter((c) => c.group === childId);
        if (!mine.length) continue;
        const present = mine.filter((c) => c.present).length;
        countEl.textContent = `${present}/${mine.length}`;
        countEl.setAttribute("data-complete", String(present === mine.length));
        indicator(countEl, "config-count", configTip(present, mine, holder));
      }
    }

    paintBoxes(svg, diagram, state);

    /* Packets move only on flows that carry traffic, so a stopped app never
       has dots arriving at it or leaving it. `liveWhen` still gates the whole
       picture on one node, for scenarios that want that. */
    const gate = diagram.liveWhen;
    const flowing = !gate || Boolean(state.nodes?.[gate]?.running);
    svg.classList.toggle("is-idle", !flowing);
    const live = liveFlows(diagram, state, gated);
    for (const packet of svg.querySelectorAll(".dg-packet[data-flow]")) {
      packet.classList.toggle("is-idle", !live.has(Number(packet.dataset.flow)));
    }
  }

  /* ---------------------------------------------- Queue depth and arrivals

     A queue box can show how many messages it holds (`depth:`), and a dead
     message queue box can show each message that arrives (`arrivals:`).
     Both come from the broker on each poll: the depth from the queue's
     message list, the arrivals from counters on the queue messages leave
     (maxTtlExpiredToDmqMsgCount and the like), which also say why. Each
     rise in a counter is drawn as a dot hopping from one box to the other,
     labelled with the reason, then on along the line to whoever reads that
     queue, if it is running. Nothing is drawn for counts the page saw
     before it opened. */

  function boxRect(svg, id) {
    const r = svg.querySelector(`[data-child="${id}"] > .dg-node__child-box`);
    return r ? r.getBBox() : null;
  }

  function paintBoxes(svg, diagram, state) {
    const boxes = state.boxes || {};
    const specs = new Map((diagram.nodes || []).flatMap((n) => (n.contains || []).map((c) => [c.id, c])));
    const seen = svg.__arrivals || (svg.__arrivals = new Map());
    for (const [id, live] of Object.entries(boxes)) {
      const spec = specs.get(id);
      if (!spec) continue;
      const depthEl = svg.querySelector(`[data-child-depth="${id}"]`);
      if (depthEl && spec.depth) {
        const unit = spec.depth.unit || "held";
        depthEl.textContent = live.depth === null ? "" : `${live.depth} ${unit}`;
        depthEl.setAttribute("data-busy", String((live.depth || 0) > 0));
        indicator(depthEl, "box-depth", live.depth === null
          ? `The broker did not say how many messages ${spec.depth.queue} holds. Press Play if the queue is not there yet.`
          : `${live.depth} message(s) on ${spec.depth.queue} right now: waiting to be read, or delivered and not yet acknowledged. Read from the broker every few seconds.`);
      }
      if (!spec.arrivals || !live.arrivals) continue;
      const before = seen.get(id);
      seen.set(id, { ...live.arrivals });
      if (!before) continue;
      /* Each reason has its own counter, so a hop's label is that
         counter's. When several move between two polls, all their hops run
         one after another, never on top of each other, and at most three
         per reason are drawn: the last one says how many more there were.
         A counter that went down means the queue was recreated: nothing
         moved. */
      let slot = 0;
      for (const [reason, count] of Object.entries(live.arrivals)) {
        const rise = (count ?? 0) - (before[reason] ?? 0);
        const drawn = Math.min(rise, 3);
        for (let k = 0; k < drawn; k++) {
          const label = k === drawn - 1 && rise > drawn ? `${reason}, +${rise - drawn} more` : reason;
          setTimeout(() => hop(svg, diagram, state, spec, label), slot++ * HOP_GAP_MS);
        }
      }
    }
  }

  // Time between one hop and the next, long enough for its label to be read.
  const HOP_GAP_MS = 1100;

  function hop(svg, diagram, state, spec, reason) {
    const from = boxRect(svg, spec.arrivals.from);
    const to = boxRect(svg, spec.id);
    const layer = svg.querySelector(".dg-packets");
    if (!from || !to || !layer) return;
    const x1 = from.x + from.width, y1 = from.y + from.height / 2;
    const x2 = to.x + to.width, y2 = to.y + to.height / 2;
    let d = `M ${x1 - 8} ${y1} C ${x1 + 26} ${y1}, ${x2 + 26} ${y2}, ${x2 - 8} ${y2}`;
    // On along the line to the reader of this queue, if it is running.
    const flows = diagram.flows || [];
    const brokerId = (diagram.nodes || []).find((n) => n.kind === "broker")?.id;
    const i = flows.findIndex((f) => f.via === spec.id && f.from === brokerId && state.nodes?.[f.to]?.running);
    const line = i >= 0 ? svg.querySelector(`#dg-flow-${i}`) : null;
    const m = line?.getAttribute("d").match(/^M\s*([-\d.]+)[\s,]+([-\d.]+)\s*(.*)$/);
    if (m) d += ` L ${m[1]} ${m[2]} ${m[3]}`;
    const g = el("g", { class: "dg-hop" });
    g.append(el("circle", { r: 6 }));
    indicator(g, "box-arrival", `A message moved to ${spec.label}: ${reason}. Counted by the broker on the queue it left.`);
    layer.append(g);
    // One label per box at a time: the newest hop's replaces the last.
    for (const old of layer.querySelectorAll(`.dg-hop__label[data-box="${spec.id}"]`)) old.remove();
    const label = el("text", { x: x2 + 12, y: y2 - 8, class: "dg-hop__label", "text-anchor": "start",
      "data-box": spec.id }, reason);
    indicator(label, "box-arrival", `A message moved to ${spec.label}: ${reason}. Counted by the broker on the queue it left.`);
    layer.append(label);
    const box = svg.querySelector(`[data-child="${spec.id}"]`);
    box?.classList.add("is-arriving");
    setTimeout(() => { box?.classList.remove("is-arriving"); label.remove(); }, 3500);
    const path = el("path", { d });
    const len = path.getTotalLength();
    const t0 = performance.now();
    const ms = m ? 2200 : 900;
    const step = (now) => {
      const t = Math.min(1, (now - t0) / ms);
      const pt = path.getPointAtLength(len * t);
      g.setAttribute("transform", `translate(${pt.x} ${pt.y})`);
      if (t < 1) requestAnimationFrame(step);
      else g.remove();
    };
    requestAnimationFrame(step);
  }

  /* ------------------------------------------------- Partitioned queues

     A box in the broker with `partitions: {queue, count}` is drawn as one
     lane per partition. Everything on it comes from the dashboard's
     /partitions answer (cockpit/app/partitions.py), never from a timer:

     - each lane's owner, from the broker (partitionClientName), with a line
       from the lane to that consumer, which swings across when the broker
       moves the partition;
     - each lane's counts, from the broker: messages it holds now, and how
       many it has taken in;
     - one dot per update a consumer reported handling, coloured and
       lettered by its order, run along the path it took: from the
       publisher, through the partition the broker held it in, to the
       consumer that logged it. It is drawn a moment after it happened;
     - the last few updates each consumer handled, with their sequence
       numbers, flagged when its own log says there was a gap or a step
       backwards. */

  function partitionBox(diagram) {
    for (const node of diagram.nodes || []) {
      for (const child of node.contains || []) {
        if (child.partitions) return { node, child };
      }
    }
    return null;
  }

  /* Ten colours, measured to stay apart: every pair is at least 24 apart
     in CIEDE2000 with lightness given half weight, so two shades of one hue
     never count as different, and each is far from the dashboard's dark
     backgrounds. check_scenarios.py measures them again, and fails a pace
     that keeps more orders in flight than there are colours. More than ten
     cannot be told apart on this background. They are ordered so letters
     handed out one after another look nothing alike; dark text reads on
     all of them. */
  const KEY_COLOURS = [
    "#EEFF00", "#00BBFF", "#FF6622", "#22AA44", "#FF00FF",
    "#00FFEE", "#FF99AA", "#AAAAFF", "#BB8811", "#FFFFFF",
  ].map((c) => [c, "#1b1b1b"]);
  const LETTERS = "ABCDEFGHIJ";
  // How long a dot takes from publisher to consumer, and the most dots a
  // burst may queue before older ones are dropped rather than replayed late.
  const DOT_MS = 1700;
  const CHIPS = 5;
  /* A handled update leaves the consumer's row after a while, so the row
     shows recent work and never reads as a backlog. A flagged one stays
     longer, so a gap is still there to point at. */
  const CHIP_MS = 6000;
  const FLAGGED_CHIP_MS = 20000;
  // The last part of a dot's trip, over which it is absorbed by the consumer.
  const ABSORB = 0.12;
  const MOVE_MS = 900;
  // An order not seen for this long has given up its letter.
  const QUIET_MS = 15000;

  function laneY(box, p) {
    return box.top + PART_HEAD + p * (LANE_H + LANE_GAP);
  }

  function drawLanes(kid, n, child, cyTop) {
    const count = Number(child.partitions.count) || 1;
    const x = n.x + CHILD_INSET + 6;
    const w = n.w - CHILD_INSET * 2 - 12;
    kid.append(el("text", {
      x: n.x + n.w - CHILD_INSET - 40, y: cyTop + 17,
      class: "dg-part__status", "text-anchor": "end", "data-part-status": "",
    }, ""));
    for (let p = 0; p < count; p++) {
      const y = cyTop + PART_HEAD + p * (LANE_H + LANE_GAP);
      const lane = el("g", { class: "dg-lane", "data-lane": p });
      lane.append(el("rect", { x, y, width: w, height: LANE_H, rx: 4, class: "dg-lane__box" }));
      lane.append(el("text", { x: x + 6, y: y + 13, class: "dg-lane__num" }, `P${p}`));
      lane.append(el("text", { x: x + 30, y: y + 13, class: "dg-lane__owner", "data-lane-owner": p }, ""));
      lane.append(el("text", { x: x + w - 52, y: y + 13, class: "dg-lane__held",
        "text-anchor": "end", "data-lane-held": p }, ""));
      lane.append(el("text", { x: x + w - 6, y: y + 13, class: "dg-lane__in",
        "text-anchor": "end", "data-lane-in": p }, ""));
      kid.append(lane);
    }
  }

  /* Geometry and memory for one rendered diagram, kept on the svg so a
     re-render starts clean. */
  function partsContext(placed, lanesOf, edges, packets, pubPort, reduceMotion) {
    const broker = placed.get(lanesOf.node.id);
    let top = broker.y + CHILD_TOP;
    for (const c of lanesOf.node.contains) {
      if (c === lanesOf.child) break;
      top += childHeight(c) + CHILD_GAP;
    }
    const count = Number(lanesOf.child.partitions.count) || 1;
    const laneEdges = el("g", { class: "dg-lane-edges" });
    const paths = [];
    for (let p = 0; p < count; p++) {
      const path = el("path", { class: "dg-edge dg-lane-edge", "data-lane-edge": p, d: "" });
      laneEdges.append(path);
      paths.push(path);
    }
    edges.append(laneEdges);
    return {
      placed, broker, box: { top, count }, packets, pubPort, reduceMotion,
      queue: lanesOf.child.partitions.queue, paths,
      // lane -> {node, end: {x, y}} as last drawn
      owners: new Map(),
      letters: new Map(),     // order id -> {i, last}
      homes: new Map(),       // letter -> the partition its orders went to
      lastUse: new Map(),     // letter -> when it was last drawn
      chips: new Map(),       // node id -> [event], oldest first
      drains: new Map(),      // node id -> timer for the next chip to leave
      primed: false,
      clock: 0,               // newest event time already drawn
    };
  }

  // Where a lane's line meets a consumer: its owned lanes spread down its
  // left edge, in lane order.
  function consumerPort(ctx, nodeId, lane, owned) {
    const n = ctx.placed.get(nodeId);
    const mine = owned.filter((o) => o.node === nodeId).map((o) => o.partition);
    const k = Math.max(0, mine.indexOf(lane));
    const span = Math.min(n.h - 30, (mine.length - 1) * 12);
    return { x: n.x, y: n.y + n.h / 2 - span / 2 + (mine.length > 1 ? k * span / (mine.length - 1) : 0) };
  }

  function laneEdgePath(ctx, lane, end) {
    const y = laneY(ctx.box, lane) + LANE_H / 2;
    return flowPath({ x: ctx.broker.x + ctx.broker.w, y, nx: 1, ny: 0 }, { ...end, nx: -1, ny: 0 });
  }

  function stubEnd(ctx, lane) {
    return { x: ctx.broker.x + ctx.broker.w + 22, y: laneY(ctx.box, lane) + LANE_H / 2 };
  }

  /* Swing a lane's line from where it was drawn to where it belongs now. */
  function moveEdge(ctx, lane, to, owned) {
    const path = ctx.paths[lane];
    const before = ctx.owners.get(lane);
    const end = to ? consumerPort(ctx, to, lane, owned) : stubEnd(ctx, lane);
    path.classList.toggle("is-unowned", !to);
    ctx.owners.set(lane, { node: to, end });
    const from = before?.end;
    if (!from || ctx.reduceMotion || (before.node === to)) {
      path.setAttribute("d", laneEdgePath(ctx, lane, end));
      return;
    }
    const t0 = performance.now();
    const step = (now) => {
      const t = Math.min(1, (now - t0) / MOVE_MS);
      const e = t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2;
      path.setAttribute("d", laneEdgePath(ctx, lane,
        { x: from.x + (end.x - from.x) * e, y: from.y + (end.y - from.y) * e }));
      if (t < 1 && ctx.owners.get(lane)?.end === end) requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
  }

  /* The letter (and colour) for an update's order. A letter stands for one
     order while it is in flight, and orders keep starting, so letters are
     reused. To keep the picture true, a letter has a home partition, taken
     from where the broker placed its keyed orders, and a new order reuses
     a free letter from its own partition (or, before its partition is
     known, one from the partitions its consumer owns). So in steady
     running a letter only ever shows at one consumer; one letter at two
     consumers means one order really did split. An order gives its letter
     back when it is delivered (releaseIfDone), or when it has been quiet so
     long it was abandoned (its publisher restarted). */
  function letterFor(ctx, e, owned) {
    const now = performance.now();
    let got = ctx.letters.get(e.order);
    if (!got) {
      for (const [o, v] of ctx.letters) if (now - v.last > QUIET_MS) ctx.letters.delete(o);
      const held = new Map([...ctx.letters.values()].map((v) => [v.i, v.last]));
      const placed = e.partition !== null && e.partition !== undefined;
      const want = placed ? [e.partition]
        : (owned || []).filter((o) => o.node === e.node).map((o) => o.partition);
      const free = [...LETTERS].map((_, i) => i).filter((i) => !held.has(i));
      const home = (i) => ctx.homes.get(i);
      const lastUse = (i) => ctx.lastUse.get(i) ?? 0;
      const byAge = (a, b) => lastUse(a) - lastUse(b);
      const best = free.filter((i) => want.includes(home(i))).sort(byAge)[0]
        ?? free.find((i) => home(i) === undefined)
        ?? free.sort(byAge)[0]
        ?? [...held.entries()].sort((a, b) => a[1] - b[1])[0][0];
      for (const [o, v] of ctx.letters) if (v.i === best) ctx.letters.delete(o);
      got = { i: best, last: 0 };
      ctx.letters.set(e.order, got);
    }
    if (e.key && e.partition !== null && e.partition !== undefined) ctx.homes.set(got.i, e.partition);
    got.last = now;
    ctx.lastUse.set(got.i, now);
    return got.i;
  }

  /* A delivered order gives its letter back as soon as its last update is
     read, so the next order in that partition can take it straight away
     and a few orders in flight keep to a few colours. The update's own dot
     and chip keep the letter they were given. */
  function releaseIfDone(ctx, e) {
    if (e.seq === 5 && ctx.letters.get(e.order)?.i === e.letter) ctx.letters.delete(e.order);
  }

  function flagText(e) {
    if (e.flags.includes("gap")) return "Gap: this consumer's log says it saw this order's sequence numbers jump, so an update went to another consumer.";
    if (e.flags.includes("order")) return "Out of order: this consumer's log says this update came after a later one for the same order.";
    return "";
  }

  function eventTip(e, letter) {
    const where = e.partition === null || e.partition === undefined
      ? "The dashboard did not catch which partition the broker held it in."
      : `Through partition P${e.partition}.`;
    const extra = [];
    if (e.flags.includes("picked")) extra.push("This consumer picked the order up part way: earlier updates went to another consumer.");
    if (e.flags.includes("redelivered")) extra.push("Delivered again after a consumer went away without acknowledging it.");
    const flag = flagText(e);
    return [`${letter} is order ${e.order}, update #${e.seq} (${e.status}), ${e.key ? `key ${e.key}` : "no partition key"}. ${where}`,
      flag, ...extra, `Log line: ${e.text.trim()}`].filter(Boolean).join("\n");
  }

  function dotPath(ctx, e, owned) {
    const handler = ctx.placed.get(e.node);
    if (!handler || !ctx.pubPort) return null;
    const known = e.partition !== null && e.partition !== undefined && e.partition < ctx.box.count;
    const y = known ? laneY(ctx.box, e.partition) + LANE_H / 2
      : ctx.box.top + childHeight({ partitions: { count: ctx.box.count } }) / 2;
    const bx1 = ctx.broker.x;
    const bx2 = ctx.broker.x + ctx.broker.w;
    const p = ctx.pubPort;
    const k = Math.max(28, (bx1 - p.x) / 2);
    const end = known && owned.some((o) => o.partition === e.partition && o.node === e.node)
      ? consumerPort(ctx, e.node, e.partition, owned)
      : { x: handler.x, y: handler.y + handler.h / 2 };
    const k2 = Math.max(28, (end.x - bx2) / 2);
    return `M ${p.x} ${p.y} C ${p.x + k} ${p.y}, ${bx1 - k} ${y}, ${bx1} ${y} L ${bx2} ${y} `
      + `C ${bx2 + k2} ${y}, ${end.x - k2} ${end.y}, ${end.x} ${end.y}`;
  }

  function launchDot(ctx, e, owned, delay, onArrive) {
    const i = e.letter;
    const d = dotPath(ctx, e, owned);
    if (!d || ctx.reduceMotion) { onArrive(); return; }
    const known = e.partition !== null && e.partition !== undefined;
    const g = el("g", { class: `dg-dot${known ? "" : " is-unplaced"}` });
    g.append(el("circle", { r: 7, fill: KEY_COLOURS[i][0] }));
    g.append(el("text", { class: "dg-dot__letter", "text-anchor": "middle", y: 3, fill: KEY_COLOURS[i][1] }, LETTERS[i]));
    if (known) indicator(g, "event-dot", eventTip(e, LETTERS[i]));
    else indicator(g, "event-dot-unplaced", eventTip(e, LETTERS[i]));
    g.setAttribute("visibility", "hidden");
    ctx.packets.append(g);
    const path = el("path", { d });
    const len = path.getTotalLength();
    const t0 = performance.now() + delay;
    const step = (now) => {
      const t = (now - t0) / DOT_MS;
      if (t < 0) { requestAnimationFrame(step); return; }
      if (t >= 1 || !g.isConnected) { g.remove(); onArrive(); return; }
      const pt = path.getPointAtLength(len * t);
      // Consumed: it shrinks and fades into the consumer as it arrives.
      const left = Math.min(1, (1 - t) / ABSORB);
      g.setAttribute("transform", `translate(${pt.x} ${pt.y}) scale(${0.3 + 0.7 * left})`);
      g.setAttribute("opacity", String(left));
      g.setAttribute("visibility", "visible");
      requestAnimationFrame(step);
    };
    requestAnimationFrame(step);
  }

  /* A consumer's row works like a queue: an update comes in at the left
     end of the line, the line is packed against the right end, and updates
     leave only from the right end, oldest first, sliding the rest along.
     Each chip keeps its element, so it moves rather than being redrawn. */
  const CHIP_W = 27;

  function chipSlotX(n, k) {
    // The oldest (k = 0) sits in the rightmost slot.
    return n.x + 10 + (CHIPS - 1 - k) * CHIP_W;
  }

  function drawChips(svg, ctx, nodeId) {
    const row = svg.querySelector(`[data-seq="${nodeId}"]`);
    const n = ctx.placed.get(nodeId);
    if (!row || !n) return;
    const list = ctx.chips.get(nodeId) || [];
    const y = n.y + 55;
    list.forEach((e, k) => {
      const x = chipSlotX(n, k);
      if (!e.el) {
        const i = e.letter;
        const flagged = e.flags.includes("gap") || e.flags.includes("order");
        const chip = el("g", { class: `dg-chip is-entering${flagged ? " is-flagged" : ""}` });
        chip.append(el("rect", { x: 0, y: 0, width: 25, height: 15, rx: 4, fill: KEY_COLOURS[i][0] }));
        chip.append(el("text", { x: 12.5, y: 11, "text-anchor": "middle",
          class: "dg-chip__text", fill: KEY_COLOURS[i][1] }, `${LETTERS[i]}${e.seq}${flagged ? "!" : ""}`));
        if (flagged) indicator(chip, "seq-chip-flagged", eventTip(e, LETTERS[i]));
        else indicator(chip, "seq-chip", eventTip(e, LETTERS[i]));
        chip.style.transform = `translate(${x - 6}px, ${y}px)`;
        row.append(chip);
        e.el = chip;
        // Next frame, so the browser sees where it started from.
        requestAnimationFrame(() => requestAnimationFrame(() => {
          chip.classList.remove("is-entering");
          chip.style.transform = `translate(${e.x}px, ${y}px)`;
        }));
      } else {
        e.el.style.transform = `translate(${x}px, ${y}px)`;
      }
      e.x = x;
    });
  }

  function leaveChip(e, n) {
    const chip = e.el;
    if (!chip) return;
    chip.classList.add("is-leaving");
    // A short slide: the rightmost slot ends a few pixels inside the card.
    chip.style.transform = `translate(${e.x + 5}px, ${n.y + 55}px)`;
    setTimeout(() => chip.remove(), 500);
  }

  /* Updates leave the right end in order once their time is up: a chip
     whose time has come waits behind an older one still being held (a
     flagged chip stays longer, so a gap can be pointed at). */
  function drainChips(svg, ctx, nodeId) {
    const list = ctx.chips.get(nodeId) || [];
    const n = ctx.placed.get(nodeId);
    clearTimeout(ctx.drains.get(nodeId));
    const now = performance.now();
    while (list.length && now >= list[0].due) leaveChip(list.shift(), n);
    drawChips(svg, ctx, nodeId);
    if (list.length) {
      ctx.drains.set(nodeId, setTimeout(() => drainChips(svg, ctx, nodeId),
        Math.max(50, list[0].due - now)));
    }
  }

  function pushChip(svg, ctx, e) {
    const list = ctx.chips.get(e.node) || [];
    const flagged = e.flags.includes("gap") || e.flags.includes("order");
    e.due = performance.now() + (flagged ? FLAGGED_CHIP_MS : CHIP_MS);
    list.push(e);
    ctx.chips.set(e.node, list);
    const n = ctx.placed.get(e.node);
    // A full row pushes its oldest out of the right end.
    while (list.length > CHIPS) leaveChip(list.shift(), n);
    drainChips(svg, ctx, e.node);
  }

  const STATUS = {
    ready: ["", "The broker has given every partition to a consumer and nothing is waiting to move."],
    holddown: ["waiting to rebalance", "A consumer came or went. The broker waits the rebalance delay (5 seconds here) before it moves any partition, in case the consumer comes straight back."],
    rebalancing: ["moving partitions", "The broker is handing partitions from one consumer to another. Each old owner gets a few seconds to acknowledge what it holds first."],
  };

  function nodeLabel(ctx, id) {
    return ctx.placed.get(id)?.label || id;
  }

  function ownedText(parts) {
    if (!parts.length) return "no partition: idle";
    const names = parts.map((p) => `P${p}`);
    return `${parts.length === 1 ? "partition" : "partitions"} ${names.length > 1
      ? `${names.slice(0, -1).join(", ")} and ${names.at(-1)}` : names[0]}`;
  }

  /* Apply one /partitions answer. Called about once a second. */
  function paintPartitions(host, data) {
    const svg = host.querySelector(".dg");
    const ctx = svg?.__parts;
    if (!ctx || !data) return;

    const status = svg.querySelector("[data-part-status]");
    if (status) {
      const [text, why] = data.present ? (STATUS[data.status] || [data.status || "", ""])
        : ["not on the broker", "The queue does not exist yet. Press Play, or run Apply configuration."];
      status.textContent = text;
      status.setAttribute("data-state", data.present ? data.status || "" : "absent");
      indicator(status, "partition-status", why || `The broker reports the queue's rebalance state as ${data.status}.`);
    }

    const owned = (data.lanes || []).filter((l) => l.node).map((l) => ({ partition: l.partition, node: l.node }));
    const byLane = new Map((data.lanes || []).map((l) => [l.partition, l]));
    for (let p = 0; p < ctx.box.count; p++) {
      const l = byLane.get(p);
      const lane = svg.querySelector(`[data-lane="${p}"]`);
      const owner = svg.querySelector(`[data-lane-owner="${p}"]`);
      const held = svg.querySelector(`[data-lane-held="${p}"]`);
      const taken = svg.querySelector(`[data-lane-in="${p}"]`);
      lane?.setAttribute("data-owned", l ? String(Boolean(l.owner)) : "absent");
      if (!l) {
        owner.textContent = data.present ? "not on the broker" : "";
        held.textContent = "";
        taken.textContent = "";
        indicator(owner, "lane-owner", data.present
          ? `The broker reports fewer partitions than the ${ctx.box.count} this diagram draws, so P${p} does not exist. Check partition_count in the scenario's terraform.`
          : "The queue is not on the broker yet. Press Play.");
        ctx.paths[p].setAttribute("d", "");
        ctx.owners.delete(p);
        continue;
      }
      const name = l.node ? nodeLabel(ctx, l.node) : l.owner ? l.owner : "no owner";
      owner.textContent = name;
      indicator(owner, "lane-owner", l.owner
        ? `Partition P${p} belongs to ${name} (client ${l.owner}): the broker delivers everything in it to that consumer, and to no other. Read from the broker every second.`
        : `Partition P${p} has no owner right now. Its consumer went away, and the broker has not given it to another yet, so its messages wait.`);
      held.textContent = `${l.held} held`;
      held.setAttribute("data-busy", String(l.held > 0));
      indicator(held, "lane-held", `${l.held} message(s) in partition P${p} right now: waiting, or delivered and not yet acknowledged. It climbs when nothing is reading the partition.`);
      taken.textContent = `${l.spooled} in`;
      indicator(taken, "lane-in", `${l.spooled} message(s) have gone into partition P${p} since the queue was created. Keys are hashed onto partitions, so the counts differ.`);
      const was = ctx.owners.get(p);
      if (!was || was.node !== (l.node || null)) {
        if (was && was.node && l.node) {
          lane?.classList.add("is-moved");
          setTimeout(() => lane?.classList.remove("is-moved"), 2500);
        }
        moveEdge(ctx, p, l.node || null, owned);
      }
      indicator(ctx.paths[p], "lane-edge", l.node
        ? `Partition P${p} is delivered to ${name}. When the broker moves the partition, this line swings to its new owner.`
        : `Partition P${p} has no owner, so nothing is delivered from it.`);
    }
    // Owners that kept their lane still re-spread when a neighbour moved.
    for (const [p, o] of ctx.owners) {
      if (!o.node) continue;
      const end = consumerPort(ctx, o.node, p, owned);
      if (Math.abs(end.y - o.end.y) > 0.5) moveEdge(ctx, p, o.node, owned);
    }

    // Each consumer says which partitions it owns.
    for (const g of svg.querySelectorAll('.dg-node[data-partitioned="true"]')) {
      const id = g.dataset.node;
      const sub = g.querySelector(".dg-node__sub");
      if (!sub) continue;
      if (sub.dataset.text === undefined) sub.dataset.text = sub.textContent;
      const running = g.getAttribute("data-status") === "running";
      const mine = owned.filter((o) => o.node === id).map((o) => o.partition);
      sub.textContent = running && data.present ? ownedText(mine) : sub.dataset.text;
      g.toggleAttribute("data-idle", running && data.present && !mine.length);
      indicator(sub, "consumer-partitions", running && data.present
        ? (mine.length
          ? `The broker gives this consumer ${ownedText(mine)}: every update with a key in them comes here.`
          : "Bound to the queue, but the broker has given it no partition, so it receives nothing. With more consumers than partitions, the extra ones sit idle; it can also be the few seconds before a rebalance.")
        : null);
    }

    // New events: a dot each, spaced as they happened, then a chip.
    const events = data.events || [];
    if (!ctx.primed) {
      // History from before the page opened: chips only, no replay.
      ctx.primed = true;
      const cutoff = (data.until || 0) - 2;
      for (const e of events) {
        if (e.ts <= cutoff) {
          e.letter = letterFor(ctx, e, owned);
          releaseIfDone(ctx, e);
          pushChip(svg, ctx, e);
          ctx.clock = Math.max(ctx.clock, e.ts);
        }
      }
    }
    const fresh = events.filter((e) => e.ts > ctx.clock);
    if (!fresh.length) return;
    const first = fresh[0].ts;
    for (const e of fresh) {
      ctx.clock = Math.max(ctx.clock, e.ts);
      const delay = Math.min(1500, (e.ts - first) * 1000);
      e.letter = letterFor(ctx, e, owned);
      releaseIfDone(ctx, e);
      launchDot(ctx, e, owned, delay, () => pushChip(svg, ctx, e));
    }
  }

  return { render, paint, paintPartitions, selectPath };
})();
