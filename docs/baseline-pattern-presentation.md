# Baseline improvements on pattern presentation

Status: agreed 2026-11-06. Apply to every scenario as we work down the
pattern list, one scenario at a time, and record it in the table at the end.

The audience: mid-level developers, with or without depth in
event-driven architecture or messaging patterns. The bar: the flow, the
diagram and the use case are clear without the viewer having to assume
anything.

Where the items come from: the Event-Carried State Transfer rework of
2026-11-06 (diagram flows, "What each app holds", guide payload table,
corrected claims) and the Fleet Flight Recorder console rework of the same
day (orientation panel, numbered flow diagram, plain language, readable
errors). The examples below name where each one was first done.

## The checklist

### A. Orientation: the use case before the mechanism

1. **Scenario first, in plain words.** Before any broker term, say who the
   actors are and what business thing happens (customers move city, orders
   get shipped and invoiced). *ECST: diagram caption.*
2. **The one idea, in one sentence, plus where to see it.** Say what the
   pattern is and what on the page proves it: "Compare the Customer
   Service and Shipping tables: they match, and shipping never asked the
   customer service for anything. That is event-carried state transfer."
   Point at things by name, never by position ("the first two tables"):
   positions move when the layout does. *ECST: `holdsCaption:`.*
3. **Show what the events carry.** A table of each kind of event with its
   topic and an example payload, and one line on what that tells you (the
   whole record vs. only an id). *ECST guide: "Start with what the two kinds
   of event carry".*
1a. **Stay on the pattern; a broker detail is supporting, never the
   lesson.** Every actor and every card must teach the pattern itself. A
   node or card that exists only to show a broker mechanism (a replay log
   dropping its oldest messages) is a digression: cut it, and keep the
   fact as one sentence in the guide's "How Solace does this". Where a
   broker feature stands in for a production component (a replay log for
   an event store), name it by the role it plays ("rl-esrc: event
   history") and say once that it is a stand-in. *Event Sourcing: the
   Statements Job and "the replay log rolls over" card removed; card 2
   became "The event history is lost" (2026-11-06).*
1b. **The demo is the pattern, correctly.** Check the scenario against
   the pattern's standard definition (for Event Sourcing: commands
   validated against state derived from events; the event the only write;
   state rebuilt from a complete history), not only against the page. Name
   any implementation choice that is not the pattern (here, the ledger
   consuming its own events). Where the pattern's precondition breaks, the
   app fails safe and says so, rather than carrying on as if nothing
   happened. And each numbered step says what the pattern does at that
   point; mechanics come second. *Event Sourcing: the ledger once invented
   its own transactions, decided on balances its own events had not yet
   reached, started the bank again from 0.00 when its history was lost,
   and its steps described messaging, not event sourcing (2026-11-06).*
4. **Show the contrast in the same scenario.** Where a pattern exists to
   avoid something, keep the alternative on the page so the difference is
   visible side by side. *ECST: invoicing asks for every order, shipping
   keeps a copy.*

### B. The diagram

5. **Every line ends where the message really goes.** A publish ends at
   the queue whose subscription takes it, not at the broker's edge; a topic
   that goes into several queues forks into one line per queue. No line may
   suggest a path the broker does not take. *ECST: `via:` on publishes.*
5a. **A line that does not use a queue never touches one.** A direct
   message (a lookup, a request) leaving the broker level with a queue box
   reads as coming out of that queue, because the box spans the broker's
   width. Rearrange the nodes so the line leaves bare broker wall: a queue
   box sits level with the lane of the app it feeds, so changing which app
   is on which row moves the box. Likewise a publish into a queue runs
   outside the broker and enters level with its box, never up the inside
   past another queue. Give every publish into a queue a `via:`, so it
   ends inside the box. Where the only layouts left trade a line beside a
   queue for a line crossing another at an angle, take the crossing.
   *ECST: Invoicing moved to the top row so the lookup into Customer
   Service no longer looked like it left q.ecst.invoicing; "whole customer
   records" routed outside the broker (2026-11-06). CQRS: Account Page
   moved above Checkout so the query and answer leave bare broker wall.*
5b. **Draw the step the pattern depends on, even inside the broker.**
   If the broker itself moves messages (a history read back into a queue
   at start), draw it as a hop between the two boxes (`into:`), labelled
   with when it happens. Without it the picture shows plain messaging and
   the pattern is invisible. *Event Sourcing: "read back at start" from
   rl-esrc into q.esrc.ledger.*
6. **Label lines with what they carry, specifically.** "whole customer
   records", "orders only", "orders + records", not "events". Move a label
   (`labelAt:`) when it crowds a node.
