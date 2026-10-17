#!/bin/bash
#
# Send a signal to one named workshop app.
#
#   signal_app.sh <client-name> <signal> [--wait-exit]
#
# For a scenario's only instance of an app, which a failure card names
# rather than finding by role (that is signal_consumer.sh). The app is found
# by --client-name <name> on its command line, anchored on the workshop
# binary so nothing else can match. With --wait-exit, return only once the
# app has gone, for a signal the app answers by stopping itself a moment
# later (up to 15 seconds).

set -euo pipefail

CLIENT="${1:-}"
SIGNAL="${2:-}"
WAIT="${3:-}"
if [ -z "$CLIENT" ] || [ -z "$SIGNAL" ]; then
  echo "usage: signal_app.sh <client-name> <signal> [--wait-exit]" >&2
  exit 2
fi

PATTERN="bin/workshop .*--client-name $CLIENT( |\$)"
echo "Sending SIG$SIGNAL to the app connected as $CLIENT."
if ! pkill "-$SIGNAL" -f "$PATTERN"; then
  echo "No running workshop app is connected as $CLIENT." >&2
  exit 1
fi

if [ "$WAIT" = "--wait-exit" ]; then
  for _ in $(seq 1 60); do
    if ! pgrep -f "$PATTERN" >/dev/null; then
      sleep 0.5   # let the dashboard see the exit before the next step runs
      echo "$CLIENT has stopped."
      exit 0
    fi
    sleep 0.25
  done
  echo "$CLIENT is still running after 15 seconds." >&2
  exit 1
fi
