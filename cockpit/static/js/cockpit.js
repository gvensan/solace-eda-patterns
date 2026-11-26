/* Workshop cockpit UI.

   No framework and no build step: the container should boot straight into a
   working cockpit, and an attendee who opens devtools should see the same code
   we wrote. State is a plain object; rendering is direct DOM construction.
*/

(() => {
  const state = {
    env: null,
    scenarios: [],
    activeId: null,
    // What is typed in the sidebar's search box, lower-cased; "" shows all.
    navQuery: "",
    /* One open log socket per action, keyed "<scenario>:<action>". Closed on
       navigation so a long session doesn't accumulate sockets. */
    sockets: new Map(),
    /* The node panel: which node is open, which tab, and its own log socket
       kept apart from the per-action sockets. */
    panel: null,
    panelSocket: null,
    diagramState: null,
    /* Last run status per "<scenario>:<action>", so a flow step opened between
       polls paints its badge and buttons straight away. */
    runStatus: new Map(),
    // The one flow step whose detail is expanded, if any.
    openStep: null,
    /* The failure mode currently in effect, as { scenarioId, modeId }, and
       the log socket its card is streaming. One at a time, so the diagram
       only ever marks one thing as broken. */
    failure: null,
    failureSocket: null,
    /* The failure mode most recently reset, as { scenarioId, modeId }. Its
       card stays open on the restarted app's log, because what happens after
       a reset (a backlog draining, a queued command arriving, a gap that
       never fills) is half of what the failure teaches. */
    recovered: null,
    /* The Break it tile whose card is open, as { scenarioId, modeId }. One
       card shows at a time, under the row of tiles; the card in effect is
       always the open one. Cards tried and closed with Done keep a tick on
       their tile, as "<scenario>:<mode>", for this visit. */
    selectedFailure: null,
    triedFailures: new Set(),
    // The last paint's inputs, so a tile click can repaint between polls.
    failurePaint: { live: false, starting: false },
    // Inspect shades the attendee has opened, as "<scenario>:<label>".
    openInspect: new Set(),
  };

  const el = {
    nav: document.getElementById("nav"),
    view: document.getElementById("view"),
    crumb: document.getElementById("crumb"),
    toasts: document.getElementById("toasts"),
    brokerDot: document.getElementById("broker-dot"),
    brokerText: document.getElementById("broker-text"),
    brokerLink: document.getElementById("broker-link"),
    activeBadge: document.getElementById("active-badge"),
    stopAll: document.getElementById("stop-all"),
    clearBroker: document.getElementById("clear-broker"),
    refresh: document.getElementById("refresh"),
    sidebarToggle: document.getElementById("sidebar-toggle"),
    navSearch: document.getElementById("nav-search"),
    navFold: document.getElementById("nav-fold"),
  };

  /* ------------------------------------------------------------ helpers */

  function h(tag, attrs = {}, ...children) {
    const node = document.createElement(tag);
    for (const [key, value] of Object.entries(attrs)) {
      if (value === null || value === undefined || value === false) continue;
      if (key === "class") node.className = value;
      else if (key === "text") node.textContent = value;
      else if (key === "indicator") indicator(node, ...value);
      else if (key.startsWith("on")) node.addEventListener(key.slice(2).toLowerCase(), value);
      else node.setAttribute(key, value === true ? "" : value);
    }
    for (const child of children.flat()) {
      if (child === null || child === undefined || child === false) continue;
      node.append(child instanceof Node ? child : document.createTextNode(String(child)));
    }
    return node;
  }

  function toast(message, isError = false) {
    const node = h("div", { class: `toast${isError ? " toast--error" : ""}`, text: message });
    el.toasts.append(node);
    setTimeout(() => node.remove(), isError ? 7000 : 4000);
  }

  /* SEMP returns camelCase keys; turn them into readable column headers rather
     than maintaining a label map for every field a scenario might inspect. */
  function humanize(key) {
    return key
      .replace(/([A-Z])/g, " $1")
      .replace(/^./, (c) => c.toUpperCase())
      .trim();
  }

  function formatCell(value, column = "") {
    // SEMP reports times (spooledTime, uptime is a duration and is left
    // alone) as Unix seconds, which read as a meaningless large number.
    if (typeof value === "number" && /Time$/.test(column) && value > 1e9) {
      return { text: new Date(value * 1000).toLocaleTimeString([], { hour12: false }), cls: "is-mono" };
    }
    if (value === true) return { text: "Yes", cls: "is-true" };
    if (value === false) return { text: "No", cls: "is-false" };
    if (value === null || value === undefined || value === "") return { text: "none", cls: "is-false" };
    if (typeof value === "number") return { text: value.toLocaleString(), cls: "is-mono" };
    return { text: String(value), cls: "is-mono" };
  }

  const STATE_LABEL = {
    running: "Running",
    succeeded: "Succeeded",
    failed: "Failed",
    stopped: "Stopped",
    idle: "Idle",
  };

  /* Hover text for anything the page shows as a symbol, a count, a colour
     or a short label. Named, so check_scenarios.py can make sure each one
     has a row in guides/000-reading-the-dashboard.md. An HTML element gets
     a title; an SVG one gets a <title> child, which is what browsers show. */
  function indicator(node, name, text) {
    if (!node) return;
    node.setAttribute("data-indicator", name);
    if (node instanceof SVGElement) {
      let t = node.querySelector(":scope > title");
      if (!text) { t?.remove(); return; }
      if (!t) {
        t = document.createElementNS("http://www.w3.org/2000/svg", "title");
        node.prepend(t);
      }
      t.textContent = text;
    } else if (text) {
      node.title = text;
    } else {
      node.removeAttribute("title");
    }
  }

  /* ------------------------------------------------------- broker status */

  /* Every log on the page keeps only its last LOG_LINES lines. The server
     keeps a bounded history too, but a page left open on a busy card would
     otherwise grow by thousands of lines a minute. Follows the bottom only
     when the reader is already there, so scrolling back to read is not
     yanked away. */
  const LOG_LINES = 500;
  function appendLog(pre, line) {
    const atBottom = pre.scrollHeight - pre.scrollTop - pre.clientHeight < 40;
    pre.append(h("div", { class: `line--${line.stream}`, text: line.text }));
    while (pre.childElementCount > LOG_LINES) pre.firstElementChild.remove();
    if (atBottom) pre.scrollTop = pre.scrollHeight;
  }

  // Polls skip a hidden tab, and catch up the moment it is shown again.
  const pageVisible = () => document.visibilityState === "visible";

  async function pollBroker() {
    if (!pageVisible()) return;
    try {
      const health = await API.health();
      if (health.ready) {
        el.brokerDot.className = "dot dot--ready";
        el.brokerText.textContent = `Ready · ${health.msgVpn}`;
        indicator(el.brokerText.parentElement, "broker-status",
          `The dashboard can reach the broker, and message VPN ${health.msgVpn} is the one it configures`);
      } else {
        /* Almost always the 30-60s container boot, so say that rather than
           showing a raw connection error an attendee cannot act on. */
        el.brokerDot.className = "dot dot--wait";
        el.brokerText.textContent = "Starting…";
        indicator(el.brokerText.parentElement, "broker-status",
          "The broker answers but is not ready yet. A new broker takes about a minute to start");
      }
    } catch (err) {
      el.brokerDot.className = "dot dot--down";
      el.brokerText.textContent = "Unreachable";
      indicator(el.brokerText.parentElement, "broker-status",
        "The dashboard cannot reach the broker's management port. Check the broker container is running");
    }
  }

  async function pollRuns() {
    if (!pageVisible()) return;
    try {
      const { activeCount, sequences, runs, sequenceErrors } = await API.runs(state.activeId);
      el.activeBadge.classList.toggle("hidden", activeCount === 0);
      el.activeBadge.textContent = `${activeCount} running`;
      indicator(el.activeBadge, "running-count",
        `${activeCount} app(s) or step(s) running, across every scenario. Stop everything ends them all`);

      /* Drive the transport from what is actually running: a sequence in
         flight means busy, a live process means Pause is available. */
      if (state.activeId) {
        const busy = (sequences || []).includes(state.activeId);
        const mine = (runs || []).filter(
          (r) => r.key.startsWith(`${state.activeId}:`) && r.state === "running"
        ).length;
        const active = mine > 0;
        /* A halted Play must say why. Without this the buttons simply return
           to resting and it looks like nothing happened. */
        const failure = (sequenceErrors || {})[state.activeId];
        // The diagram's rings and checklists come from the broker, so refresh
        // them on the same tick that updates the transport.
        const current = state.scenarios.find((s) => s.id === state.activeId);
        if (current) refreshDiagramState(current);

        /* The action flow has no log socket of its own, so the run list is
           what keeps its steps honest. */
        for (const r of runs || []) {
          const [sid, aid] = r.key.split(":");
          if (sid === state.activeId) paintStatus(sid, aid, r);
        }

        // A card stays locked while Play is still starting the apps: breaking
        // one half way through stops the sequence and leaves the rest unstarted.
        paintFailureModes(active && !busy, busy);

        setTransport({
          busy,
          active,
          status: busy
            ? "Working through the steps..."
            : failure
            ? failure
            : active
            ? `${mine} process(es) running in this scenario`
            : "",
          failed: Boolean(failure) && !busy,
        });
      }
    } catch (_) {
      /* transient; the next tick will correct it */
    }
  }

  /* ------------------------------------------------------------- console */

  /* Log streams are opened by the node panel now, one at a time. This clears
     any that a future surface might open, and keeps teardown in one place. */
  function closeSockets() {
    for (const socket of state.sockets.values()) socket.close();
    state.sockets.clear();
  }

  /* One place that paints run state, so a flow step's dot, the open step's
     badge and its Run and Stop buttons can never disagree. */
  function paintStatus(scenarioId, actionId, status) {
    const key = `${scenarioId}:${actionId}`;
    state.runStatus.set(key, status);
    const stateName = status?.state || "idle";
    const label = STATE_LABEL[status?.state] || "Not run";

    const badge = document.getElementById(`badge-${key}`);
    if (badge) {
      badge.className = `badge badge--${stateName}`;
      badge.textContent = label;
    }

    const card = document.getElementById(`action-${key}`);
    if (card) {
      card.dataset.state = stateName;
      // The dot's colour is the state; say it in words on hover.
      indicator(card, "step-dot", `${status ? label : "Not run yet"}. Click to see what this step does, run it or stop it`);
    }

    const running = stateName === "running";
    const run = document.getElementById(`run-${key}`);
    const stop = document.getElementById(`stop-${key}`);
    if (run) run.disabled = running;
    if (stop) stop.disabled = !running;
  }

  /* ---------------------------------------------------------- action flow */

  /* Every action drawn as a flowchart: what Play runs, then what Cleanup runs,
     then anything in neither as optional tools. Each step is one line until
     clicked, when a single detail area underneath shows its description and
     its own Run and Stop. */
  function makeActionFlow(scenario) {
    const byId = (id) => scenario.actions.find((a) => a.id === id);
    const play = scenario.runSequence.map(byId).filter(Boolean);
    const cleanup = scenario.cleanupSequence.map(byId).filter(Boolean);
    const sequenced = new Set([...scenario.runSequence, ...scenario.cleanupSequence]);
    // An action that exists only to cause a failure belongs to that failure
    // mode's card, not to the list of optional steps.
    const optional = scenario.actions.filter((a) => !sequenced.has(a.id) && !a.failureOnly);

    state.openStep = null;
    for (const a of scenario.actions) {
      state.runStatus.set(`${scenario.id}:${a.id}`, scenario.runs?.[a.id]);
    }

    let n = 0;
    const step = (action, numbered) =>
      h(
        "button",
        {
          class: `aflow__step aflow__step--${action.variant}`,
          id: `action-${scenario.id}:${action.id}`,
          "data-step": action.id,
          "data-state": scenario.runs?.[action.id]?.state || "idle",
          "aria-expanded": "false",
          "aria-controls": "aflow-detail",
          onClick: () => toggleStep(scenario, action),
        },
        numbered && h("span", { class: "aflow__num", text: String(++n) }),
        h("span", { class: "aflow__dot", "aria-hidden": "true" }),
        h("span", { class: "aflow__label", text: action.label }),
        h("span", { class: "aflow__chev", "aria-hidden": "true", text: "\u25BE" })
      );

    const arrow = (long) =>
      h("span", { class: `aflow__arrow${long ? " aflow__arrow--long" : ""}`, "aria-hidden": "true" });

    /* Steps in a sequence are joined by arrows; optional tools stand alone.
       Each arrow is bound to the step it points at, so a long sequence wraps
       between steps and never leaves an arrow dangling at the end of a line.
       `lead` draws the arrow from Play into the first Cleanup step. */
    const group = (title, actions, { sequence = true, lead = false, cls = "" } = {}) =>
      actions.length
        ? h(
            "div",
            { class: `aflow__group ${cls}`, role: "group", "aria-label": title },
            h("span", { class: "aflow__title", text: title }),
            h(
              "div",
              { class: "aflow__steps" },
              actions.map((a, i) =>
                sequence && (i > 0 || lead)
                  ? h("span", { class: "aflow__hop" }, arrow(i === 0), step(a, true))
                  : step(a, sequence)
              )
            )
          )
        : null;

    return h(
      "div",
      { class: "aflow" },
      h(
        "div",
        { class: "aflow__chart" },
        group("Play", play),
        group("Cleanup", cleanup, { lead: play.length > 0 }),
        group("Optional", optional, { sequence: false, cls: "aflow__group--optional" })
      ),
      h("div", { class: "aflow__detail", id: "aflow-detail", hidden: true })
    );
  }

  function toggleStep(scenario, action) {
    const detail = document.getElementById("aflow-detail");
    if (!detail) return;
    state.openStep = state.openStep === action.id ? null : action.id;
    // Whatever log the previous step was streaming belongs to that step.
    closeSockets();

    for (const node of document.querySelectorAll(".aflow__step")) {
      const open = node.dataset.step === state.openStep;
      node.classList.toggle("is-open", open);
      node.setAttribute("aria-expanded", String(open));
    }

    if (!state.openStep) {
      detail.hidden = true;
      detail.replaceChildren();
      return;
    }

    const key = `${scenario.id}:${action.id}`;
    const status = state.runStatus.get(key);
    detail.replaceChildren(
      h(
        "div",
        { class: "aflow__detail-head" },
        h("span", { class: "action__title", text: action.label }),
        h("span", {
          class: `badge badge--${status?.state || "idle"}`,
          id: `badge-${key}`,
          text: STATE_LABEL[status?.state] || "Not run",
        })
      ),
      action.description && h("p", { class: "action__desc", text: action.description }),
      h(
        "div",
        { class: "action__controls" },
        h("button", {
          class: `btn btn--${action.variant} btn--sm`,
          id: `run-${key}`,
          text: action.label,
          onClick: () => startAction(scenario, action),
        }),
        h("button", {
          class: "btn btn--ghost btn--sm",
          id: `stop-${key}`,
          text: "Stop",
          disabled: true,
          onClick: () => stopAction(scenario, action),
        }),
        h("button", {
          class: "btn btn--ghost btn--sm aflow__logs-btn",
          text: "Show logs",
          "aria-expanded": "false",
          onClick: (e) => toggleStepLog(scenario, action, e.currentTarget),
        })
      ),
      h("pre", { class: "panel__log aflow__log", id: "aflow-log", hidden: true })
    );
    detail.hidden = false;
    paintStatus(scenario.id, action.id, status);
  }

  /* A step's own output, streamed under its detail. Not every action has a
     diagram node to open logs from, and a step that is meant to fail, like a
     refused subscription, is only useful if its output can be read. */
  function toggleStepLog(scenario, action, button) {
    const pre = document.getElementById("aflow-log");
    if (!pre) return;
    const key = `${scenario.id}:${action.id}`;
    const opening = pre.hidden;
    pre.hidden = !opening;
    button.textContent = opening ? "Hide logs" : "Show logs";
    button.setAttribute("aria-expanded", String(opening));

    if (!opening) {
      closeSockets();
      return;
    }

    const append = (line) => appendLog(pre, line);
    state.sockets.set(key, API.streamRun(scenario.id, action.id, {
      onHistory: (lines) => { pre.textContent = ""; lines.forEach(append); },
      onLine: append,
    }));
  }

  async function startAction(scenario, action) {
    if (action.confirm && !window.confirm(action.confirm)) return;
    try {
      await API.startAction(scenario.id, action.id);
      toast(`${action.label} started`);
    } catch (err) {
      toast(err.message, true);
    }
  }

  async function stopAction(scenario, action) {
    try {
      await API.stopAction(scenario.id, action.id);
    } catch (err) {
      toast(err.message, true);
    }
  }

  /* -------------------------------------------------------- failure modes */

  /* Each failure mode is a tile in a row, and a card that opens under the
     row when its tile is picked: what breaks, a button that breaks it, what
     to look for once it has, and a reset that puts the scenario back so the
     next one starts from a working system. One card is open at a time, so
     the row shows every choice at once and the open card has the room.
     Breaking needs the scenario running, so Break it stays locked until
     Play has brought it up; reading a card does not. */
  function makeFailureModes(scenario) {
    const tiles = h(
      "div",
      { class: "fm-tiles", role: "group", "aria-label": "Break it cards" },
      scenario.failureModes.map((mode, i) =>
        h(
          "button",
          {
            class: "fm-tile",
            id: `fmt-${mode.id}`,
            type: "button",
            "data-mode": mode.id,
            "data-state": "locked",
            "aria-controls": `fm-${mode.id}`,
            onClick: () => selectFailure(scenario, mode),
          },
          h("span", { class: "fm__num", text: String(i + 1) }),
          h(
            "span",
            { class: "fm-tile__body" },
            h("span", { class: "fm-tile__title", text: mode.title }),
            h("span", { class: "fm-tile__status" },
              h("span", { class: "badge badge--idle fm-tile__badge", text: "Not ready" }),
              h("span", { class: "fm-tile__tried", hidden: true, text: "✓ tried" }))
          )
        )
      )
    );
    const hint = h("p", {
      class: "fm-hint",
      text: "Pick a card to see what it breaks, then break it, watch, and reset.",
    });
    return h("div", { class: "fm-board" }, tiles, hint, h(
      "div",
      { class: "fm-list" },
      scenario.failureModes.map((mode, i) => {
        const id = `fm-${mode.id}`;
        return h(
          "article",
          { class: "fm", id, "data-mode": mode.id, "data-state": "locked", hidden: true },
          h(
            "div",
            { class: "fm__head" },
            h("span", { class: "fm__num", text: String(i + 1) }),
            h("h3", { class: "fm__title", text: mode.title }),
            h("span", { class: "badge badge--idle fm__badge", text: "Not ready" })
          ),
          h("p", { class: "fm__breaks", text: mode.breaks }),
          h(
            "div",
            { class: "fm__controls" },
            h("button", {
              class: "btn btn--danger btn--sm fm__break",
              text: "Break it",
              disabled: true,
              onClick: () => breakScenario(scenario, mode),
            }),
            h("button", {
              class: "btn btn--secondary btn--sm fm__reset",
              text: "Reset",
              disabled: true,
              onClick: () => resetFailure(scenario, mode),
            }),
            h("button", {
              class: "btn btn--secondary btn--sm fm__done",
              text: "Done",
              hidden: true,
              indicator: ["card-done", "Close this card and go back to the cards. Its tile keeps a tick"],
              onClick: () => doneFailure(scenario, mode),
            })
          ),
          (mode.why || mode.real_world) &&
            h(
              "details",
              { class: "shade fm__why" },
              h(
                "summary",
                { class: "shade__summary fm__why-summary" },
                h("span", { class: "shade__chev", "aria-hidden": "true" }),
                h("span", { text: "Why this breaks, and where you would see it" })
              ),
              h(
                "div",
                { class: "shade__body fm__why-body" },
                mode.why && h("p", { class: "fm__why-title", text: "Why it breaks" }),
                mode.why && h("p", { class: "fm__why-text", text: mode.why }),
                mode.real_world && h("p", { class: "fm__why-title", text: "In the real world" }),
                mode.real_world && h("p", { class: "fm__why-text", text: mode.real_world })
              )
            ),
          h(
            "div",
            { class: "fm__watch", hidden: true },
            /* The app's own output comes first: it is the evidence, and the
               explanation below it makes more sense once it has been read.
               Errors are the red lines. */
            mode.logs &&
              h(
                "div",
                { class: "fm__output" },
                h(
                  "p",
                  { class: "fm__watch-title" },
                  h("span", { class: "fm__output-label", text: "What the app printed" }),
                  h("span", {
                    class: "fm__output-source",
                    text: ` · ${typeof mode.logs === "object"
                      ? roleWords(mode.logs)
                      : scenario.actions.find((a) => a.id === mode.logs)?.label || mode.logs}`,
                  })
                ),
                h("pre", { class: "panel__log fm__log" }),
                h("p", { class: "fm__output-hint", text: "Errors from the broker show in red. The same output is under this node's logs button in the diagram while the failure is in effect." })
              ),
            h("p", { class: "fm__watch-title", text: "What to look for" }),
            watchStory(mode.watch)
          ),
          /* Where to read more: the Go API's reference for this failure, and
             the broker documentation for the behaviour behind it. Always
             shown, so the card is useful before and after breaking it. */
          (mode.docs || []).length &&
            h(
              "div",
              { class: "fm__docs" },
              h("span", { class: "fm__docs-title", text: "Docs" }),
              mode.docs.map((d) =>
                h("a", { class: "fm__doc", href: d.url, target: "_blank", rel: "noopener", text: d.label })
              )
            )
        );
      })
    ));
  }

  /* `watch:` is plain text, or the four parts of the story in order: what
     the app does normally, what broke, the impact, and what Reset does. The
     parts are labelled the same on every card, so they scan. */
  const WATCH_PARTS = [
    ["normally", "Normally"],
    ["broke", "What broke"],
    ["impact", "Impact"],
    ["recover", "To recover"],
  ];

  function watchStory(watch) {
    if (!watch || typeof watch !== "object") {
      return h("p", { class: "fm__watch-text", text: watch || "" });
    }
    return h(
      "dl",
      { class: "fm__story" },
      WATCH_PARTS.filter(([key]) => watch[key]).map(([key, label]) =>
        h("div", { class: `fm__part fm__part--${key}` },
          h("dt", { class: "fm__part-label", text: label }),
          h("dd", { class: "fm__part-text", text: watch[key] }))
      )
    );
  }

  /* Open a card under the row. While a card is in effect no other can open:
     its tile is the one marked, and Reset or the bar at the top lead back to
     it. A recovered card that is left for another counts as done. */
  function selectFailure(scenario, mode, scroll = true) {
    if (state.failure) return;
    const rec = state.recovered;
    if (rec && rec.scenarioId === scenario.id && rec.modeId !== mode.id) {
      state.triedFailures.add(`${scenario.id}:${rec.modeId}`);
      state.recovered = null;
      closeFailureSocket();
    }
    state.selectedFailure = { scenarioId: scenario.id, modeId: mode.id };
    paintFailureModes(state.failurePaint.live, state.failurePaint.starting);
    if (scroll) {
      document.getElementById(`fm-${mode.id}`)?.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  }

  // Done: close the card, stop its log, and give its tile a tick.
  function doneFailure(scenario, mode) {
    state.triedFailures.add(`${scenario.id}:${mode.id}`);
    if (state.recovered?.scenarioId === scenario.id && state.recovered?.modeId === mode.id) {
      state.recovered = null;
    }
    closeFailureSocket();
    state.selectedFailure = null;
    paintFailureModes(state.failurePaint.live, state.failurePaint.starting);
    document.getElementById(`fmt-${mode.id}`)?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }

  /* A failure mode's `node` and `logs` can name a role rather than a fixed
     diagram node: {role: "active"} or {role: "standby"}, optionally with a
     `queue:`. That is the consumer the broker has made active on an exclusive
     queue, or the one waiting, among the nodes with `flowOn`. Which app holds
     a role changes as apps stop and start, so it is resolved from the live
     diagram state when Break it is pressed, and kept with the failure so its
     border and log stay on the same app through Reset. */
  /* Roles: "active" and "standby" are what the broker reports for a
     consumer's flow; "other" is an active consumer that is not the one the
     card's `node:` names, which is how a card on a shared queue says "the
     worker that was not hit". Several nodes can hold a role on a shared
     queue, so the one with the lowest client name is taken, the same rule
     signal_consumer.sh uses, so the page marks the app the trigger hits. */
  function roleNode(scenario, ref, exclude = null) {
    if (!ref || typeof ref === "string") return ref || null;
    const want = ref.role === "standby" ? ["standby"] : ["active", "stalled"];
    const skip = ref.role === "other" ? exclude : null;
    return (scenario.diagram?.nodes || [])
      .filter((n) => n.flowOn && (!ref.queue || n.flowOn.queue === ref.queue)
        && n.id !== skip && want.includes(state.diagramState?.nodes?.[n.id]?.flow))
      .sort((a, b) => a.flowOn.client.localeCompare(b.flowOn.client))[0]?.id || null;
  }

  function bindMode(scenario, mode) {
    const node = roleNode(scenario, mode.node);
    if (mode.logs && typeof mode.logs === "object") {
      const id = roleNode(scenario, mode.logs, node);
      const logs = (scenario.diagram?.nodes || []).find((n) => n.id === id)?.action || null;
      return { node, logs };
    }
    return { node, logs: mode.logs || null };
  }

  // The node and log a failure is using: as bound at Break it, or as written.
  const boundOf = (scenario, mode) =>
    (state.failure?.scenarioId === scenario.id && state.failure?.modeId === mode.id && state.failure.bound)
    || (state.recovered?.scenarioId === scenario.id && state.recovered?.modeId === mode.id && state.recovered.bound)
    || { node: typeof mode.node === "string" ? mode.node : null,
         logs: typeof mode.logs === "string" ? mode.logs : null };

  const roleWords = (ref) => `the ${ref.role} app`;

  const failureActive = (scenario, mode) =>
    state.failure?.scenarioId === scenario.id && state.failure?.modeId === mode.id;

  /* Keep every card honest about whether it can be used. Called on each run
     poll, since "the scenario is running" is what unlocks them. */
  function paintFailureModes(live, starting = false) {
    state.failurePaint = { live, starting };
    const scenario = state.scenarios.find((s) => s.id === state.activeId);
    if (!scenario) return;
    /* The open card: the one in effect, else the one just reset (it stays
       open on the recovery until Done), else the one picked. */
    const mine = (ref) => (ref?.scenarioId === scenario.id ? ref.modeId : null);
    const openId = mine(state.failure) || mine(state.recovered) || mine(state.selectedFailure);
    const hint = document.querySelector(".fm-hint");
    if (hint) hint.hidden = Boolean(openId);
    /* Most failures need apps running to break. A scenario with no apps, like
       the broker tour, marks its modes `needs: applied`: they unlock once its
       configuration has been applied. */
    const applied = live || state.runStatus.get(`${scenario.id}:apply`)?.state === "succeeded";
    for (const mode of scenario.failureModes || []) {
      const card = document.getElementById(`fm-${mode.id}`);
      if (!card) continue;
      const ready = mode.needs === "applied" ? applied : live;
      const active = failureActive(scenario, mode);
      const other = Boolean(state.failure) && !active;
      const recovered =
        state.recovered?.scenarioId === scenario.id && state.recovered?.modeId === mode.id;
      const cardState = active
        ? "broken"
        : !ready
        ? "locked"
        : other
        ? "waiting"
        : recovered
        ? "recovered"
        : "ready";
      card.dataset.state = cardState;

      const badge = card.querySelector(".fm__badge");
      const [cls, text] = {
        broken: ["failed", "Broken"],
        locked: ["idle", starting ? "Starting" : "Not ready"],
        waiting: ["idle", "Reset the other first"],
        recovered: ["succeeded", "Recovered"],
        ready: ["succeeded", "Ready"],
      }[cardState];
      badge.className = `badge badge--${cls} fm__badge`;
      badge.textContent = text;
      indicator(badge, "card-badge", {
        broken: "This failure is in effect now. Press Reset to put things back",
        locked: starting
          ? "Play is still starting the apps. Wait for it to finish, then break it"
          : mode.needs === "applied"
          ? "Apply this scenario's configuration first (press Play)"
          : "Press Play first. This failure needs the scenario running",
        waiting: "Another failure on this page is in effect. Reset that one first",
        recovered: "Reset is done. The log shows the recovery. You can break it again",
        ready: "Press Break it to cause this failure on purpose",
      }[cardState]);

      // A recovered card can be broken again straight away: try again.
      card.querySelector(".fm__break").disabled = cardState !== "ready" && cardState !== "recovered";
      card.querySelector(".fm__reset").disabled = !active;
      card.querySelector(".fm__watch").hidden = !active && cardState !== "recovered";
      card.querySelector(".fm__done").hidden = cardState !== "recovered";
      const label = card.querySelector(".fm__output-label");
      if (label) label.textContent = cardState === "recovered" ? "After reset" : "What the app printed";

      // Only the open card shows; its tile is marked and cannot be pressed.
      const open = mode.id === openId;
      card.hidden = !open;
      const tile = document.getElementById(`fmt-${mode.id}`);
      if (!tile) continue;
      const tried = state.triedFailures.has(`${scenario.id}:${mode.id}`);
      tile.dataset.state = cardState;
      tile.classList.toggle("is-open", open);
      tile.classList.toggle("is-tried", tried);
      tile.setAttribute("aria-expanded", String(open));
      tile.disabled = open || other;
      const tileBadge = tile.querySelector(".fm-tile__badge");
      tileBadge.className = `badge badge--${cls} fm-tile__badge`;
      tileBadge.textContent = cardState === "waiting" ? "Locked" : text;
      tile.querySelector(".fm-tile__tried").hidden = !tried;
      indicator(tile, "card-tile", open
        ? "This card is open below"
        : other
        ? "Another card is in effect. Reset it first"
        : "Open this card to see what it breaks");
      indicator(tile.querySelector(".fm-tile__tried"), "card-tried",
        "You have broken and reset this card on this visit. You can open it and try again");
    }
  }

  /* Steps in order, as a failure mode's trigger or reset lists them. `start`
     and `stop` return at once; `run` starts a step and waits for it to finish,
     for a step the next one depends on, like deleting terraform's state
     before an apply that should then fail. */
  async function runSteps(scenario, steps) {
    for (const step of steps || []) {
      const [verb, actionId] = Object.entries(step)[0];
      if (verb === "stop") {
        await API.stopAction(scenario.id, actionId);
        continue;
      }
      // A step already running is left alone, so a failure mode can make
      // sure something is up (like the processor's router) without failing
      // when it already is.
      const { runs } = await API.runs(scenario.id);
      const current = (runs || []).find((r) => r.key === `${scenario.id}:${actionId}`);
      if (current?.state !== "running") await API.startAction(scenario.id, actionId);
      if (verb === "run") await waitForStep(scenario.id, actionId);
    }
  }

  async function waitForStep(scenarioId, actionId, timeoutMs = 180000) {
    const key = `${scenarioId}:${actionId}`;
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 1000));
      const { runs } = await API.runs(scenarioId);
      const run = (runs || []).find((r) => r.key === key);
      if (run && run.state !== "running") return run;
    }
    throw new Error(`Timed out waiting for ${actionId} to finish`);
  }

  function markDiagramNode(nodeId, broken) {
    for (const n of document.querySelectorAll(".dg-node.is-faulted")) {
      n.classList.remove("is-faulted");
      indicator(n.querySelector(":scope > .dg-node__box"), "faulted-border", null);
    }
    if (broken && nodeId) {
      const node = document.querySelector(`.dg-node[data-node="${nodeId}"]`);
      node?.classList.add("is-faulted");
      // The red border needs saying in words: it is the failure, not an error.
      indicator(node?.querySelector(":scope > .dg-node__box"), "faulted-border",
        "A Break it card is in effect on this app. See the card below");
    }
  }

  function closeFailureSocket() {
    if (state.failureSocket) {
      state.failureSocket.close();
      state.failureSocket = null;
    }
  }

  // Stream the step whose output tells this failure's story into its card.
  function streamFailureLog(scenario, mode) {
    closeFailureSocket();
    const pre = document.querySelector(`#fm-${mode.id} .fm__log`);
    const logs = boundOf(scenario, mode).logs;
    if (!pre || !logs) return;
    // A role names whichever app held it at Break it; say which one that was.
    const source = document.querySelector(`#fm-${mode.id} .fm__output-source`);
    if (source && typeof mode.logs === "object") {
      source.textContent = ` · ${scenario.actions.find((a) => a.id === logs)?.label || logs}`;
    }
    pre.textContent = "";
    const append = (line) => appendLog(pre, line);
    state.failureSocket = API.streamRun(scenario.id, logs, {
      onHistory: (lines) => { pre.textContent = ""; lines.forEach(append); },
      onLine: append,
    });
  }

  async function breakScenario(scenario, mode) {
    const roles = [mode.node, mode.logs].filter((r) => r && typeof r === "object");
    // Roles are read from the broker, so ask it now rather than trust the
    // last poll: the attendee may have stopped an app a second ago.
    if (roles.length) await refreshDiagramState(scenario);
    let bound = bindMode(scenario, mode);
    const unbound = (b) => (roles.length && !b.node && mode.node) || (mode.logs && !b.logs);
    /* Just after another card's Reset, consumers can be between roles for a
       few seconds (partitions moving back, a standby taking over), so ask
       the broker again for a while before giving up. */
    for (let tries = 0; unbound(bound) && roles.length && tries < 8; tries++) {
      await new Promise((r) => setTimeout(r, 1000));
      await refreshDiagramState(scenario);
      bound = bindMode(scenario, mode);
    }
    if (unbound(bound)) {
      toast(`The broker has no ${roles.map((r) => r.role).join(" or ")} consumer right now. ` +
        "Make sure every app is running (press Play), wait a moment, and try again.", true);
      return;
    }
    state.recovered = null;
    state.selectedFailure = { scenarioId: scenario.id, modeId: mode.id };
    state.failure = { scenarioId: scenario.id, modeId: mode.id, bound, since: Date.now() };
    paintFailureModes(true);
    markDiagramNode(bound.node, true);
    paintFailBar();
    try {
      await runSteps(scenario, mode.trigger);
      streamFailureLog(scenario, mode);
      /* The failure shows on the diagram, a screen or more above the card,
         so go there, with the broken app's log open beside it. The bar at
         the top leads back to the card and its Reset. */
      if (bound.node) selectNode(scenario, bound.node, "logs");
      document.querySelector(".stage")?.scrollIntoView({ behavior: "smooth", block: "start" });
    } catch (err) {
      toast(err.message, true);
    }
  }

  /* Play, Pause, Cleanup and the rest restart or stop the apps a failure
     mode broke, so a card left saying "Broken" afterwards would be wrong, and
     its Reset would start apps against whatever is left. With no scenario,
     any failure is cleared. */
  function forgetFailure(scenarioId) {
    if (scenarioId && state.failure?.scenarioId !== scenarioId
        && state.recovered?.scenarioId !== scenarioId) return;
    state.failure = null;
    state.recovered = null;
    closeFailureSocket();
    markDiagramNode(null, false);
    paintFailBar();
    // The next run poll repaints the cards with the real running state.
  }

  async function resetFailure(scenario, mode) {
    const bound = boundOf(scenario, mode);
    try {
      await runSteps(scenario, mode.reset);
      toast(`${mode.title}: reset`);
    } catch (err) {
      toast(err.message, true);
    }
    state.failure = null;
    state.recovered = { scenarioId: scenario.id, modeId: mode.id, bound };
    state.triedFailures.add(`${scenario.id}:${mode.id}`);
    markDiagramNode(null, false);
    paintFailureModes(true);
    paintFailBar();
    // Reattach to the log, which now belongs to the app's new run.
    streamFailureLog(scenario, mode);
    if (state.panel?.nodeId === bound.node) renderPanel();
  }

  /* -------------------------------------------------------------- inspect */

  /* Each view is a shade, closed until opened, so the part reads as a list
     of what is on the broker rather than a wall of tables. The summary still
     says how many objects there are, so a closed shade is not empty of
     information. Which shades are open is remembered across refreshes. */
  function makeInspectView(scenarioId, view) {
    const key = `${scenarioId}:${view.label}`;
    const count = view.error
      ? "unavailable"
      : view.rows.length
      ? `${view.rows.length} on the broker`
      : "none yet";

    let body;
    if (view.error) {
      body = h("div", { class: "empty", text: view.error });
    } else if (!view.rows.length) {
      body = h("div", { class: "empty",
        text: view.empty || "Nothing on the broker matches this view right now." });
    } else {
      /* A scenario may declare which columns matter; fall back to whatever
         keys the first row happens to have so a new inspect view still renders. */
      const columns = view.columns.length ? view.columns : Object.keys(view.rows[0]);
      body = h(
        "div",
        { class: "table-wrap" },
        h(
          "table",
          {},
          h("thead", {}, h("tr", {}, columns.map((c) => h("th", { text: humanize(c) })))),
          h(
            "tbody",
            {},
            view.rows.map((row) =>
              h(
                "tr",
                {},
                columns.map((c) => {
                  const { text, cls } = formatCell(row[c], c);
                  return h("td", { class: cls, text });
                })
              )
            )
          )
        )
      );
    }

    const shade = h(
      "details",
      { class: "shade", open: state.openInspect.has(key) },
      h(
        "summary",
        { class: "inspect__head shade__summary" },
        h("span", { class: "shade__chev", "aria-hidden": "true" }),
        h("h4", { text: view.label }),
        h("span", { class: "inspect__hint", text: count }),
        view.uiHint && h("span", { class: "inspect__hint inspect__where", text: view.uiHint })
      ),
      h("div", { class: "shade__body" }, body)
    );
    shade.addEventListener("toggle", () => {
      if (shade.open) state.openInspect.add(key);
      else state.openInspect.delete(key);
    });
    return shade;
  }

  /* On the broker is read again every few seconds while the page is open
     (see init), so it never shows a moment that has passed, such as the
     instant during Play when the queue exists but nothing has bound to it.
     The tables are rebuilt only when what the broker returns has changed,
     so an open shade does not flicker or lose its scroll position. */
  async function refreshInspect(scenarioId) {
    const host = document.getElementById("inspect-host");
    if (!host || state.inspectBusy) return;
    state.inspectBusy = true;
    try {
      const { views } = await API.inspect(scenarioId);
      if (document.getElementById("inspect-host") !== host) return;
      const seen = JSON.stringify(views);
      if (seen !== host.dataset.seen) {
        host.dataset.seen = seen;
        host.replaceChildren(...views.map((v) => makeInspectView(scenarioId, v)));
      }
      const asof = document.getElementById("inspect-asof");
      if (asof) {
        asof.textContent = `Updated ${new Date().toLocaleTimeString([], { hour12: false })}`;
        indicator(asof, "inspect-updated", "Read from the broker every 5 seconds while this page is open. Refresh reads it now.");
      }
    } catch (err) {
      host.dataset.seen = "";
      host.replaceChildren(h("div", { class: "empty", text: err.message }));
    } finally {
      state.inspectBusy = false;
    }
  }

  /* --------------------------------------------------------------- render */

  /* The sidebar groups scenarios under their eyebrow, which is the pattern
     guide's category. Scenarios arrive sorted by order, and order is
     category times 100 plus position, so a category's scenarios are already
     together: a heading goes in wherever the eyebrow changes. */
  /* The search box above the list narrows it as you type, on the pattern's
     name or its category. A category with no match disappears with its
     patterns, so the list reads as a shorter version of itself. Numbers
     keep their place in the full list. Collapsed, the box is hidden, so it
     filters nothing until the sidebar opens again. */
  /* Temporary: only the first four categories are released. A later
     category shows its heading in the sidebar with "coming soon" and none
     of its patterns, and no released page links to one. A held pattern still
     opens from its own address (#<id>), for whoever is working on it. To
     release more, raise this to the next category's first order (600, 700,
     ...), or to Infinity for all of them. */
  const RELEASED_BELOW = 500;
  const held = (scenario) => scenario.order >= RELEASED_BELOW;

  function navMatches(scenario) {
    const q = state.navQuery;
    if (!q || state.sidebarCollapsed) return true;
    // A held pattern is not listed, so its name finds nothing; its category does.
    return (!held(scenario) && scenario.title.toLowerCase().includes(q)) || (scenario.eyebrow || "").toLowerCase().includes(q);
  }

  /* A category folds away under its heading: click the heading, or use the
     button beside the search box for all of them at once. What is folded is
     remembered. Two things show a folded category's patterns anyway: a
     search, which must be able to find them, and the sidebar shrunk to
     numbers, which has no headings to click. */
  const foldKey = "eda.foldedGroups";
  const folded = new Set();
  try { for (const g of JSON.parse(localStorage.getItem(foldKey) || "[]")) folded.add(g); } catch (_) { /* private window */ }
  const saveFolds = () => { try { localStorage.setItem(foldKey, JSON.stringify([...folded])); } catch (_) { /* private window */ } };
  const releasedGroups = () => [...new Set(state.scenarios.filter((s) => !held(s)).map((s) => s.eyebrow))];
  const foldsApply = () => !state.navQuery && !state.sidebarCollapsed;

  function toggleGroup(name) {
    if (folded.has(name)) folded.delete(name); else folded.add(name);
    saveFolds();
    renderNav();
  }

  function paintFoldAll() {
    const groups = releasedGroups();
    const allFolded = groups.length > 0 && groups.every((g) => folded.has(g));
    el.navFold.textContent = allFolded ? "\u25be" : "\u25b4";
    el.navFold.setAttribute("aria-label", allFolded ? "Expand all categories" : "Collapse all categories");
    indicator(el.navFold, "nav-fold-all", allFolded
      ? "Expand every category, to list all the patterns"
      : "Collapse every category to its heading. Click a heading to open one");
  }

  function renderNav() {
    let group = null;
    const shown = state.scenarios.filter(navMatches);
    paintFoldAll();
    el.nav.replaceChildren(
      ...state.scenarios.flatMap((scenario, index) => {
        if (!shown.includes(scenario)) return [];
        const name = scenario.eyebrow;
        const isFolded = folded.has(name);
        const heading = name && name !== group
          ? held(scenario)
            ? h("div", { class: "nav-group", role: "heading", "aria-level": "2" },
                h("span", { text: name }),
                h("span", { class: "nav-group__soon", text: "coming soon",
                  indicator: ["nav-coming-soon", `The ${name} are not released yet. They will appear here when they are`] }))
            : h("button", { class: "nav-group nav-group--fold", type: "button", "aria-expanded": String(!isFolded),
                indicator: ["nav-group-fold", isFolded
                  ? `Show the ${name}`
                  : `Hide the ${name}, leaving the heading`],
                onClick: () => toggleGroup(name) },
                h("span", { class: "nav-group__chevron", "aria-hidden": "true", text: isFolded ? "\u25b8" : "\u25be" }),
                h("span", { text: name }))
          : null;
        group = name;
        const hidden = held(scenario) || (isFolded && foldsApply());
        return [heading, !hidden && navItem(scenario, index)].filter(Boolean);
      }),
      // Spread, so a list with matches adds nothing: replaceChildren writes
      // a bare null out as the word "null".
      ...(shown.length ? [] : [h("div", { class: "nav-empty", text: "No pattern matches" })])
    );
  }

  function navItem(scenario, index) {
    const item = navButton(scenario, index);
    // Collapsed to numbers only, the name is a hover away.
    if (state.sidebarCollapsed) indicator(item, "nav-collapsed", `${scenario.title}, ${scenario.eyebrow}`);
    return item;
  }

  function navButton(scenario, index) {
    return (
        h(
          "button",
          {
            class: "nav-item",
            "aria-current": String(scenario.id === state.activeId),
            onClick: () => select(scenario.id),
          },
          h("span", { class: "nav-item__index", text: String(index + 1).padStart(2, "0") }),
          h("span", { class: "nav-item__title", text: scenario.title })
        )
    );
  }

  function renderScenario(scenario) {
    el.crumb.textContent = scenario.title;

    /* Two columns: this scenario's story and what it lets you see on the
       left, the pattern itself on the right. The scenario is one example of
       the pattern; keeping them apart lets an attendee tell the idea from
       the shop or bank that illustrates it. */
    const header = h(
      "section",
      { class: `scenario-header${scenario.pattern?.what ? " scenario-header--split" : ""}` },
      h("div", { class: "scenario-header__story" },
        h("p", { class: "eyebrow", text: scenario.eyebrow }),
        h("h1", { text: scenario.title }),
        h("p", { class: "scenario-header__label", text: "This scenario" }),
        h("p", { class: "scenario-header__summary", text: scenario.summary }),
        scenario.objectives.length &&
          h("ul", { class: "objectives" }, scenario.objectives.map((o) => h("li", { text: o })))),
      scenario.pattern?.what && makePatternPanel(scenario)
    );

    /* The diagram is the primary interface: nodes carry live status, open their
       own logs, and can be clicked to start or stop what they represent. */
    const diagramHost = h("div", { class: "stage__diagram", id: "diagram-host" });
    const panelHost = h("aside", { class: "stage__panel", id: "panel-host" });
    const stage = h("section", { class: "stage" }, diagramHost, panelHost);

    let pathList = null;
    if (scenario.diagram && (scenario.diagram.nodes || []).length) {
      Diagram.render(diagramHost, scenario.diagram, {
        onSelect: (id) => selectNode(scenario, id),
        onLogs: (id) => selectNode(scenario, id, "logs"),
        onPath: (k) => markPath(pathList, k),
      });
      // Under the diagram and its panel, full width, so opening a node's
      // panel narrows the diagram but never reflows the list.
      pathList = makePathList(scenario, diagramHost);
      refreshDiagramState(scenario);
      watchPartitions(scenario, diagramHost);
    }

    const holds = makeHolds(scenario);

    /* The diagram, its side panel and what each app holds, as one canvas
       that can fill the window. It stays below the Broken bar, so Reset is
       always in reach. */
    const expand = h("button", { class: "btn btn--ghost btn--sm canvas__expand", type: "button",
      onClick: () => setExpanded(!document.getElementById("canvas")?.classList.contains("is-expanded")) });
    const canvas = h("div", { class: "canvas", id: "canvas" }, expand, stage, pathList, holds);
    document.body.classList.remove("canvas-expanded");
    paintExpand(expand, false);
    const stats = h("section", { class: "stats", id: "stats" });

    /* Transport controls: the primary way to drive a scenario. The action flow
       below is for anyone who wants one step on its own. */
    const transport = makeTransport(scenario);

    /* The page reads top to bottom in the order a scenario is worked through:
       run it, step through it, break it, then look at what is on the broker.
       Numbering the parts makes that order visible rather than implied. */
    const parts = [];
    const part = (key, title, hint, extra, ...content) => {
      const n = String(parts.length + 1).padStart(2, "0");
      parts.push(
        h(
          "section",
          { class: "part", id: `part-${key}` },
          h(
            "div",
            { class: "part__head" },
            h("span", { class: "part__num", text: n }),
            h("div", { class: "part__titles" },
              h("h2", { class: "part__title", text: title }),
              hint && h("p", { class: "part__hint", text: hint })),
            extra
          ),
          ...content
        )
      );
    };

    part(
      "run",
      "Run it",
      "Press Play to bring the scenario up. The diagram shows what is running and where events go.",
      null,
      transport, canvas, stats
    );

    part(
      "steps",
      "Step through it",
      "Every step Play and Cleanup run, one at a time. Click a step to run it on its own or read its logs.",
      h("button", {
        class: "btn btn--ghost btn--sm",
        text: "Reset this scenario",
        onClick: () => resetScenario(scenario),
      }),
      makeActionFlow(scenario)
    );

    if ((scenario.failureModes || []).length) {
      part(
        "break",
        "Break it",
        "With the scenario running, cause one failure on purpose, watch what the broker does, then reset and try another.",
        null,
        makeFailureModes(scenario)
      );
    }

    part(
      "broker",
      "On the broker",
      "What the broker holds right now, read over SEMP.",
      h(
        "div",
        { class: "inspect-tools" },
        h("span", { id: "inspect-asof", class: "inspect-asof" }),
        h("button", {
          class: "btn btn--secondary btn--sm",
          text: "Refresh",
          indicator: ["refresh-button", "Read the broker again now"],
          onClick: () => refreshInspect(scenario.id),
        })
      ),
      h("div", { id: "inspect-host", class: "shade-list" }, h("div", { class: "empty", text: "Loading…" }))
    );

    const failBar = h("div", { class: "failbar", id: "failbar", hidden: true });
    el.view.replaceChildren(failBar, header, ...parts);

    /* A failure left in effect is still in effect on the broker, so coming
       back to the scenario shows it broken again rather than a fresh card. */
    const broken = (scenario.failureModes || []).find((m) => failureActive(scenario, m));
    if (broken) {
      markDiagramNode(boundOf(scenario, broken).node, true);
      streamFailureLog(scenario, broken);
    }
    paintFailBar();

    refreshInspect(scenario.id);
    refreshStats();
  }

  function makePatternPanel(scenario) {
    const p = scenario.pattern;
    const row = (title, text) => text && h("div", { class: "pattern__row" },
      h("p", { class: "pattern__title", text: title }),
      h("p", { class: "pattern__text", text: text }));
    const related = (p.related || [])
      .map((id) => state.scenarios.find((s) => s.id === id))
      .filter(Boolean);
    return h("aside", { class: "pattern", "aria-label": "The pattern" },
      h("p", { class: "pattern__eyebrow", text: "The pattern" }),
      h("p", { class: "pattern__what", text: p.what }),
      row("Use it when", p.when),
      row("Watch out for", p.cost),
      row("On Solace", p.solace),
      related.length && h("div", { class: "pattern__row" },
        h("p", { class: "pattern__title", text: "Next to it" }),
        h("div", { class: "pattern__related" },
          related.map((r) => held(r)
            ? h("span", { class: "pattern__link pattern__link--soon", text: `${r.title} (coming soon)`,
                indicator: ["related-coming-soon", `${r.title} is not released yet, so there is nothing to open`] })
            : h("a", { class: "pattern__link", href: `#${r.id}`, text: r.title })))),
      p.guide && h("a", { class: "pattern__guide", href: p.guide, target: "_blank", rel: "noopener",
        text: "Read about it in Solace's Event-Driven Architecture Patterns" }));
  }

  /* The paths a message takes (`paths:`), listed under the diagram with
     the same numbers as on its lines. Clicking a path, or a line on it,
     highlights the whole route; the list marks the one shown. */
  function makePathList(scenario, diagramHost) {
    const paths = scenario.diagram?.paths || [];
    if (!paths.length) return null;
    let n = 0;
    return h("section", { class: "paths", "aria-label": "Follow a message" },
      h("p", { class: "paths__head", text: "Follow a message" }),
      h("p", { class: "paths__hint", text: "Click a path, or any numbered line on the diagram, to highlight its whole route. Esc clears it." }),
      h("div", { class: "paths__list" },
        paths.map((path, k) => {
          const start = n + 1;
          n += (path.steps || []).length;
          return h("div", { class: "paths__path", "data-path": k },
            h("button", {
              class: "paths__title", type: "button", "aria-pressed": "false", text: path.label,
              indicator: ["path-pick", `Highlight this path on the diagram: steps ${start} to ${n}. Click again, or press Esc, to clear`],
              onClick: () => {
                const host = diagramHost;
                const on = host.querySelector(".dg")?.__paths?.current === k;
                Diagram.selectPath(host, on ? null : k);
              },
            }),
            h("ol", { class: "paths__steps", start: String(start) },
              (path.steps || []).map((st) => h("li", { text: st.text }))));
        })));
  }

  function markPath(list, k) {
    for (const p of list?.querySelectorAll(".paths__path") || []) {
      const on = Number(p.dataset.path) === k;
      p.classList.toggle("is-active", on);
      p.querySelector(".paths__title")?.setAttribute("aria-pressed", String(on));
    }
  }

  function paintExpand(btn, on) {
    btn.textContent = on ? "Collapse" : "Expand";
    btn.setAttribute("aria-pressed", String(on));
    indicator(btn, "canvas-expand", on
      ? "Back to the page. Esc does the same"
      : "Fill the window with the diagram, its side panel and what each app holds. Esc or Collapse brings it back");
  }

  function setExpanded(on) {
    const canvas = document.getElementById("canvas");
    if (!canvas) return;
    canvas.classList.toggle("is-expanded", on);
    document.body.classList.toggle("canvas-expanded", on);
    const btn = canvas.querySelector(".canvas__expand");
    if (btn) paintExpand(btn, on);
    if (!on) canvas.scrollIntoView({ block: "start" });
  }

  /* While a failure is in effect a bar stays at the top of the page: which
     card, for how long, a way back to the diagram or the card, and Reset, so
     the attendee can watch the diagram without losing the card. */
  function paintFailBar() {
    const bar = document.getElementById("failbar");
    if (!bar) return;
    const scenario = state.scenarios.find((s) => s.id === state.activeId);
    const mode = scenario && (scenario.failureModes || []).find((m) => failureActive(scenario, m));
    if (!mode) {
      bar.hidden = true;
      bar.replaceChildren();
      measureFailBar();
      return;
    }
    const jump = (selector) => document.querySelector(selector)?.scrollIntoView({ behavior: "smooth", block: "start" });
    bar.hidden = false;
    bar.replaceChildren(
      h("span", { class: "badge badge--failed", text: "Broken",
        indicator: ["failure-bar", `"${mode.title}" is in effect. Watch the diagram and the log, then press Reset to put things back`] }),
      h("span", { class: "failbar__title", text: mode.title }),
      h("span", { class: "failbar__time", id: "failbar-time",
        indicator: ["failure-timer", "How long this failure has been in effect. Backlogs and timeouts grow with it"] }),
      h("span", { class: "failbar__spacer" }),
      h("button", { class: "btn btn--ghost btn--sm", text: "Diagram", onClick: () => jump(".stage") }),
      h("button", { class: "btn btn--ghost btn--sm", text: "Card", onClick: () => jump(`#fm-${mode.id}`) }),
      h("button", { class: "btn btn--secondary btn--sm", text: "Reset", onClick: () => resetFailure(scenario, mode) })
    );
    tickFailBar();
    measureFailBar();
  }

  // The expanded canvas sits below the Broken bar, however tall it wraps.
  function measureFailBar() {
    const bar = document.getElementById("failbar");
    const px = bar && !bar.hidden ? bar.offsetHeight : 0;
    document.documentElement.style.setProperty("--failbar-h", `${px}px`);
  }

  function tickFailBar() {
    const t = document.getElementById("failbar-time");
    if (!t || !state.failure?.since) return;
    const secs = Math.floor((Date.now() - state.failure.since) / 1000);
    t.textContent = `${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, "0")}`;
  }

  async function refreshStats() {
    const host = document.getElementById("stats");
    if (!host) return;
    try {
      const data = await API.overview();
      const tiles = [
        ["Queues", data.queues, "queues"],
        ["Client profiles", data.clientProfiles, "client profiles"],
        ["ACL profiles", data.aclProfiles, "ACL profiles"],
        ["Client usernames", data.clientUsernames, "client usernames"],
      ];
      const vpn = state.env?.msgVpn || "the message VPN";
      // Built once, then updated in place, so a hover in progress survives.
      if (host.children.length !== tiles.length) {
        host.replaceChildren(...tiles.map(([label]) => h("div", { class: "stat" },
          h("div", { class: "stat__label", text: label }),
          h("div", { class: "stat__value" },
            h("span", { class: "stat__number" }),
            h("span", { class: "stat__unit", text: "on broker" })))));
      }
      tiles.forEach(([, value, noun], i) => {
        const tile = host.children[i];
        tile.querySelector(".stat__number").textContent = value === null ? "?" : String(value);
        indicator(tile, "broker-total", value === null
          ? `The broker did not answer, so the number of ${noun} is not known. It is read again every 5 seconds.`
          : `${value} ${noun} in message VPN ${vpn} right now, across every scenario and including the broker's own defaults. Read every 5 seconds. This scenario's own objects are listed on the broker box in the diagram.`);
      });
    } catch (_) {
      host.replaceChildren();
    }
  }


  /* --------------------------------------------------------- node panel */

  /* Clicking a node opens this beside the diagram: what the node needs, whether
     it has it, and its logs. Everything about a node in one place, so nothing
     has to live further down the page. */

  function nodeSpec(scenario, nodeId) {
    return (scenario.diagram?.nodes || []).find((n) => n.id === nodeId);
  }

  function selectNode(scenario, nodeId, tab) {
    const spec = nodeSpec(scenario, nodeId);
    if (!spec) return;

    // Re-selecting the open node with no tab given closes the panel, so a
    // second click on a node is a way back to the full-width diagram.
    if (state.panel?.nodeId === nodeId && !tab) {
      closePanel();
      return;
    }

    state.panel = { nodeId, tab: tab || state.panel?.tab || "detail", scenario };
    renderPanel();
  }

  function closePanel() {
    state.panel = null;
    closePanelSocket();
    const host = document.getElementById("panel-host");
    if (host) host.replaceChildren();
    document.querySelector(".stage")?.classList.remove("is-open");
    for (const n of document.querySelectorAll(".dg-node")) n.classList.remove("is-selected");
  }

  function closePanelSocket() {
    if (state.panelSocket) {
      state.panelSocket.close();
      state.panelSocket = null;
    }
  }

  function renderPanel() {
    const host = document.getElementById("panel-host");
    if (!host || !state.panel) return;
    const { nodeId, tab, scenario } = state.panel;
    const spec = nodeSpec(scenario, nodeId);
    const action = spec.action
      ? scenario.actions.find((a) => a.id === spec.action)
      : null;
    /* While a failure mode marks this node, its logs are the failure's, not
       the node's own app: a refused subscription happens in a separate
       process, and the healthy subscriber's log would show nothing wrong. */
    const failing = (scenario.failureModes || []).find((m) =>
      failureActive(scenario, m) && boundOf(scenario, m).node === nodeId && boundOf(scenario, m).logs);
    // A variant running in this node's place has the log worth reading.
    const standIn = state.diagramState?.nodes?.[nodeId]?.standIn;
    const ownAction = (standIn && scenario.actions.find((a) => a.id === standIn)) || action;
    const logAction = failing
      ? scenario.actions.find((a) => a.id === boundOf(scenario, failing).logs) || ownAction
      : ownAction;

    closePanelSocket();
    document.querySelector(".stage")?.classList.add("is-open");
    for (const n of document.querySelectorAll(".dg-node")) {
      n.classList.toggle("is-selected", n.dataset.node === nodeId);
    }

    const tabBtn = (id, label) =>
      h("button", {
        class: `panel__tab${tab === id ? " is-active" : ""}`,
        text: label,
        onClick: () => {
          state.panel.tab = id;
          renderPanel();
        },
      });

    const head = h(
      "div",
      { class: "panel__head" },
      h("div", { class: "panel__titles" },
        h("p", { class: "panel__eyebrow", text: spec.kind || "node" }),
        h("h4", { class: "panel__title", text: spec.label })),
      h("button", { class: "panel__close", text: "\u00d7", indicator: ["close-panel", "Close this panel"],
        onClick: closePanel })
    );

    const tabs = h(
      "div",
      { class: "panel__tabs" },
      tabBtn("detail", "Detail"),
      logAction && tabBtn("logs", "Logs")
    );

    const body = h("div", { class: "panel__body", id: "panel-body" });
    host.replaceChildren(head, tabs, body);

    if (tab === "logs" && logAction) {
      renderPanelLogs(scenario, logAction, body, failing);
    } else {
      renderPanelDetail(scenario, spec, action, body);
    }
  }

  function renderPanelDetail(scenario, spec, action, body) {
    const nodeState = state.diagramState?.nodes?.[spec.id];
    const checklist = nodeState?.checklist || [];

    const standIn = nodeState?.standIn && scenario.actions.find((a) => a.id === nodeState.standIn);
    const statusText = standIn
      ? `Running as: ${standIn.label}`
      : nodeState?.running
      ? "Running"
      : nodeState?.run
      ? STATE_LABEL[nodeState.run.state] || "Idle"
      : "Not started";

    const rows = [
      h("div", { class: "panel__status" },
        h("span", { class: `dot dot--${nodeState?.running ? "ready" : "idle"}` }),
        h("span", { text: statusText })),
    ];

    if (spec.sublabel) {
      rows.push(h("p", { class: "panel__meta", text: spec.sublabel }));
    }

    if (checklist.length) {
      rows.push(
        h("p", { class: "panel__section-title",
          text: spec.provides ? "Configuration it provides" : "Configuration it needs" }),
        h("ul", { class: "checklist" },
          checklist.map((c) =>
            h("li", { class: `checklist__item is-${c.present === null ? "unknown" : c.present}` },
              h("span", { class: "checklist__mark",
                text: c.present === null ? "?" : c.present ? "\u2713" : "\u00d7" }),
              h("span", { text: c.label }))))
      );
    }

    for (const [name, setting] of Object.entries(scenario.settings || {})) {
      if (setting.node !== spec.id) continue;
      const current = choiceOf(scenario, name);
      rows.push(
        h("p", { class: "panel__section-title", text: setting.label }),
        h("div", { class: "panel__choices", role: "group", "aria-label": setting.label },
          setting.choices.map((c) => h("button", {
            class: `btn btn--${c.value === current?.value ? "primary" : "ghost"} btn--sm`,
            "aria-pressed": String(c.value === current?.value),
            "data-setting": name,
            "data-value": c.value,
            text: c.label,
            indicator: ["setting-choice", `${c.description.trim()} Choosing it restarts the app if it is running.`],
            onClick: () => chooseSetting(scenario, name, c.value),
          }))),
        h("p", { class: "panel__desc", text: current?.description || "" }),
      );
    }

    if (action) {
      const running = Boolean(nodeState?.running);
      rows.push(
        h("div", { class: "panel__actions" },
          h("button", {
            class: `btn btn--${running ? "secondary" : "primary"} btn--sm`,
            text: running ? "Stop" : "Start",
            // Read state at click time, not render time: the label is updated
            // in place by polling, so a captured value would go stale.
            // A variant running in this node's place is what Stop stops.
            onClick: () => {
              const now = state.diagramState?.nodes?.[spec.id];
              const current = (now?.standIn && scenario.actions.find((a) => a.id === now.standIn)) || action;
              toggleNode(scenario, current, Boolean(now?.running));
            },
          }),
          h("button", {
            class: "btn btn--ghost btn--sm",
            text: "View logs",
            onClick: () => { state.panel.tab = "logs"; renderPanel(); },
          }))
      );
      if (action.description) {
        rows.push(h("p", { class: "panel__desc", text: action.description }));
      }
    }

    body.replaceChildren(...rows);
  }

  function renderPanelLogs(scenario, action, body, failing) {
    const pre = h("pre", { class: "panel__log" });
    // replaceChildren, unlike h(), turns undefined into the text "undefined",
    // so the notice is only passed when there is one.
    const notice = failing
      ? h("p", {
          class: "panel__notice",
          text: `"${failing.title}" is in effect, so this shows the output of "${action.label}". Reset it to see this node's own log again.`,
        })
      : null;
    body.replaceChildren(...(notice ? [notice] : []), pre);

    const append = (line) => appendLog(pre, line);

    // A dedicated socket for whatever the panel is showing, closed on every
    // switch so a long session cannot accumulate one per node visited.
    state.panelSocket = API.streamRun(scenario.id, action.id, {
      onHistory: (lines) => { pre.textContent = ""; lines.forEach(append); },
      onLine: append,
    });
  }

  /* Update the open Detail tab without rebuilding it, so a poll landing between
     a mousedown and its click cannot swallow the press. */
  function repaintPanelDetail() {
    if (!state.panel) return;
    const { nodeId, scenario } = state.panel;
    const spec = nodeSpec(scenario, nodeId);
    const nodeState = state.diagramState?.nodes?.[nodeId];
    if (!spec || !nodeState) return;

    const statusEl = document.querySelector("#panel-body .panel__status");
    if (statusEl) {
      const standIn = nodeState.standIn && scenario.actions.find((a) => a.id === nodeState.standIn);
      const text = standIn
        ? `Running as: ${standIn.label}`
        : nodeState.running
        ? "Running"
        : nodeState.run
        ? STATE_LABEL[nodeState.run.state] || "Idle"
        : "Not started";
      statusEl.lastElementChild.textContent = text;
      statusEl.firstElementChild.className = `dot dot--${nodeState.running ? "ready" : "idle"}`;
    }

    const items = document.querySelectorAll("#panel-body .checklist__item");
    (nodeState.checklist || []).forEach((c, i) => {
      const li = items[i];
      if (!li) return;
      li.className = `checklist__item is-${c.present === null ? "unknown" : c.present}`;
      li.firstElementChild.textContent =
        c.present === null ? "?" : c.present ? "\u2713" : "\u00d7";
    });

    const btn = document.querySelector("#panel-body .panel__actions .btn");
    if (btn) {
      const label = nodeState.running ? "Stop" : "Start";
      // Only touch the button when it is actually wrong, so an in-flight click
      // is never interrupted by a needless rewrite.
      if (btn.textContent !== label) {
        btn.textContent = label;
        btn.className = `btn btn--${nodeState.running ? "secondary" : "primary"} btn--sm`;
      }
    }
  }

  async function toggleNode(scenario, action, running) {
    try {
      if (running) {
        await API.stopAction(scenario.id, action.id);
      } else {
        await API.startAction(scenario.id, action.id);
      }
    } catch (err) {
      toast(err.message, true);
    }
  }

  // Diagram nodes driven by a long-running process, whose packets follow it.
  function gatedNodes(scenario) {
    const longRunning = new Set(
      (scenario.actions || []).filter((a) => a.longRunning).map((a) => a.id));
    return new Set((scenario.diagram?.nodes || [])
      .filter((n) => longRunning.has(n.action)).map((n) => n.id));
  }

  /* A diagram with a partitioned queue reads its lanes and the updates the
     consumers handled about once a second, for as long as it is on screen.
     Each answer carries `until`, where the next one picks up. */
  function watchPartitions(scenario, host) {
    const hasLanes = (scenario.diagram?.nodes || [])
      .some((n) => (n.contains || []).some((c) => c.partitions));
    if (!hasLanes) return;
    let since = 0;
    const tick = async () => {
      if (state.activeId !== scenario.id || !host.isConnected) return;
      if (document.visibilityState === "visible") {
        try {
          const data = await API.partitions(scenario.id, since);
          since = data.until;
          Diagram.paintPartitions(host, data);
        } catch (_) {
          /* transient; the next tick will catch up */
        }
      }
      setTimeout(tick, 1000);
    };
    // The host is put on the page after this returns.
    setTimeout(tick, 0);
  }

  /* A setting (the Order Service's pace, say) shows its current choice as
     the sublabel of the node it belongs to. */
  function choiceOf(scenario, name) {
    const spec = scenario.settings?.[name];
    const value = state.diagramState?.settings?.[name] ?? spec?.default;
    return spec?.choices.find((c) => c.value === value);
  }

  function paintSettings(scenario) {
    for (const [name, spec] of Object.entries(scenario.settings || {})) {
      const choice = choiceOf(scenario, name);
      const sub = document.querySelector(`.dg-node[data-node="${spec.node}"] .dg-node__sub`);
      if (!sub || !choice) continue;
      sub.textContent = choice.sublabel;
      indicator(sub, "node-setting",
        `${spec.label}: ${choice.label}. ${choice.description.trim()} Change it in this node's panel.`);
    }
  }

  async function chooseSetting(scenario, name, value) {
    const spec = scenario.settings[name];
    try {
      const { restarted } = await API.setSetting(scenario.id, name, value);
      const label = spec.choices.find((c) => c.value === value)?.label;
      toast(restarted.length
        ? `${spec.label} set to ${label}. Restarted: ${labelsFor(scenario, restarted).join(", ")}`
        : `${spec.label} set to ${label}. It applies the next time it starts`);
      await refreshDiagramState(scenario);
      renderPanel();
    } catch (err) {
      toast(err.message, true);
    }
  }

  /* ------------------------------------------------------------ holds

     What each app holds (a copy, a read model, a database), as a table under
     the diagram, for the nodes whose yaml has holds:. The app reports it
     itself (solace.NewHolding in Go); the dashboard keeps the latest report
     of the run now in place, so a restarted app starts from nothing. Kept
     in memory means gone when the app stops, and the card says so rather
     than showing rows that no longer exist anywhere. The diagram's
     holdsCaption says what to look for; each card's role: says what that
     app is in the pattern. */

  function holdNodes(scenario) {
    return (scenario.diagram?.nodes || []).filter((n) => n.holds);
  }

  function makeHolds(scenario) {
    const nodes = holdNodes(scenario);
    if (!nodes.length) return null;
    const caption = scenario.diagram?.holdsCaption;
    return h("section", { class: "holds", id: "holds", "aria-label": "What each app holds" },
      h("p", { class: "holds__head", text: "What each app holds" }),
      caption && h("p", { class: "holds__lead", text: caption }),
      h("div", { class: "holds__cards" },
        nodes.map((n) => h("article", { class: "hold", "data-hold": n.id },
          h("div", { class: "hold__top" },
            h("span", { class: "hold__app", text: n.label }),
            h("span", { class: "hold__kept", "data-kept": n.holds.kept || "memory" })),
          h("p", { class: "hold__role", text: n.holds.role || "" }),
          h("p", { class: "hold__match", hidden: true }),
          h("div", { class: "hold__body" }),
          h("p", { class: "hold__change", hidden: true })))));
  }

  /* What changed between two reports of the same table, in words: "C-03:
     city Lyon to Leeds, version 6 to 7". Rows are matched on their first
     cell. Several rows at once (a fill, a republish) are summed up. */
  function describeChange(columns, before, rows) {
    const old = new Map(before.map((r) => [r[0], r]));
    const now = new Set(rows.map((r) => r[0]));
    const said = [];
    const changed = new Set();
    for (const r of rows) {
      const was = old.get(r[0]);
      if (!was) { said.push(`${r[0]} added`); changed.add(r[0]); continue; }
      const diffs = r.map((c, i) => (i && c !== was[i] ? `${columns[i] || "value"} ${was[i]} to ${c}` : null))
        .filter(Boolean);
      if (diffs.length) { said.push(`${r[0]}: ${diffs.join(", ")}`); changed.add(r[0]); }
    }
    for (const r of before) if (!now.has(r[0])) said.push(`${r[0]} gone`);
    if (!said.length) return null;
    return { changed, text: said.length > 2 ? `${said.length} rows changed` : said.join("; ") };
  }

  function paintHolds(scenario) {
    const nodes = holdNodes(scenario);
    const byId = new Map((scenario.diagram?.nodes || []).map((n) => [n.id, n]));
    for (const n of nodes) {
      const card = document.querySelector(`[data-hold="${n.id}"]`);
      if (!card) continue;
      const s = state.diagramState?.nodes?.[n.id] || {};
      const disk = n.holds.kept === "disk";
      const kept = card.querySelector(".hold__kept");
      kept.textContent = disk ? "on disk" : "in memory";
      indicator(kept, "hold-kept", disk
        ? `${n.label} keeps this on disk, so it is still there after a restart`
        : `${n.label} keeps this in memory only: a restart or a crash loses all of it`);

      const body = card.querySelector(".hold__body");
      const match = card.querySelector(".hold__match");
      const change = card.querySelector(".hold__change");
      const report = s.running ? s.holds : null;
      card.dataset.state = report ? "live" : s.running ? "starting" : "stopped";
      if (!report) {
        card.__rows = null;
        match.hidden = true;
        change.hidden = true;
        body.replaceChildren(h("p", { class: "hold__empty", text: s.running
          ? `${n.label} is starting. Nothing reported yet.`
          : !s.run
          ? `${n.label} is not running.`
          : disk
          ? `${n.label} is not running. What it keeps stays on disk, and shows here when it starts again.`
          : `Held in memory: gone when ${n.label} stopped.` }));
        continue;
      }

      // A copy says, in one line, how much of the owner's data it has.
      const m = s.match;
      if (m) {
        const owner = byId.get(n.holds.copyOf)?.label || n.holds.copyOf;
        const whole = m.matched === m.of && m.of > 0;
        const gaps = [m.missing && `${m.missing} missing`, m.different && `${m.different} different`].filter(Boolean);
        match.textContent = (whole
          ? `Matches ${owner}: ${m.matched} of ${m.of}`
          : `Matches ${owner} on ${m.matched} of ${m.of}: ${gaps.join(", ")}`)
          + (m.ownerUp ? "" : `, with ${owner} down`);
        match.dataset.whole = String(whole);
        match.hidden = false;
        indicator(match, "hold-match", `${n.label}'s copy compared with what ${owner} holds${m.ownerUp ? "" : " (as it last reported before it stopped)"}, row by row on every column both share. `
          + (whole ? "Every row is there and the same." : "Missing rows have not reached the copy; different rows are older or newer than the owner's."));
      } else {
        match.hidden = true;
      }

      const columns = report.columns || [];
      const rows = report.rows || [];
      const before = card.__rows;
      // Counters change on every read, so marking them would mark everything.
      const diff = before && !n.holds.counters ? describeChange(columns, before, rows) : null;
      if (diff) {
        change.textContent = `Last change, ${new Date().toLocaleTimeString([], { hour12: false })}: ${diff.text}`;
        change.hidden = false;
        indicator(change, "hold-last-change", `The last time ${n.label}'s table changed, read every 3 seconds. The rows that changed are green until the next read`);
      }
      const table = h("table", { class: "hold__table" },
        h("thead", {}, h("tr", {}, columns.map((c) => h("th", { text: c })))),
        h("tbody", {}, rows.length
          ? rows.map((r) => h("tr", diff?.changed.has(r[0])
              ? { class: "is-changed", indicator: ["hold-changed", "This row changed since the last read, a few seconds ago"] }
              : {}, r.map((c) => h("td", { text: c }))))
          : h("tr", {}, h("td", { class: "hold__none", colspan: String(columns.length || 1),
              text: "Holds nothing yet" }))));
      card.__rows = rows;
      // The table scrolls inside a fixed height, so a list that grows and
      // shrinks does not move the page. Each read follows the first changed
      // row, unless the pointer is on the table: then the reader is
      // scrolling it, and it stays where they left it.
      // Read before the swap: an element taken off the page reports 0.
      const scrolledTo = body.querySelector(".hold__scroll")?.scrollTop || 0;
      const scroll = h("div", { class: "hold__scroll" }, table);
      body.replaceChildren(...[scroll, report.note && h("p", { class: "hold__note", text: report.note })].filter(Boolean));
      scroll.scrollTop = scrolledTo;
      const first = table.querySelector("tr.is-changed");
      // Asked of the body, which stays: the table under the pointer was just
      // replaced, and a new element is not :hover until the pointer moves.
      if (first && !body.matches(":hover")) {
        const head = table.querySelector("thead")?.offsetHeight || 0;
        const top = first.offsetTop - head;
        const bottom = first.offsetTop + first.offsetHeight;
        if (top < scroll.scrollTop || bottom > scroll.scrollTop + scroll.clientHeight) scroll.scrollTop = top;
      }
    }
  }

  async function refreshDiagramState(scenario) {
    try {
      state.diagramState = await API.scenarioState(scenario.id);
      const host = document.getElementById("diagram-host");
      if (host) Diagram.paint(host, scenario.diagram, state.diagramState, gatedNodes(scenario));
      paintSettings(scenario);
      paintHolds(scenario);
      narration?.tourPaint?.();
      // Keep an open Detail tab honest as things start and stop, but update it
      // in place rather than rebuilding: a full re-render on every poll would
      // replace the Start/Stop button out from under a click in progress.
      if (state.panel && state.panel.tab === "detail") repaintPanelDetail();
    } catch (_) {
      /* transient; the next tick will correct it */
    }
  }

  /* ----------------------------------------------------------- transport */

  function labelsFor(scenario, ids) {
    return ids
      .map((id) => scenario.actions.find((a) => a.id === id)?.label)
      .filter(Boolean);
  }

  /* Play / Pause / Cleanup, with the steps each one runs listed underneath so
     the buttons are never a mystery box. */
  function makeTransport(scenario) {
    const playSteps = labelsFor(scenario, scenario.runSequence);
    const cleanupSteps = labelsFor(scenario, scenario.cleanupSequence);

    const play = h("button", {
      class: "btn btn--primary",
      id: "tp-play",
      text: "Play",
      onClick: () => transportPlay(scenario),
    });

    const pause = h("button", {
      class: "btn btn--secondary",
      id: "tp-pause",
      text: "Pause",
      disabled: true,
      onClick: () => transportPause(scenario),
    });

    const cleanup = h("button", {
      class: "btn btn--danger",
      id: "tp-cleanup",
      text: "Cleanup",
      disabled: cleanupSteps.length === 0,
      // Remembered so polling never re-enables a button the scenario cannot use.
      "data-empty": String(cleanupSteps.length === 0),
      onClick: () => transportCleanup(scenario),
    });

    const taskList = (title, steps, note) =>
      steps.length
        ? h(
            "div",
            { class: "tasks" },
            h("p", { class: "tasks__title", text: title }),
            h("ol", { class: "tasks__list" }, steps.map((s) => h("li", { text: s }))),
            note && h("p", { class: "tasks__note", text: note })
          )
        : null;

    return h(
      "section",
      { class: "transport" },
      h(
        "div",
        { class: "transport__controls" },
        play, pause, cleanup,
        h("span", { class: "transport__status", id: "tp-status", text: "" })
      ),
      h(
        "div",
        { class: "transport__tasks" },
        taskList("When you press Play", playSteps),
        taskList(
          "When you press Cleanup",
          cleanupSteps,
          "Anything still running is stopped first."
        )
      )
    );
  }

  async function transportPlay(scenario) {
    setTransport({ busy: true, status: "Starting..." });
    forgetFailure(scenario.id);
    // What Play does shows on the diagram, so make sure it is in view.
    const stage = document.querySelector(".stage");
    if (stage && stage.getBoundingClientRect().top > window.innerHeight * 0.5) {
      stage.scrollIntoView({ behavior: "smooth", block: "start" });
    }
    try {
      await API.runScenario(scenario.id);
    } catch (err) {
      toast(err.message, true);
      setTransport({ busy: false, status: "" });
    }
    /* The sequence advances server-side and each step's output lands in its own
       log pane over the existing sockets, so there is nothing to poll here.
       pollRuns settles the buttons once the sequence finishes. */
  }

  async function transportPause(scenario) {
    setTransport({ busy: true, status: "Stopping..." });
    forgetFailure(scenario.id);
    try {
      const { stopped } = await API.pauseScenario(scenario.id);
      toast(stopped ? `Paused: stopped ${stopped} process(es)` : "Nothing was running");
    } catch (err) {
      toast(err.message, true);
    }
  }

  async function transportCleanup(scenario) {
    const steps = labelsFor(scenario, scenario.cleanupSequence).join(", ");
    const ok = window.confirm(
      `Clean up ${scenario.title}?\n\nThis stops anything running, then: ${steps}.\n` +
        `Configuration this scenario created is removed from the broker.`
    );
    if (!ok) return;

    setTransport({ busy: true, status: "Cleaning up..." });
    forgetFailure(scenario.id);
    try {
      await API.cleanupScenario(scenario.id);
    } catch (err) {
      toast(err.message, true);
      setTransport({ busy: false, status: "" });
    }
  }

  /* One place that owns transport button state, so Play, Pause and Cleanup can
     never contradict each other or the actual run state. */
  function setTransport({ busy, active, status, failed }) {
    const play = document.getElementById("tp-play");
    const pause = document.getElementById("tp-pause");
    const cleanup = document.getElementById("tp-cleanup");
    const label = document.getElementById("tp-status");
    if (!play) return;

    play.disabled = Boolean(busy);
    play.textContent = busy ? "Working..." : active ? "Restart" : "Play";
    // Pause is only meaningful while something is actually running.
    pause.disabled = !active;
    cleanup.disabled = Boolean(busy) || cleanup.dataset.empty === "true";
    if (label && status !== undefined) {
      label.textContent = status;
      label.classList.toggle("is-failed", Boolean(failed));
    }
  }

  async function resetScenario(scenario) {
    const ok = window.confirm(
      `Reset ${scenario.title}?\n\nThis stops anything still running and deletes this scenario's ` +
        `terraform state. Configuration already on the broker is left in place. Run ` +
        `"Remove configuration" first if you want a clean broker.`
    );
    if (!ok) return;
    forgetFailure(scenario.id);
    try {
      await API.reset(scenario.id);
      toast(`${scenario.title} reset`);
      await load();
    } catch (err) {
      toast(err.message, true);
    }
  }

  /* The hash is "<scenario>" or "<scenario>/<section>", where a section is
     run, steps, break, broker or break-<n> for one card, so a guide can link
     straight to "Break it, card 2". */
  function parseHash() {
    const [id, section] = decodeURIComponent(location.hash.slice(1)).split("/");
    return { id, section: section || null };
  }

  function scrollToSection(scenario, section) {
    const content = document.querySelector(".content");
    if (!section) {
      if (content) content.scrollTop = 0;
      return;
    }
    const card = section.match(/^break-(\d+)$/);
    const mode = card && (scenario.failureModes || [])[Number(card[1]) - 1];
    // A link to one card opens it, unless another is in effect.
    if (mode) selectFailure(scenario, mode, false);
    const target = mode
      ? (document.getElementById(`fm-${mode.id}`)?.hidden ? document.getElementById("part-break")
        : document.getElementById(`fm-${mode.id}`))
      : document.getElementById(`part-${section}`);
    if (target) requestAnimationFrame(() => target.scrollIntoView({ block: "start" }));
    else if (content) content.scrollTop = 0;
  }

  function select(scenarioId, section = null) {
    if (scenarioId === state.activeId) {
      const scenario = state.scenarios.find((s) => s.id === scenarioId);
      if (scenario) scrollToSection(scenario, section);
      return;
    }
    closeSockets();
    closeFailureSocket();
    state.recovered = null;
    state.selectedFailure = null;
    closePanel();
    state.diagramState = null;
    state.activeId = scenarioId;
    // Opened from a link or the address bar: its category must show it.
    const opened = state.scenarios.find((s) => s.id === scenarioId);
    if (opened && folded.delete(opened.eyebrow)) saveFolds();
    const scenario = state.scenarios.find((s) => s.id === scenarioId);
    renderNav();
    if (scenario) {
      renderScenario(scenario);
      mountNarration(scenario);
      scrollToSection(scenario, section);
    } else {
      unmountNarration();
    }
    const hash = section ? `${scenarioId}/${section}` : scenarioId;
    if (decodeURIComponent(location.hash.slice(1)) !== hash) history.replaceState(null, "", `#${hash}`);
    // The sidebar is long; keep the open scenario's entry in view.
    el.nav.querySelector('[aria-current="true"]')?.scrollIntoView({ block: "nearest" });
  }

  /* ------------------------------------------------------------ narration */

  /* A spoken overview of the scenario's use case. Closed, it is a Listen
     button in the top bar; opened, a column on the right of the page with
     the controls and the whole transcript, and the page narrows to make
     room. The audio and its sentence timings are made by
     cockpit/scripts/narrate.py and listed in /static/audio/index.json; a
     scenario without an entry has no button. Nothing plays until the
     attendee presses play, and every page visit starts closed. */
  let narration = null;
  let toursListed = null;
  const appEl = document.querySelector(".app");
  const topbarActions = document.querySelector(".topbar__actions");

  function fmtTime(s) {
    const t = Math.max(0, Math.floor(s || 0));
    return `${Math.floor(t / 60)}:${String(t % 60).padStart(2, "0")}`;
  }

  function unmountNarration() {
    if (!narration) return;
    narration.leave();
    narration.audio.pause();
    narration.audio.removeAttribute("src");
    narration.side.remove();
    narration.launcher.remove();
    document.querySelectorAll(".tour-target").forEach((n) => n.classList.remove("tour-target"));
    appEl.classList.remove("narration-open");
    narration = null;
  }

  function mountNarration(scenario) {
    // Clear broker config and Reset draw the page again: the same scenario
    // keeps its player, and whatever it is playing.
    if (narration?.id === scenario.id) return;
    unmountNarration();
    const entry = state.narrations?.[scenario.id];
    if (!entry) return;
    const audio = new Audio();
    // Nothing is fetched until play, so leaving the page cancels no download.
    audio.preload = "none";
    audio.src = entry.src;
    const duration = () => (Number.isFinite(audio.duration) ? audio.duration : entry.duration);
    // A play cut short by pause (pressed twice quickly, or the page left) is
    // not a failure.
    const playFailed = (err) => { if (err.name !== "AbortError") toast(`Audio did not play: ${err.message}`, true); };
    const icon = (kind) => h("span", { class: `narration__icon narration__icon--${kind}`, "aria-hidden": "true" });

    const launcher = h("button", { class: "btn btn--secondary btn--sm narration-launch", type: "button",
      "aria-controls": "narration", "aria-expanded": "false" });
    const close = h("button", { class: "narration__close", type: "button", text: "×",
      "aria-label": "Close the guide column",
      indicator: ["narration-close", "Close this column. The audio keeps playing; Listen in the top bar opens it again"] });
    const play = h("button", { class: "narration__btn narration__btn--main", type: "button" });
    const rewind = h("button", { class: "narration__btn", type: "button", text: "-10s",
      "aria-label": "Back 10 seconds",
      indicator: ["narration-rewind", "Back 10 seconds. Press again to go further back"] });
    const stop = h("button", { class: "narration__btn", type: "button", "aria-label": "Stop",
      indicator: ["narration-stop", "Stop, and go back to the start"] }, icon("stop"));
    const bar = h("input", { class: "narration__bar", type: "range", min: "0", max: "1000", value: "0",
      "aria-label": "Position in the quick overview" });
    const time = h("span", { class: "narration__time" });
    const lines = entry.sentences.map((s) => {
      const line = h("button", { class: "narration__line", type: "button", text: s.text,
        indicator: ["narration-line", `Play from here (${fmtTime(s.start)})`] });
      line.addEventListener("click", () => {
        audio.currentTime = s.start;
        if (audio.paused) audio.play().catch(playFailed);
        paint();
      });
      return line;
    });
    // Two panes share the column: the guided tour, where the scenario has
    // one, and the audio overview. The tabs appear only when there are two.
    const eyebrow = h("p", { class: "eyebrow", text: "Quick overview" });
    const title = h("h2", { class: "narration__title", text: "The use case" });
    const sub = h("p", { class: "narration__sub", text: `${scenario.title}, ${fmtTime(entry.duration)}` });
    const tabs = h("div", { class: "narration__tabs", role: "tablist", hidden: true });
    const overviewPane = h("div", { class: "narration__pane", "data-pane": "overview", role: "tabpanel", "aria-label": "Quick overview" },
      h("div", { class: "narration__controls" },
        h("div", { class: "narration__row" }, rewind, play, stop),
        h("div", { class: "narration__row" }, bar, time)),
      h("p", { class: "narration__label", text: "Transcript" }),
      h("div", { class: "narration__transcript" }, lines),
      h("p", { class: "narration__credit", text: "Voice generated with ElevenLabs." }));
    const tourPane = h("div", { class: "narration__pane tour", "data-pane": "tour", role: "tabpanel", "aria-label": "Guided tour", hidden: true });
    const side = h("aside", { class: "narration", id: "narration", "aria-label": "Guided tour and quick overview of this scenario" },
      h("header", { class: "narration__head" }, h("div", {}, eyebrow, title, sub), close),
      tabs, tourPane, overviewPane);
    let tour = null;
    let pane = "overview";

    let current = -1;
    function paint() {
      const playing = !audio.paused && !audio.ended;
      play.replaceChildren(icon(playing ? "pause" : "play"));
      play.setAttribute("aria-label", playing ? "Pause" : "Play");
      indicator(play, "narration-play", playing
        ? "Pause. Play carries on from here"
        : audio.currentTime > 0 ? "Play, from where it paused" : "Play the spoken overview of this scenario's use case");
      const d = duration();
      bar.value = d ? String(Math.round((audio.currentTime / d) * 1000)) : "0";
      bar.setAttribute("aria-valuetext", `${fmtTime(audio.currentTime)} of ${fmtTime(d)}`);
      indicator(bar, "narration-progress",
        `How far through the overview you are. Drag or click to jump; ${fmtTime(audio.currentTime)} of ${fmtTime(d)}`);
      time.textContent = `${fmtTime(audio.currentTime)} / ${fmtTime(d)}`;
      indicator(time, "narration-time", `Time played, then the overview's length (${fmtTime(d)})`);

      const started = audio.currentTime > 0 || playing;
      const now = started ? entry.sentences.findLastIndex((s) => s.start <= audio.currentTime + 0.05) : -1;
      if (now !== current) {
        lines.forEach((ln, i) => {
          ln.classList.toggle("is-current", i === now);
          ln.classList.toggle("is-past", i < now);
        });
        if (now >= 0 && open()) lines[now].scrollIntoView({ block: "nearest", behavior: "smooth" });
        current = now;
      }

      const idle = tour ? "Guided tour" : "Listen: the use case";
      launcher.replaceChildren(icon(playing ? "pause" : "play"),
        h("span", { text: playing ? `Listening ${fmtTime(audio.currentTime)}` : idle }));
      launcher.classList.toggle("is-playing", playing);
      launcher.setAttribute("aria-label", playing ? `Quick overview, playing ${fmtTime(audio.currentTime)}` : idle);
      indicator(launcher, "narration-open", playing
          ? "The quick overview is playing. Click to show its controls and transcript"
          : tour
          ? "Open the guided tour: step by step through this scenario, waiting while you act. The quick overview, which is spoken, is on its second tab"
          : "Open the quick overview: a short spoken story of this scenario's use case, with its transcript");
    }

    /* ---- the guided tour ----
       A scenario with /static/tours/<id>.json gets a second pane, shown
       first. Each step has text to read (and, once voiced, to hear), may
       point at part of the page, and ends on Continue: the tour never moves
       on by itself. A step can say what it expects to find on the page; when
       that is not there yet a note says so, and Continue still works. */
    // The tour needs the width: opening it shrinks the sidebar to its
    // numbers, and closing it puts the sidebar back. A sidebar the attendee
    // had already shrunk, or changes while the tour is open, is left alone.
    let shrankSidebar = false;
    function tourRoom(on) {
      if (!tour) return;
      if (on && !state.sidebarCollapsed) { shrankSidebar = true; setSidebar(true, false); }
      else if (!on && shrankSidebar) {
        shrankSidebar = false;
        if (state.sidebarCollapsed) setSidebar(false, false);
      }
    }
    const tourKey = `eda.tour.${scenario.id}`;
    let stepAt = 0;
    const kindLabel = { listen: "Read", do: "Your turn", look: "Look" };

    function showPane(name) {
      pane = name;
      tourPane.hidden = name !== "tour";
      overviewPane.hidden = name !== "overview";
      for (const t of tabs.children) t.setAttribute("aria-selected", String(t.dataset.pane === name));
      eyebrow.textContent = name === "tour" ? "Guided tour" : "Quick overview";
      title.textContent = name === "tour" ? "Step by step" : "The use case";
      sub.textContent = name === "tour"
        ? `${scenario.title}, about ${tour.minutes} minutes`
        : `${scenario.title}, ${fmtTime(entry.duration)}`;
      if (name === "tour") paintTour(false); else clearTourTargets();
    }

    function clearTourTargets() {
      document.querySelectorAll(".tour-target").forEach((n) => n.classList.remove("tour-target"));
    }

    // What the step expects, checked against what the page knows now.
    function stepMet(expect) {
      if (!expect) return true;
      const nodes = state.diagramState?.nodes || {};
      if (expect.running && !expect.running.every((id) => nodes[id]?.running)) return false;
      if (expect.stopped && expect.stopped.some((id) => nodes[id]?.running)) return false;
      if (expect.holdsMin) {
        const n = nodes[expect.holdsMin.node];
        if (!n?.running || (n.holds?.rows?.length || 0) < expect.holdsMin.min) return false;
      }
      const cardState = (id) => document.getElementById(`fmt-${id}`)?.dataset.state;
      if (expect.card && cardState(expect.card.id) !== expect.card.state) return false;
      // "No longer broken" covers both a card just reset and one since
      // closed with Done.
      if (expect.cardNot && cardState(expect.cardNot.id) === expect.cardNot.state) return false;
      if (expect.logsOpen && !(state.panel?.nodeId === expect.logsOpen && state.panel?.tab === "logs")) return false;
      return true;
    }

    // The page is redrawn under the tour (a poll, a Reset), so the targets
    // are marked again each time. Only a new step scrolls to one, and to
    // the first that is on the page: a target such as the failure bar is
    // hidden until its card is in effect.
    function markTargets(step, scroll) {
      clearTourTargets();
      const targets = (step.highlight || []).flatMap((sel) => [...document.querySelectorAll(sel)]);
      targets.forEach((n) => n.classList.add("tour-target"));
      const shown = targets.find((n) => n.getClientRects().length);
      if (scroll && shown) shown.scrollIntoView({ behavior: "smooth", block: "center" });
    }

    function checkText(step, met) {
      return met ? "Done. Press Continue." : step.expect.note;
    }

    /* Every few seconds, with the page's own poll: only the line that says
       whether the step is done changes. The step is not drawn again, so the
       text keeps its scroll position and a click on Continue is never lost
       to a button replaced under the pointer. */
    function refreshTour() {
      if (!tour || pane !== "tour" || !open()) { clearTourTargets(); return; }
      const step = tour.steps[stepAt];
      markTargets(step, false);
      const line = tourPane.querySelector(".tour__check");
      if (!line || !step.expect) return;
      const met = stepMet(step.expect);
      line.className = `tour__check ${met ? "is-met" : "is-waiting"}`;
      line.textContent = checkText(step, met);
      indicator(line, "tour-check", met
        ? "The page shows what this step asked for"
        : "The page does not show what this step asked for yet. You can still press Continue, but the next step will describe something that is not there");
    }

    function paintTour(scroll) {
      if (!tour || pane !== "tour" || !open()) { clearTourTargets(); return; }
      const step = tour.steps[stepAt];
      const last = stepAt === tour.steps.length - 1;
      markTargets(step, scroll);

      const met = stepMet(step.expect);
      const count = h("span", { class: "tour__count", text: `Step ${stepAt + 1} of ${tour.steps.length}`,
        indicator: ["tour-step", `Where you are in the tour: step ${stepAt + 1} of ${tour.steps.length}. It is remembered if you close the tour or leave the page`] });
      const kind = h("span", { class: `tour__kind tour__kind--${step.kind}`, text: kindLabel[step.kind],
        indicator: ["tour-kind", step.kind === "do"
          ? "This step asks you to do something on the page, then press Continue"
          : step.kind === "look"
          ? "This step asks you to look at the highlighted part of the page, then press Continue"
          : "This step only explains. Read it, then press Continue"] });
      const back = h("button", { class: "btn btn--ghost btn--sm", type: "button", text: "Back", disabled: stepAt === 0,
        onClick: () => goStep(stepAt - 1) });
      const home = h("button", { class: "btn btn--ghost btn--sm tour__home", type: "button", disabled: stepAt === 0,
        "aria-label": "Go to step 1",
        indicator: ["tour-home", stepAt === 0
          ? "You are on step 1, the start of the tour"
          : "Go to step 1, the start of the tour. The scenario is left as it is: press Cleanup first if you want to start it from nothing"],
        onClick: () => goStep(0) }, h("span", { class: "tour__home-icon", "aria-hidden": "true" }));
      const next = h("button", { class: "btn btn--primary btn--sm tour__next", type: "button", text: last ? "Finish" : "Continue",
        onClick: () => (last ? finishTour() : goStep(stepAt + 1)) });
      tourPane.replaceChildren(
        h("div", { class: "tour__meta" }, count, kind),
        h("div", { class: "tour__progress", "aria-hidden": "true" },
          h("span", { style: `width:${((stepAt + 1) / tour.steps.length) * 100}%` })),
        h("div", { class: "tour__body" },
          h("h3", { class: "tour__title", text: step.title }),
          ...step.say.map((t) => h("p", { class: "tour__say", text: t })),
          step.do && h("p", { class: "tour__do" },
            h("span", { class: "tour__do-label", text: step.kind === "do" ? "Do this" : "Look at" }),
            h("span", { text: step.do })),
          step.expect && h("p", { class: `tour__check ${met ? "is-met" : "is-waiting"}`, text: checkText(step, met), "aria-live": "polite",
            indicator: ["tour-check", met
              ? "The page shows what this step asked for"
              : "The page does not show what this step asked for yet. You can still press Continue, but the next step will describe something that is not there"] })),
        h("div", { class: "tour__nav" }, h("div", { class: "tour__nav-back" }, home, back), next),
        h("p", { class: "narration__credit", text: "Captions only for now. Voice comes later." }));
    }

    function goStep(i) {
      stepAt = Math.max(0, Math.min(tour.steps.length - 1, i));
      try { localStorage.setItem(tourKey, String(stepAt)); } catch (_) { /* private window */ }
      paintTour(true);
      tourPane.querySelector(".tour__body")?.scrollTo(0, 0);
      tourPane.querySelector(".tour__next")?.focus({ preventScroll: true });
    }

    function finishTour() {
      try { localStorage.removeItem(tourKey); } catch (_) { /* private window */ }
      stepAt = 0;
      clearTourTargets();
      setOpen(false);
      toast("Tour finished. Open it again from the top bar to start over.");
    }

    // index.json lists the scenarios that have a tour, so one without is
    // never asked for and logs no failed request.
    toursListed ||= fetch("/static/tours/index.json", { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : [])).catch(() => []);
    toursListed
      .then((ids) => (ids.includes(scenario.id)
        ? fetch(`/static/tours/${scenario.id}.json`, { cache: "no-store" }).then((res) => (res.ok ? res.json() : null))
        : null))
      .then((found) => {
        if (!found || narration?.id !== scenario.id) return;
        tour = found;
        try { stepAt = Math.min(tour.steps.length - 1, Number(localStorage.getItem(tourKey)) || 0); } catch (_) { /* private window */ }
        tabs.replaceChildren(
          // Each tab says whether it has a voice, so nobody reaches for
          // headphones, or goes without, for the wrong one.
          h("button", { class: "narration__tab", type: "button", role: "tab", "data-pane": "tour",
            indicator: ["tour-tab", "Step by step through this scenario: it tells you what to do, waits while you do it, and explains what you see"],
            onClick: () => showPane("tour") },
            h("span", { text: "Guided tour" }),
            h("span", { class: "narration__tab-hint", text: tour.voiced ? "audio" : "text only",
              indicator: ["tab-audio-hint", tour.voiced
                ? "The guided tour is spoken, with each step's text shown as well"
                : "The guided tour has no voice yet: each step is text to read"] })),
          h("button", { class: "narration__tab", type: "button", role: "tab", "data-pane": "overview",
            indicator: ["overview-tab", "A short spoken story of this scenario's use case, with its transcript. Nothing to do but listen"],
            onClick: () => showPane("overview") },
            h("span", { text: "Quick overview" }),
            h("span", { class: "narration__tab-hint is-audio", text: `audio ${fmtTime(entry.duration)}`,
              indicator: ["tab-audio-hint", `The quick overview is spoken, ${fmtTime(entry.duration)} long, with a transcript to read along`] })));
        tabs.hidden = false;
        narration.tourPaint = refreshTour;
        showPane("tour");
        paint();
      })
      .catch(() => { /* no tour for this scenario */ });

    const open = () => appEl.classList.contains("narration-open");
    function setOpen(on) {
      appEl.classList.toggle("narration-open", on);
      launcher.setAttribute("aria-expanded", String(on));
      if (on) current = -2;
      tourRoom(on);
      paint();
      if (tour && pane === "tour") paintTour(on);
      else if (!on) clearTourTargets();
      if (on && pane === "overview") play.focus({ preventScroll: true });
    }

    launcher.addEventListener("click", () => setOpen(!open()));
    close.addEventListener("click", () => { setOpen(false); launcher.focus(); });
    play.addEventListener("click", () => {
      if (audio.paused || audio.ended) audio.play().catch(playFailed);
      else audio.pause();
    });
    rewind.addEventListener("click", () => { audio.currentTime = Math.max(0, audio.currentTime - 10); paint(); });
    stop.addEventListener("click", () => { audio.pause(); audio.currentTime = 0; paint(); });
    bar.addEventListener("input", () => { audio.currentTime = (Number(bar.value) / 1000) * duration(); paint(); });
    for (const ev of ["play", "pause", "ended", "timeupdate", "loadedmetadata", "seeked"]) {
      audio.addEventListener(ev, paint);
    }
    audio.addEventListener("ended", () => { audio.currentTime = 0; });
    audio.addEventListener("error", () => toast("The quick overview's audio could not be loaded.", true));

    topbarActions.prepend(launcher);
    appEl.append(side);
    narration = { id: scenario.id, audio, side, launcher,
      close: () => { if (open()) { setOpen(false); launcher.focus(); } },
      // Leaving the scenario with the tour open: give the sidebar back.
      leave: () => tourRoom(false),
      // The attendee moved the sidebar themselves: it is theirs now.
      sidebarTouched: () => { shrankSidebar = false; } };
    setOpen(false);
  }

  /* ----------------------------------------------------------------- boot */

  async function load() {
    const { scenarios } = await API.scenarios();
    state.scenarios = scenarios;
    if (!scenarios.length) {
      el.view.replaceChildren(h("div", { class: "empty", text: "No scenarios found." }));
      return;
    }
    const { id, section } = parseHash();
    const target = scenarios.find((s) => s.id === id) || scenarios[0];
    state.activeId = null;
    select(target.id, target.id === id ? section : null);
  }

  // The server only knows the broker as localhost, which is wrong for any
  // browser outside the container. Codespaces forwards each port to its own
  // host (name-3000.app.github.dev), so swap our port for the SEMP one; a
  // browser on localhost, as with a local devcontainer, keeps the server's URL.
  function brokerUiUrl(env) {
    const { protocol, hostname, port } = location;
    if (hostname === "localhost" || hostname === "127.0.0.1") return env.brokerUiUrl;
    const [first, ...rest] = hostname.split(".");
    const forwarded = !port && first.match(/^(.+)-\d+$/);
    if (forwarded) return `${protocol}//${forwarded[1]}-${env.sempPort}.${rest.join(".")}`;
    return `${protocol}//${hostname}:${env.sempPort}`;
  }

  /* Development mode only (COCKPIT_DEV=1): reload the page when a static file
     or a scenario.yaml changes, so an edit shows up without pressing reload.
     The hash keeps the open scenario. While the server restarts after a
     Python change the poll fails; the first answer after that reloads too. */
  function watchForChanges() {
    let seen = null;
    let lost = false;
    setInterval(async () => {
      try {
        const res = await fetch("/api/dev/version", { cache: "no-store" });
        if (!res.ok) return;
        const { version } = await res.json();
        if (lost || (seen !== null && version !== seen)) location.reload();
        seen = version;
      } catch {
        lost = true;
      }
    }, 1500);
  }

  /* The sidebar can shrink to a rail of pattern numbers, for more room on a
     small screen. The choice is remembered in this browser only. */
  /* remember: false is for a change the page makes by itself (the guided
     tour making room), which must not become the attendee's own setting. */
  function setSidebar(collapsed, remember = true) {
    state.sidebarCollapsed = collapsed;
    document.querySelector(".app")?.classList.toggle("is-collapsed", collapsed);
    el.sidebarToggle.setAttribute("aria-expanded", String(!collapsed));
    el.sidebarToggle.firstElementChild.textContent = collapsed ? "\u00bb" : "\u00ab";
    indicator(el.sidebarToggle, "sidebar-toggle", collapsed
      ? "Show the full sidebar, with every pattern's name"
      : "Shrink the sidebar to pattern numbers, for more room");
    if (remember) {
      try { localStorage.setItem("eda.sidebarCollapsed", collapsed ? "1" : "0"); } catch (_) { /* private window */ }
    }
    if (state.scenarios.length) {
      renderNav();
      el.nav.querySelector('[aria-current="true"]')?.scrollIntoView({ block: "nearest" });
    }
  }

  async function init() {
    /* Esc clears a highlighted path first, then collapses an expanded
       canvas, one step per press. */
    document.addEventListener("keydown", (e) => {
      if (e.key !== "Escape" || e.target === el.navSearch) return;
      const host = document.getElementById("diagram-host");
      if (host?.querySelector(".dg.has-path")) Diagram.selectPath(host, null);
      else if (document.getElementById("canvas")?.classList.contains("is-expanded")) setExpanded(false);
      else narration?.close();
    });
    window.addEventListener("resize", measureFailBar);

    let collapsed = false;
    try { collapsed = localStorage.getItem("eda.sidebarCollapsed") === "1"; } catch (_) { /* private window */ }
    setSidebar(collapsed);
    el.sidebarToggle.addEventListener("click", () => { narration?.sidebarTouched(); setSidebar(!state.sidebarCollapsed); });

    indicator(el.navSearch, "nav-search",
      "Type part of a pattern's name or category to shorten the list. Enter opens the first match; Escape clears");
    el.navFold.addEventListener("click", () => {
      const groups = releasedGroups();
      const allFolded = groups.every((g) => folded.has(g));
      folded.clear();
      if (!allFolded) groups.forEach((g) => folded.add(g));
      saveFolds();
      renderNav();
    });
    el.navSearch.addEventListener("input", () => {
      state.navQuery = el.navSearch.value.trim().toLowerCase();
      renderNav();
    });
    el.navSearch.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        el.navSearch.value = "";
        state.navQuery = "";
        renderNav();
      } else if (e.key === "Enter") {
        // Only a pattern the list shows: a held one matches on its
        // category's name but is not listed, so it is not opened either.
        const first = state.scenarios.find((s) => !held(s) && navMatches(s));
        if (first) select(first.id);
      }
    });

    state.env = await API.env();
    el.brokerLink.href = brokerUiUrl(state.env);
    // Which scenarios have an audio overview. Without the list, no player.
    try {
      const resp = await fetch("/static/audio/index.json");
      state.narrations = resp.ok ? await resp.json() : {};
    } catch (_) {
      state.narrations = {};
    }

    if (!state.env.terraformAvailable) {
      toast("Terraform is not installed in this container; terraform actions will fail.", true);
    }

    await load();

    pollBroker();
    pollRuns();
    setInterval(pollBroker, 5000);
    setInterval(pollRuns, 3000);
    // The counts above the page are read on the same 5 second beat as the
    // On the broker tables. Read only once, they went on saying "0 on
    // broker" after Play had created the queues.
    setInterval(() => {
      if (state.activeId && document.visibilityState === "visible") {
        refreshInspect(state.activeId);
        refreshStats();
      }
    }, 5000);
    if (state.env?.dev) watchForChanges();

    el.refresh.addEventListener("click", () => {
      refreshStats();
      if (state.activeId) refreshInspect(state.activeId);
    });

    el.stopAll.addEventListener("click", async () => {
      try {
        const { stopped } = await API.stopAll();
        forgetFailure();
        toast(stopped ? `Stopped ${stopped} process(es)` : "Nothing was running");
      } catch (err) {
        toast(err.message, true);
      }
    });

    /* Wipes every scenario's configuration so the next Play starts from an
       empty broker. Asks first, because queued messages go with the queues. */
    el.clearBroker.addEventListener("click", async () => {
      const ok = window.confirm(
        "Clear broker config?\n\nThis stops everything that is running, then deletes every " +
          "queue, client profile, ACL profile, client username, REST delivery point, bridge, " +
          "replay log and extra message VPN the workshop scenarios create, along with any " +
          "messages waiting on those queues. Broker defaults and anything you created yourself " +
          "are left alone.\n\nPress Play on a scenario to set it up again."
      );
      if (!ok) return;
      el.clearBroker.disabled = true;
      try {
        const { deleted, failed } = await API.clearBroker();
        forgetFailure();
        toast(
          failed.length
            ? `Cleared ${deleted} object(s); ${failed.length} could not be deleted: ${failed[0]}`
            : `Cleared ${deleted} object(s) from the broker`,
          failed.length > 0
        );
        await load();
        refreshStats();
      } catch (err) {
        toast(err.message, true);
      } finally {
        el.clearBroker.disabled = false;
      }
    });

    window.addEventListener("hashchange", () => {
      const { id, section } = parseHash();
      if (id) select(id, section);
    });

    // Polls skip a hidden tab; catch up as soon as it is shown again.
    document.addEventListener("visibilitychange", () => {
      if (!pageVisible()) return;
      pollBroker();
      pollRuns();
    });
    setInterval(tickFailBar, 1000);
  }

  init().catch((err) => {
    el.view.replaceChildren(h("div", { class: "empty", text: `The dashboard failed to start: ${err.message}` }));
  });
})();
