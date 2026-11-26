#!/bin/bash
#
# Delete every request waiting on q.sa.purchases.dmq.
#
#   clear-dmq.sh
#
# A SEMP action, not configuration. Used by the reset of "Requests wait so
# long that they go stale": the requests that expired and were moved aside
# are deleted, so the next burst starts from an empty dead message queue.

set -euo pipefail

VPN="${SOLACE_MSG_VPN:-default}"
URL="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/action/msgVpns/$VPN/queues/q.sa.purchases.dmq/deleteMsgs"
echo "PUT $URL"
OUT="$(curl -sS -u "${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}" -X PUT \
  -H "Content-Type: application/json" -d '{}' "$URL")"
python3 -c '
import json, sys
meta = json.loads(sys.argv[1]).get("meta", {})
err = meta.get("error")
if err:
    print(str(meta.get("responseCode")) + " " + str(err.get("status")) + ": " + str(err.get("description")))
    sys.exit(1)
print(str(meta.get("responseCode")) + " OK: q.sa.purchases.dmq is empty")
' "$OUT"
