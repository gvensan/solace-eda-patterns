#!/bin/bash
#
# One SEMP v2 config call against the workshop broker.
#
#   semp.sh <METHOD> <path> [json]
#
# <path> is relative to /SEMP/v2/config and may contain {vpn}. For a failure
# mode that changes one setting behind terraform's back, such as switching a
# queue option on to show what it does. The next Apply puts terraform's value
# back, so Play always starts from the configuration in tf/.

set -euo pipefail

METHOD="${1:-}"
SEMP_PATH="${2:-}"
BODY="${3:-}"
if [ -z "$METHOD" ] || [ -z "$SEMP_PATH" ]; then
  echo "usage: semp.sh <METHOD> <path> [json]" >&2
  exit 2
fi

VPN="${SOLACE_MSG_VPN:-default}"
URL="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/config${SEMP_PATH//\{vpn\}/$VPN}"
echo "$METHOD $URL ${BODY}"

args=(-sS -u "${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}" -X "$METHOD")
[ -n "$BODY" ] && args+=(-H "Content-Type: application/json" -d "$BODY")
OUT="$(curl "${args[@]}" "$URL")"

# SEMP puts the outcome in meta; print the broker's own words either way.
python3 -c '
import json, sys
meta = json.loads(sys.argv[1]).get("meta", {})
err = meta.get("error")
if err:
    print(str(meta.get("responseCode")) + " " + str(err.get("status")) + ": " + str(err.get("description")))
    sys.exit(1)
print(str(meta.get("responseCode")) + " OK")
' "$OUT"
