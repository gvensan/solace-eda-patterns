# Guided tour: design note and first script

Status, 2026-11-26: built for the eight released patterns (110 to 240),
text only. Not voiced yet.

What was built differs from the proposal below in three places:

- The scripts are `cockpit/static/tours/<id>.json`, listed in
  `tours/index.json`, not `tour.md` in the scenario folder. Moving them
  beside `scenario.yaml` is still the intent, once voicing settles the format.
- The guide column has two tabs, **Guided tour** first and **Quick
  overview** (the audio overview, renamed) second, each saying whether it
  has audio. Opening the tour shrinks the sidebar; closing it puts it back.
- `check_scenarios.py` checks every tour against its scenario: nodes,
  cards and held tables it points at, card titles it names, and the
  writing rules. The tours are not yet part of `smoke.py` or
  `browser_pass.py`.

The Event Streaming script at the end of this note was the first draft;
the script in use is `cockpit/static/tours/event-streaming.json`.

## What it is

A narrated, step-by-step walk through one pattern, for an attendee working
alone. Each step tells them what is on the page and what to do, then stops.
They act on the real page, press **Continue**, and the tour explains what
just happened before moving on. It runs from "what is this pattern" through
running it, breaking it and recovering, to "when would you use it".

Decisions taken:

- **Audience:** self-paced attendees. Small steps, each explained.
- **Pacing:** the tour never moves on by itself. Every step ends on a
  **Continue** button.
- **The audio overview stays.** "Listen: the use case" is the short,
  hands-free version; the tour is the hands-on one. Step 1 of a tour uses
  the overview's opening wording, so the two cannot disagree. Revisit once
  one tour has been tried.

## How a step works

1. **Tell.** A short voice clip plays, with its text as a caption. The part
   of the page it is about is highlighted (a button, a node, a card, a
   table).
2. **Wait.** The clip ends. The step shows what to do ("Press Play") and a
   **Continue** button.
3. **Check, without blocking.** If the step expects something on the page
   (apps running, a log line, rows in a table) and it is not there yet, a
   small note says so: "The counter has not restarted yet." Continue still
   works.
4. **Explain.** The next step opens by saying what the attendee is now
   looking at, and why.

A step is one of three kinds:

| Kind | The attendee | Example |
| --- | --- | --- |
| `listen` | listens, presses Continue | the pattern's definition |
| `do` | acts on the page, then presses Continue | "Press Play" |
| `look` | looks at something highlighted, then presses Continue | "Compare the two tables" |

## Rules

- **The voice never quotes a live number.** It says "a few rides"; the
  caption can show the figure from the page.
- **A tour only uses what the page already has:** Play, the cards' Break
  it and Reset, the logs, Follow a message, What each app holds. It adds no
  action of its own and never presses a button for the attendee.
- **Leaving is always safe.** Closing the tour leaves the scenario as it
  is, with the card's Reset still in reach. Reopening offers to carry on
  from the step that was left.
- **Works without sound.** Captions carry everything.
- **One source for the words.** A step that explains a card reuses that
  card's `watch` and `why` ideas, in spoken form. Renaming something on the
  page means the tour's text for it changes in the same commit, and the
  checker says when a tour was voiced from text that has since changed (as
  it does for the overview).
- **Writing style** is the attendee-facing one: short plain sentences, no
  dashes, "the dashboard", "Solace Broker Manager".

## Where it would live

Not decided in detail; the proposal:

- `cockpit/scenarios/<id>/tour.md`, one section per step, with a small
  header per step (kind, what to highlight, what to expect). Plain text, so
  the script reads as a script.
- `narrate.py` voices one clip per step, cached per step, so a wording
  change re-voices only that step.
- A tour panel in the page's right column, where the audio overview sits.
- `expect:`-style checks reuse what `smoke.py` already knows how to test,
  so a tour can be smoke tested: every `do` step is performed and its
  expectation checked.

## Cost and risk

- Shared dashboard work: the panel, highlights, the step checks, the
  checker and `narrate.py`. Built once, outside a scenario wave.
- Voicing: a tour is roughly five to six times the words of an overview.
  Per-step clips keep re-voicing small.
- Writing: one script per pattern. Start with one, review it by reading it
  aloud against the page, then decide the pace for the rest.

## First script: Event Streaming

Twelve steps, about eight minutes of listening plus the attendee's own
time. Text in quotes is spoken and shown as the caption.

### 1. The pattern (`listen`)

Highlight: the page title and The pattern panel.

