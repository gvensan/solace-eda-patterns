# Polish

Ideas held back while shared features are frozen (CLAUDE.md, Working in
waves). One line each; pick them up between waves.

- A shared speed control for every scenario's publisher, built on `settings:`.
- Partitioning: lane-to-consumer lines cross a lot; order consumers by the lanes they own.
- Partitioning: show a consumer's owned lanes as small chips, not only in its sublabel.
- Dead Message Queue: draw the hop's reason as a chip on the dot, not only a label beside the box.
- Stat tiles: also count this scenario's own objects, beside the VPN-wide totals.
- smoke.py: a group mode (`--group "Communication Patterns"`) for group-level regression.
- Clear broker config: also delete replay logs, bridges and a scenario's extra VPN (Event Sourcing, Event Bridge list them in imports.tsv, but `_TOP_LEVEL` covers queues, usernames and profiles only, and a VPN must be emptied before it can be deleted).
- Diagram: a flow whose messages cross a bridge stops its packets while the bridge's inboundState is not ready-in-sync (Event Bridge card 1 says so in text for now).
- Diagram: queue boxes whose flows lead into processors at the same height are placed at the same height and overlap (placeQueueBoxes uses each processor's lane). Discard, Pause and Retry draws its retry queue without a flow, and Data Synchronization draws the old CRM's line from the broker rather than its queue, for now.
