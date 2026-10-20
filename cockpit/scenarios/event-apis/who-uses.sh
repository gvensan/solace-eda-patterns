#!/bin/bash
#
# Ask the broker who still consumes a version of the shipping API: every
# queue in the VPN with a live subscription on that version's topic. This is
# the check to make before retiring a version.
#
#   who-uses.sh <v1|v2>

set -uo pipefail
VERSION="${1:-}"
[ -n "$VERSION" ] || { echo "usage: who-uses.sh <v1|v2>" >&2; exit 2; }
TOPIC="ea/shipping/parcel/dispatched/$VERSION"
VPN="${SOLACE_MSG_VPN:-default}"
BASE="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/config/msgVpns/$VPN"
AUTH="${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}"

OUT="$(mktemp)"
echo "GET $BASE/queues/<each queue>/subscriptions"
echo "asking the broker which queues subscribe to $TOPIC/..."
curl -sS -u "$AUTH" "$BASE/queues?count=1000&select=queueName" |
  python3 -c 'import json,sys; [print(q["queueName"]) for q in json.load(sys.stdin).get("data", [])]' |
  while read -r q; do
    curl -sS -u "$AUTH" "$BASE/queues/$q/subscriptions?count=100&select=subscriptionTopic" |
      python3 -c '
import json, sys
topic, queue = sys.argv[1], sys.argv[2]
for s in json.load(sys.stdin).get("data", []):
    t = s["subscriptionTopic"]
    if t.startswith(topic + "/") or t == topic:
        print(f"  {queue} subscribes to {t}")
' "$TOPIC" "$q"
  done > "$OUT"
n=$(wc -l < "$OUT" | tr -d ' ')
cat "$OUT"; rm -f "$OUT"
if [ "$n" -eq 0 ]; then
  echo "nobody subscribes to $VERSION: safe to retire"
else
  echo "$n queue(s) still subscribe to $VERSION: retiring it now leaves them with no events"
fi
