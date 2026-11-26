#!/bin/bash
#
# Pre-push check, run by hand before every push while the CI smoke test is
# manual only. It runs the scenario checker, then the smoke test.
#
#   scripts/smoke.sh                 the released scenarios (what attendees see)
#   scripts/smoke.sh cqrs saga       only these scenarios, released or not
#   scripts/smoke.sh --all           every scenario, about an hour
#
# The smoke test clears the whole broker as it goes, so nothing else should
# be running on the dashboard. Takes about 20 minutes for the released set.
# The full output is kept in cockpit/.state/smoke-last.log.

set -uo pipefail

usage() {
  cat <<'USAGE'
Pre-push check: the scenario checker, then the smoke test.

Usage:
  scripts/smoke.sh                 the released scenarios (about 20 minutes)
  scripts/smoke.sh <id> [<id>...]  only these scenarios, released or not
                                   (about 3 minutes each)
  scripts/smoke.sh --all           every scenario (about an hour)
  scripts/smoke.sh --list          list scenario ids, released ones marked
  scripts/smoke.sh -h | --help     this help

Which to run:
  one pattern's text or app changed      scripts/smoke.sh <id>
  shared code changed (dashboard, Go     scripts/smoke.sh
  helpers, broker setup)
  before a workshop or a release         scripts/smoke.sh --all

Notes:
  - The dashboard must be running (./dev.sh start).
  - The smoke test clears the whole broker. If anything is running on the
    dashboard, you are asked before it is stopped.
  - Close any open dashboard tab: a click there during the run fails it.
  - Full output: cockpit/.state/smoke-last.log
  - Exit code 0 means passed and safe to push.
USAGE
}

case "${1:-}" in
  -h|--help) usage; exit 0 ;;
  --all|--list|"") ;;
  -*) echo "Unknown option: $1"; echo; usage; exit 2 ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ -f .env.dev ]; then
  set -a
  # shellcheck disable=SC1091
  source .env.dev
  set +a
  unset ELEVENLABS_API_KEY
fi

PORT="${COCKPIT_PORT:-3000}"
PY="${COCKPIT_VENV:-cockpit/.venv}/bin/python"
LOG="cockpit/.state/smoke-last.log"

if [ "${1:-}" = "--list" ]; then
  if ! curl -sf "http://localhost:$PORT/api/env" >/dev/null; then
    echo "The dashboard is not running on port $PORT. Start it with: ./dev.sh start"
    exit 1
  fi
  BELOW="$(grep -Eo 'const RELEASED_BELOW = ([0-9]+|Infinity);' cockpit/static/js/cockpit.js | grep -Eo '[0-9]+|Infinity')"
  curl -s "http://localhost:$PORT/api/scenarios" | python3 -c '
import json, sys
below = float("inf") if sys.argv[1] == "Infinity" else float(sys.argv[1])
for s in json.load(sys.stdin)["scenarios"]:
    mark = "released" if s["order"] < below else "held"
    print("{:>4}  {:<9} {}".format(s["order"], mark, s["id"]))
' "$BELOW"
  exit 0
fi

case "${1:-}" in
  "")    ARGS=(--released); WHAT="the released scenarios" ;;
  --all) ARGS=();           WHAT="every scenario" ;;
  *)     ARGS=("$@");       WHAT="$*" ;;
esac

echo "== 1. Scenario checker"
if ! python3 cockpit/scripts/check_scenarios.py | tail -3; then
  echo "The checker failed. Fix that before the smoke test."
  exit 1
fi
if ! python3 cockpit/scripts/check_scenarios.py | tail -1 | grep -q " 0 problem"; then
  echo "The checker reports problems. Fix those before the smoke test."
  exit 1
fi

echo
echo "== 2. Dashboard"
if ! curl -sf "http://localhost:$PORT/api/env" >/dev/null; then
  echo "The dashboard is not running on port $PORT. Start it with: ./dev.sh start"
  exit 1
fi
ACTIVE="$(curl -s "http://localhost:$PORT/api/runs" | python3 -c 'import json,sys; print(json.load(sys.stdin)["activeCount"])')"
if [ "$ACTIVE" != "0" ]; then
  echo "$ACTIVE app(s) or step(s) are running on the dashboard."
  echo "The smoke test clears the whole broker and would stop them."
  read -r -p "Stop them and carry on? [y/N] " answer
  [ "$answer" = "y" ] || [ "$answer" = "Y" ] || { echo "Left as it is. Nothing was tested."; exit 1; }
  curl -s -X POST "http://localhost:$PORT/api/runs/stop-all" >/dev/null
  sleep 4
fi
echo "Dashboard is up, nothing else running. Close any open dashboard tab: a click there during the run fails it."

echo
echo "== 3. Smoke test: $WHAT"
START=$(date +%s)
# Keep a Mac awake for the run: a sleep stops the dashboard's timers and
# fails it. Elsewhere (a Codespace, Linux) there is nothing to hold off.
AWAKE=()
command -v caffeinate >/dev/null 2>&1 && AWAKE=(caffeinate -i)
${AWAKE[@]+"${AWAKE[@]}"} "$PY" cockpit/scripts/smoke.py ${ARGS[@]+"${ARGS[@]}"} 2>&1 | tee "$LOG" | grep -E "^=== |^Released|FAIL|expect after|Play:|Cleanup:|^Smoke tested"
RESULT=${PIPESTATUS[0]}
MINS=$(( ($(date +%s) - START) / 60 ))

echo
if [ "$RESULT" = "0" ]; then
  echo "PASSED in about $MINS minute(s). Safe to push."
else
  echo "FAILED after about $MINS minute(s). Do not push. Full output: $LOG"
fi
exit "$RESULT"
