#!/bin/bash
#
# The central team, as a script: what a governance pipeline would do once a
# request is approved. It changes the requesting team's ACL profile over
# SEMP, the same API terraform and Solace Broker Manager use.
#
#   central.sh approve            grant every request in requests.json as asked
#   central.sh approve-as <topic> grant the inventory team <topic> instead
#   central.sh revoke             remove every grant from the inventory team
#
# Granting what is already granted, or revoking what is not, is not an error.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
SEMP=../../scripts/semp.sh
VPN="${SOLACE_MSG_VPN:-default}"
BASE="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/config/msgVpns/$VPN"
AUTH="${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}"

grant() { # <team> <topic>
  out="$(bash "$SEMP" POST "/msgVpns/{vpn}/aclProfiles/acl-ig-$1/subscribeTopicExceptions" \
    "{\"subscribeTopicException\":\"$2\",\"subscribeTopicExceptionSyntax\":\"smf\"}" 2>&1)"
  if [ $? -eq 0 ] || grep -q ALREADY_EXISTS <<<"$out"; then
    echo "approved: team $1 may now subscribe to $2"
  else
    echo "$out"; return 1
  fi
}

case "${1:-}" in
  approve)
    python3 -c 'import json; [print(r["team"], r["subscribe"]) for r in json.load(open("requests.json"))]' |
      while read -r team topic; do grant "$team" "$topic" || exit 1; done ;;
  approve-as)
    [ -n "${2:-}" ] || { echo "usage: central.sh approve-as <topic>" >&2; exit 2; }
    echo "requested: ig/sales/order/>"
    grant inventory "$2" ;;
  revoke)
    curl -sS -u "$AUTH" "$BASE/aclProfiles/acl-ig-inventory/subscribeTopicExceptions?select=subscribeTopicException" |
      python3 -c 'import json,sys,urllib.parse; [print(urllib.parse.quote(e["subscribeTopicException"], safe="")) for e in json.load(sys.stdin).get("data", [])]' |
      while read -r enc; do
        bash "$SEMP" DELETE "/msgVpns/{vpn}/aclProfiles/acl-ig-inventory/subscribeTopicExceptions/smf,$enc" >/dev/null
        echo "revoked: $(python3 -c 'import sys,urllib.parse; print(urllib.parse.unquote(sys.argv[1]))' "$enc")"
      done
    echo "the inventory team may subscribe to nothing" ;;
  *) echo "usage: central.sh approve | approve-as <topic> | revoke" >&2; exit 2 ;;
esac
