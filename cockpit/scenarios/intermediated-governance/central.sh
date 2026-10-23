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
    # "may subscribe to nothing" is printed only once the broker has said so:
    # a revoke that could not list or delete a grant must not read as done.
    # A missing profile (or VPN) lists as empty rather than failing, so ask
    # for the profile first.
    out="$(bash "$SEMP" GET "/msgVpns/{vpn}/aclProfiles/acl-ig-inventory" 2>&1)" ||
      { echo "could not ask the broker: ${out##*$'\n'}"; exit 1; }
    body="$(curl -sS -u "$AUTH" "$BASE/aclProfiles/acl-ig-inventory/subscribeTopicExceptions?count=100&select=subscribeTopicException" 2>&1)" ||
      { echo "could not ask the broker: $body"; exit 1; }
    grants="$(python3 -c '
import json, sys, urllib.parse
try:
    body = json.loads(sys.argv[1])
except ValueError:
    print("could not ask the broker: " + sys.argv[1][:200]); sys.exit(1)
meta = body.get("meta", {})
if meta.get("responseCode") != 200:
    err = meta.get("error", {})
    print("could not ask the broker: %s %s: %s" % (meta.get("responseCode"), err.get("status"), err.get("description"))); sys.exit(1)
if meta.get("paging"):
    print("could not ask the broker: more than 100 grants, revoke the rest in Solace Broker Manager"); sys.exit(1)
for e in body.get("data", []):
    print(urllib.parse.quote(e["subscribeTopicException"], safe=""))
' "$body")" || { echo "$grants"; exit 1; }
    while read -r enc; do
      [ -n "$enc" ] || continue
      out="$(bash "$SEMP" DELETE "/msgVpns/{vpn}/aclProfiles/acl-ig-inventory/subscribeTopicExceptions/smf,$enc" 2>&1)" ||
        { echo "$out"; exit 1; }
      echo "revoked: $(python3 -c 'import sys,urllib.parse; print(urllib.parse.unquote(sys.argv[1]))' "$enc")"
    done <<<"$grants"
    echo "the inventory team may subscribe to nothing" ;;
  *) echo "usage: central.sh approve | approve-as <topic> | revoke" >&2; exit 2 ;;
esac