7. **Say which kind each thing is, and how it travels.** App, processor,
   queue; direct vs. guaranteed ("lookups and their replies are direct
   messages: they never wait on a queue"). Where colour or shape carries
   meaning, the page says what it means. *FFR: colour legend for plain
   code / AI / you.*
8. **Number the steps.** When the flow has an order, put step numbers on
   the lines and explain each number in a short list next to the diagram.
   *FFR: numbered badges 1-7 with a numbered list.* Built as a shared
   feature (2026-11-06): give the scenario's flows an `id:` and declare
   `paths:` on the diagram, one per message, each step a flow and a
   sentence. The page numbers the lines, lists the paths under the diagram
   as **Follow a message**, and highlights a whole path (lines, arrowheads,
   apps and queues) when its name or one of its lines is clicked. The
   checker makes each path join up through the broker. Draw every hop a
   message really takes, or a path cannot join up: CQRS gained the
   question's hop from the broker to the order history. *CQRS, ECST.*
   One publish that the broker routes into several boxes is one step:
   name the other branches with `also:` on the step, so they share its
   number and do not read as several writes by the app. *Event Sourcing.*
9. **Balanced layout, checked by eye.** Aligned columns, straight or
   single-elbow lines, arrows centred on what they connect, no box half
   empty, no label touching a line. Judge it from a real screenshot
   (`browser_pass.py`), never from coordinates. *FFR: two passes fixed an
   off-centre box and an empty half box that the coordinates hid.*
   **Avoid crossing lines as much as possible.** Check every place a line
   meets a node or a queue box up close, not only the whole diagram: two
   lines that cross where they arrive look tangled at full size. *CQRS:
   "order placed" came down past the line leaving q.cqrs.view and crossed
   it; a line now lands on the side of the box's middle it comes from.*
   **Two or more lines ending in one queue land apart.** The diagram makes
   that queue box taller, about its middle, and gives each line its own
   height, ordered by where it comes from, as long as the taller box still
   clears the other boxes and the broker's edges. Nothing to do per
   scenario; check it in the screenshot. *ECST: "orders" and "whole
   customer records" into q.ecst.shipping (2026-11-06).*
   **A line arriving never lands next to a line leaving.** Where a publish
   lands on the side of a queue box that a delivery leaves from, it lands
   clear of it: above the delivery if it comes from above, below if from
   below, and the box grows to hold both. Done by the diagram code; check
   the screenshot up close at every box with lines on both sides. *ECST:
   "whole customer records" arrived just below "orders + records" leaving
   q.ecst.shipping and read as one tangled line; CQRS: "order placed"
   beside the delivery out of q.cqrs.view (2026-11-06).*
   The gap between the two is 30 units (`OUT_GAP`), so the box is visibly
   taller and the two lines clearly apart. *Event Sourcing, q.esrc.ledger
   (2026-11-06).*
   **A delivery starts at its queue box,** as a publish ends at the box it
   goes into: the queue sends it, not the broker as a whole. Done by the
   diagram code. *Event Sourcing (2026-11-06).*
   **Labels and step numbers stay off the meeting points.** A label or a
   numbered badge must never cover an arrowhead, the end of a line, or the
   point where a line meets a box or the broker wall: those are what the
   viewer reads to tell where a message goes. Move the label (`labelAt:`)
   or the badge (the step's `at:`) along the line until both are clear.
   *ECST: badge 2 sat on the arrowhead of "whole customer records". CDC:
   "stock changed" ran into the broker's edge until moved to the middle
   (`labelAt: 0.5`).*
   **Lines are pinned, not bunched.** An app's lines spread over its side:
   one in the middle, two at either end (30% and 70% down), more evenly
   between, ordered by where the other end sits. A direct line meets the
   broker level with its pin where the wall allows, so it runs straight
   across, and moves to the nearest bare wall rather than sit level with a
   queue box; the lines on one side keep their order, so they cannot cross.
   A direct message into a processor whose top lane a queue already uses
   enters its side. All of this is in the diagram code: per scenario, give
   every publish into a queue its `via:` (and only into a queue whose
   subscription takes it: Event Bridge's promotions go into none), then
   check the screenshot. Where crossings remain, move an app to another
   row or column. *ECST: Customer Service moved to the right column, so
   lookups stay on the right and nothing crosses. CQRS: Checkout back on
   top. Reviewed on all 29 diagrams, before and after (2026-11-06).*
   **Boxes spread over a tall broker.** A box nothing leaves through sits
   in the gap its place in `contains:` gives it, between the boxes on an
   app's line, spread evenly (at most 60 px apart) rather than packed at
   the top. Order `contains:` so a hop between two boxes does not cross a
   third. *Event Sourcing: the replay hop ran through q.esrc.commands and
   its label sat on the waiting count. The same change spread Data
   Synchronization and Discard, Pause and Retry; no other diagram moved
   (2026-11-06).*

### C. Make the invisible visible

10. **Show the state the pattern is about, not only the flow.** If the
    lesson is what an app keeps (a copy, a read model, a log position),
    show it under the diagram: where it is kept (memory or disk), a role
    sentence per holder, a comparison line ("Matches Customer Service:
    8 of 8"), and what changed last. *ECST: `holds:` cards.*
11. **Never show what is not there.** A stopped app shows no rows; data
    that may be stale says so. *ECST: in-memory state shows as gone.*
11a. **A comparison shows what can drift, not only what usually agrees.**
    If two holders are compared by their latest item, a holder that lost
    earlier items (a restart of an in-memory view) still looks in sync.
    Show the count too. *CQRS: "Built from 20 orders, up to order #70"
    against "70 orders in all".*
11b. **Seed what is compared, so the comparison means something at once.**
    A copy built only from changes holds only what has changed since it
    started, so "matches on 2 of 2" says little. Start the source with
    known data that flows through the same path. *CDC: a new inventory
    database commits opening stock for six SKUs as changes #1 to #6, so the
    search index has every SKU and "6 of 6" is a real check.*
12. **Every Break it card names what to watch on the page,** not only in
    the logs ("its copy drops to 0 customers", "the failed count climbs"),
    and the smoke `expect:` block checks the same numbers, so the card and
    the check cannot drift apart. Write it from the browser pass's
    screenshot of the broken state, not from what you expect: a stopped
    app's holds card hides its table and says "not running", so a card
    that says "the table stops growing" describes something the page does
    not show. *CQRS card 2.* If what the card points at is over in a
    second (a replay, a burst of repeats), say so and say where to scroll,
    because by the time the viewer reads the output it has moved on. *CDC
    card 2.*
12a. **"What to look for" is written as the four-part `watch:`,** not one
    paragraph. The page shows each part as a labelled block beside the
    app's live output, in the same order on every card:
    - `normally`: what the app does when all is well, and what the failure
      will not touch.
    - `broke`: what is gone or failing, the exact lines to find in the
      output, and the mark under the diagram.
    - `impact`: what stops working, what keeps working (the contrast), and
      any hidden risk left behind.
    - `recover`: what Reset does, what you see when it worked, and whether
      anything was lost (and why not).

    Quoted log lines must match what the app prints exactly. An `expect:`
    must hold however soon Reset is pressed after Break it: smoke presses
    it within seconds, and so may a person. *Event Sourcing card 2: the
    rebuild after Reset may find 0 events.*
    `check_scenarios.py` rejects a `watch:` mapping with a part missing or
    an unknown one. *ECST: both cards, 2026-11-06.*

### D. Language

13. **Plain words first, the term second, defined once.** Write "a failed
    message set aside for a decision" before "dead message queue"; the
    term is defined in the guide or `000-reading-the-dashboard.md`, not
    assumed. No internal names on the page.
14. **Technical claims exactly right.** Counter names and directions
    (`dataRxMsgCount` is what the broker received from the client), "the
    order the broker received them" rather than "publish order" when there
    is no shared publisher.
15. **Real numbers for costs and timing, here and in production.** "About
    every two minutes here; in production a customer may not change for
    months."
16. **Say the hidden risk out loud.** If the recovery leaves something
    fragile, the card says so ("the held parcels live only in memory: a
    second restart would lose them without a word").

### E. Guide the viewer

17. **Tell them what to do, in order.** Numbered steps that match the
    page's sections, and a visible cue on anything clickable ("open to
    review and approve").
18. **Errors in plain words first, the raw text kept.** Lead with what the
    failure means for the viewer, with the original message one click
    away. Never hide or soften a failure.

### F. Proof

19. **Every new mark on the page is documented** in
    `guides/000-reading-the-dashboard.md` with its indicator anchor (an
    existing rule; `check_scenarios.py` enforces it).
20. **Run the scenario's checks after applying the baseline:**
    `check_scenarios.py`, `smoke.py <id>`, `browser_pass.py <id>`, and look
    at the screenshots at desktop and phone width.
20a. **With Play running, the lines move.** No check catches still lines:
    look at the page live, or count packets without `is-idle`. An app the
    diagram shows fed by nothing but itself never animates: give it the
    input it really has. (From Event Sourcing, where no line moved until
    the teller's requests fed the ledger.)

## Reference: one diagram scale (shared, nothing to do per scenario)

Every diagram is drawn at the same size per layout unit (1.3, the look of
Event Sourcing's diagram), and where the panel is narrower, every diagram
shrinks by the same factor; at phone width each fills the screen. Text and
boxes are the same size on every page, so a scenario never tunes its own
size. A diagram wider than about 1010 units would be drawn smaller than
the rest: split or rearrange it instead (`DIAGRAM_REF_W` in `diagram.js`).

## Reference: how Break it presents (shared, nothing to do per scenario)

Built 2026-11-06, first checked on ECST. Every scenario gets this from the
dashboard; a scenario only supplies its cards' text.

- **A row of tiles, one per card,** so every failure on offer is visible at
  once. Clicking a tile opens its card underneath; one card is open at a
  time.
- **The open tile is marked and disabled,** joined to its card by a notch:
  outlined while being read, red while its failure is in effect, green once
  reset. While one card is in effect, the other tiles are faded and say
  **Locked**.
- **The card stays open after Reset** on the recovery (the log switches to
  "After reset"), because watching things come back is half the lesson.
  **Done** closes it and re-enables its tile with **✓ tried**, so the row
  also shows progress. Opening another tile after a reset counts as Done.
- **The four parts of `watch:`** sit on the left, the app's live output on
  the right (stacked on narrow screens).
- `browser_pass.py` drives it the way a person does: tile, Break it, Reset,
  Done, and fails if a card does not open from its tile, closes at Reset,
  stays open after Done, or leaves its tile disabled or unticked.

## How to apply it to a scenario

1. Read the scenario's page and guide as the audience would, with this
   list beside it, and write down each item that does not hold.
2. Fix the scenario's own files first (`scenario.yaml`, the guide, its Go
   roles). A fix that needs a shared feature (like `holds:` did) is
   designed once in `docs/designs/` and reused, not built per scenario.
3. Run the checks in item 20 and look at the screenshots.
4. Record the result below: applied, or applied except named items and why
   (an item may genuinely not apply; say so rather than skip it silently).

## Progress

| # | Scenario | Baseline | Notes |
| --- | --- | --- | --- |
| 110 | Event-Carried State Transfer | applied | 2026-11-06. 9 (records land above the delivery leaving q.ecst.shipping, clear of badge 2). 12a (both cards four-part). 5a and 9 (Customer Service in the right column, no crossings, two lines into q.ecst.shipping land apart). 8 (three paths). 17 (numbered what-to-do in the guide, matching the page's sections). 18 holds: cards say what a failure means, then quote the API error as printed. Checker, smoke and browser pass 0 problems |
| 120 | CQRS | applied | 2026-11-06. 9 ("order placed" lands clear of the delivery leaving q.cqrs.view). 8 (two paths; the question's hop to the order history drawn, into its side). 17 (numbered what-to-do in the guide). Checkout back on top: no crossings. Diagram: `via:` on both publishes, Account Page above Checkout (5, 5a). Holds: the write side's order database (on disk, new) and the read side's per-customer view, with counts in the notes (10, 11a). Guide: message table, corrected "keeps its own state" and the Connected clients claim, restart risk in Try this (3, 14, 16). Cards four-part (12a). Checker, smoke and browser pass 0 problems |
| 130 | Change Data Capture | applied | 2026-11-06. 10, 11b: holds for the inventory database (on disk, seeded with six SKUs as changes #1 to #6), the connector's saved position against the latest change, and the search index (in memory) compared with the database. 8 (one path), 5 (`via:` on the publish), 6 (label moved clear of the broker). 3, 17 (payload table with "events mirror tables", numbered what-to-do). 12, 12a (both cards four-part, holds expects; card 2 says the replay is a one-second burst). 16 (Try this corrected: a restarted search index loses SKUs unchanged since). Checker, smoke and browser pass 0 problems |
| 140 | Event Sourcing | applied | 2026-11-06, reworked the same day to the standard form (1b). A teller submits commands into q.esrc.commands; the ledger validates each against balances derived from events, records an accepted one as its only write, applies it once the broker confirms, then takes the next. Rebuild at start only: the history is replayed into q.esrc.rebuild and an end-of-replay marker says when it is complete. Assumptions stated, not built: one ledger, a complete and durable history (the replay log stands in for an event store), balances in memory. The last-value queue and the ledger consuming its own events were removed. Card 2 breaks the durable-history assumption on purpose: the ledger starts from 0.00 and nothing warns. Diagram: boxes spread over the broker in their `contains:` order, so the replay hop no longer crosses q.esrc.commands; the start-only lines are drawn still. Guide: Assumptions and What the ledger checks sections. Checker 0 problems; smoke and browser pass pending |
| 210 onward | every other scenario | not yet | In list order |
