#!/bin/bash
#
# Delete every message waiting on q.ef.alarms.
#
#   purge.sh
#
# A SEMP action, not configuration. Used by the reset of "Unmatched readings
# fill the alarm queue": the readings the selector left behind are deleted,
# alarms among them if any were still waiting.

set -euo pipefail

VPN="${SOLACE_MSG_VPN:-default}"
URL="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/action/msgVpns/$VPN/queues/q.ef.alarms/deleteMsgs"
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
print(str(meta.get("responseCode")) + " OK: q.ef.alarms is empty")
' "$OUT"
