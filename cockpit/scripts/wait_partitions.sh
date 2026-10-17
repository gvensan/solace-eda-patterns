#!/bin/bash
#
# Wait until a partitioned queue's partitions are shared among a number of
# consumers and the broker has nothing left to move.
#
#   wait_partitions.sh <queue> <owners> [seconds]
#
# For a failure card that adds consumers one at a time: starting the next
# only once the broker has finished rebalancing for the last makes the
# outcome the same every time. Gives up after 30 seconds by default.

set -euo pipefail

QUEUE="${1:-}"
OWNERS="${2:-}"
LIMIT="${3:-30}"
if [ -z "$QUEUE" ] || [ -z "$OWNERS" ]; then
  echo "usage: wait_partitions.sh <queue> <owners> [seconds]" >&2
  exit 2
fi

BASE="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/monitor/msgVpns/${SOLACE_MSG_VPN:-default}"
AUTH="${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}"
end=$((SECONDS + LIMIT))
while [ $SECONDS -lt $end ]; do
  owners="$(curl -sf -u "$AUTH" "$BASE/queues?count=100&where=partitionQueueName==$QUEUE&select=partitionClientName" | python3 -c '
import json, sys
print(len({r.get("partitionClientName") for r in json.load(sys.stdin).get("data", []) if r.get("partitionClientName")}))' || echo 0)"
  status="$(curl -sf -u "$AUTH" "$BASE/queues/$QUEUE?select=partitionRebalanceStatus" | python3 -c '
import json, sys
print(json.load(sys.stdin).get("data", {}).get("partitionRebalanceStatus", ""))' || true)"
  if [ "$owners" = "$OWNERS" ] && [ "$status" = "ready" ]; then
    echo "$QUEUE: partitions shared among $owners consumers, nothing moving."
    exit 0
  fi
  sleep 0.5
done
echo "$QUEUE: still not shared among $OWNERS consumers after $LIMIT seconds (now $owners, $status)." >&2
exit 1
