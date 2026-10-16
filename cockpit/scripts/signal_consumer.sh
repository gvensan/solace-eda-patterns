#!/bin/bash
#
# Send a signal to whichever app is in a given role on a queue right now.
#
#   signal_consumer.sh <queue> <active|standby> <signal>
#
# The broker decides which consumer of an exclusive queue is active, and that
# changes as apps stop and start. A failure mode that should hit "the active
# consumer" therefore cannot name an action: it asks the broker which client
# holds that role (SEMP monitor, the queue's txFlows), then signals the
# workshop process connected under that client name. Apps that take part
# connect with a fixed name (solace.ConnectAs) and pass it as --client-name.
#
# KILL is a crash, TERM a clean stop, USR1 whatever the app treats as a stall.

set -euo pipefail

QUEUE="${1:-}"
ROLE="${2:-}"
SIGNAL="${3:-}"
if [ -z "$QUEUE" ] || [ -z "$SIGNAL" ] || { [ "$ROLE" != active ] && [ "$ROLE" != standby ]; }; then
  echo "usage: signal_consumer.sh <queue> <active|standby> <signal>" >&2
  exit 2
fi

URL="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/monitor/msgVpns/${SOLACE_MSG_VPN:-default}/queues/$QUEUE/txFlows?count=100&select=clientName,activityState"
CLIENT="$(curl -sf -u "${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}" "$URL" | python3 -c '
import json, sys
want = sys.argv[1]
for flow in json.load(sys.stdin).get("data", []):
    active = flow.get("activityState", "").startswith("active")
    if active == (want == "active"):
        print(flow["clientName"])
        break
' "$ROLE")" || true

if [ -z "$CLIENT" ]; then
  echo "No $ROLE consumer on $QUEUE right now, so there is nothing to signal." >&2
  exit 1
fi

echo "The $ROLE consumer on $QUEUE is $CLIENT. Sending it SIG$SIGNAL."
# Anchored on the workshop binary, so only the app itself can match.
PATTERN="bin/workshop .*--client-name $CLIENT( |\$)"
if ! pkill "-$SIGNAL" -f "$PATTERN"; then
  echo "No running workshop app is connected as $CLIENT." >&2
  exit 1
fi

# For a stop or a kill, return once the app has gone, so a following step
# that starts it again does not find it still running and skip it. A stop
# that is ignored for 5 seconds becomes a kill, as the dashboard's own Stop
# does: a frozen app may never finish shutting down by itself.
gone() {
  for _ in $(seq 1 "$1"); do
    if ! pgrep -f "$PATTERN" >/dev/null; then
      sleep 0.5   # let the dashboard see the exit before the next step runs
      return 0
    fi
    sleep 0.25
  done
  return 1
}
case "$SIGNAL" in
  TERM|INT)
    gone 20 && exit 0
    echo "$CLIENT did not exit on SIG$SIGNAL; sending SIGKILL."
    pkill -KILL -f "$PATTERN" || true
    gone 20 && exit 0
    ;;
  KILL)
    gone 20 && exit 0
    ;;
  *)
    exit 0
    ;;
esac
echo "$CLIENT is still running." >&2
exit 1