"Event streaming treats events as an unbounded, ordered sequence.
Consumers read the stream in order, each keeping its own position in it,
and build state as they go. This tour takes about ten minutes. I will tell
you what to do, wait while you do it, and explain what you see. Press
Continue when you are ready."

### 2. The setting (`listen`)

Highlight: the diagram.

"This is a bike-share. Every ride start and every ride end is an event.
The Counter keeps track of which bikes are in use: it adds a bike when a
ride starts and removes it when the ride ends. That list is only right if
the Counter has seen every event, in order, from the point its list began.
In the diagram, the bikes are on the left, the broker is in the middle,
and the Counter is on the right. The City Board is not running yet. We
will start it later."

### 3. Run it (`do`)

Highlight: the Play button. Expect: the counter and the bikes running.

"Press Play. It creates a queue on the broker, starts the Counter, and
then starts the bikes. Press Continue when the dots on the diagram are
moving."

### 4. What is running (`look`)

Highlight: the queue box, then the Counter node.

"The bikes publish four ride events a second. The queue in the broker
keeps them in order, and removes each one only when the Counter
acknowledges it. So the queue is also the Counter's position in the
stream. The number on the queue is how far ahead the stream is of the
Counter. While the Counter keeps up, it stays near zero."

### 5. The Counter's log (`do`)

Highlight: the Counter's logs button. Expect: the log panel open.

"Click logs on the Counter. Once a second it prints how many bikes it has
in use, and next to it, how many the bikes themselves say are out. The
bikes put their own total in every event, as a check. The two numbers
match. Notice what the total cannot tell you: which bikes. That is only in
the events."

### 6. What the Counter holds (`look`)

Highlight: the Counter's card under What each app holds.

"Scroll down to What each app holds. This is the Counter's list, bike by
bike, built from the stream. Rows turn green as rides start. Before it
acknowledges each event, the Counter saves this list to a snapshot on
disk, together with the event it had reached. Keep that in mind for the
next step."

### 7. Crash the Counter (`do`)

Highlight: the card "The counter crashes". Expect: the counter stopped.

"Now break it. Open the card called The counter crashes, and press Break
it. The Counter is killed outright, in the middle of the stream. Press
Continue when its node on the diagram turns red."

### 8. While it is down (`look`)

Highlight: the queue box.

"The bikes have not noticed. They are still publishing. Watch the number
on the queue climb: the broker is holding every event the Counter has not
acknowledged. Nothing is lost, and the Counter's place in the stream is
kept for it. The list in its memory is gone, though. That is what the
snapshot is for."

### 9. Bring it back (`do`)

Highlight: the Reset button in the bar at the top. Expect: the counter's
log says it restored from the snapshot and resumed.

"Press Reset. The Counter starts again. In its log, look for two lines. It
restores its list from the snapshot. Then it resumes at the event where it
stopped. Its count matches the bikes again straight away. Position came
from the queue. State came from the snapshot. A stream consumer needs
both."

### 10. A late joiner (`do`)

Highlight: the card "The city board joins the stream late". Expect: the
board running, and its table has rows.

"The city wants a board that lists which bikes are out, and for how long.
Open the card called The city board joins the stream late, and press Break
it. The board starts now, in the middle of the stream. It takes the events
direct, with no queue and no snapshot."

### 11. What the board cannot know (`look`)

Highlight: the two cards under What each app holds, side by side.

"Compare the two tables. The Counter lists every bike in use. The board
lists only the rides it saw start. The bikes that were already out are
missing, and the note under the board's table says how many it cannot
name. It knows that number from the total in each event. But a total
cannot tell it which bikes, or since when. That was only in the events it
never saw. After half a minute or so the lists agree, but only because
every ride ends. A bike out all day would never appear. When you have seen
it, press Reset to stop the board."

### 12. When to use it (`listen`)

Highlight: The pattern panel.

"That is event streaming. Use it when events form a sequence that must be
processed in order: positions, telemetry, running totals, activity. The
cost is that every consumer has to keep its position and its state in
step, and a consumer that joins late has no history unless something
replays it. On Solace, a queue gives a consumer its position. A snapshot,
or a replay of the stream, gives it its state. Compare this with
Publish-Subscribe, where events are delivered and gone, and nobody has a
position to resume from. Press Cleanup when you are done, or pick another
pattern from the sidebar."

## Open questions

1. Does the pace feel right: twelve steps, about ten minutes?
2. Should a `do` step name the control only, or also say where it is on
   the page ("at the top of the page", "under the diagram")?
3. Should the tour end by pressing Cleanup as a step of its own?
