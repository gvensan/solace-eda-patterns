#!/bin/bash
#
# Empty the replay log: trim every message logged before a minute from now.
#
#   trim.sh
#
# A SEMP action, not configuration, so it is not in terraform. Used by the
# reset of "The replay log rolls over": the lost events cannot be brought
# back, so the bank starts over with an empty log.

set -euo pipefail

VPN="${SOLACE_MSG_VPN:-default}"
URL="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/action/msgVpns/$VPN/replayLogs/rl-esrc/trimLoggedMsgs"
echo "PUT $URL"
OUT="$(curl -sS -u "${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}" -X PUT \
  -H "Content-Type: application/json" -d "{\"olderThanTime\":$(( $(date +%s) + 60 ))}" "$URL")"
python3 -c '
import json, sys
meta = json.loads(sys.argv[1]).get("meta", {})
err = meta.get("error")
if err:
    print(str(meta.get("responseCode")) + " " + str(err.get("status")) + ": " + str(err.get("description")))
    sys.exit(1)
print(str(meta.get("responseCode")) + " OK: the replay log is empty")
' "$OUT"
