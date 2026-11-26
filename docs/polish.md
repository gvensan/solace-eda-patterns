# Polish

Ideas held back while shared features are frozen (CLAUDE.md, Working in
waves). One line each; pick them up between waves.

- A shared speed control for every scenario's publisher, built on `settings:`.
- Partitioning: lane-to-consumer lines cross a lot; order consumers by the lanes they own.
- Partitioning: show a consumer's owned lanes as small chips, not only in its sublabel.
- Dead Message Queue: draw the hop's reason as a chip on the dot, not only a label beside the box.
- Stat tiles: also count this scenario's own objects, beside the VPN-wide totals.
- smoke.py: a group mode (`--group "Communication Patterns"`) for group-level regression.
- CI: walk the guided tours (do each step, wait for "Done") as part of `browser_pass.py` or its own job; today only the checker covers them.
- CI: as each category is released (raise `RELEASED_BELOW`), check the smoke run still fits the 40 minute limit; split by category or raise the limit when it does not.
- CI: map MQTT (1883) and REST (9000) on the workflow's broker before Deployment Patterns are released; Event Gateway failed at Play without them.
