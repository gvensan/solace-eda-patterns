#!/bin/bash
#
# Move the refund subscription between the two queues.
#
#   route.sh <legacy|refunds> <add|remove>
#   route.sh start      refunds to the old system only (Play's starting point)
#   route.sh cut-over   the old route off, then the new one on
#
# Adding a subscription a queue already has, or removing one it does not
# have, is not an error here: the step's outcome is what matters, and the
# broker's answer is printed either way.

set -uo pipefail

TOPIC="stg/shop/refund/requested/v1/>"
ENCODED="stg%2Fshop%2Frefund%2Frequested%2Fv1%2F%3E"
case "${1:-}" in
  start) bash "$0" refunds remove && bash "$0" legacy add; exit ;;
  cut-over) bash "$0" legacy remove && bash "$0" refunds add; exit ;;
  legacy) QUEUE=q.stg.legacy ;;
  refunds) QUEUE=q.stg.refunds ;;
  *) echo "usage: route.sh <legacy|refunds> <add|remove> | start | cut-over" >&2; exit 2 ;;
esac

SEMP="$(dirname "${BASH_SOURCE[0]}")/../../scripts/semp.sh"
case "${2:-}" in
  add)
    if out="$(bash "$SEMP" POST "/msgVpns/{vpn}/queues/$QUEUE/subscriptions" "{\"subscriptionTopic\":\"$TOPIC\"}" 2>&1)"; then
      echo "$out"; echo "$QUEUE now receives refunds ($TOPIC)."
    elif grep -q ALREADY_EXISTS <<<"$out"; then
      echo "$out"; echo "$QUEUE already receives refunds."
    else
      echo "$out"; exit 1
    fi ;;
  remove)
    if out="$(bash "$SEMP" DELETE "/msgVpns/{vpn}/queues/$QUEUE/subscriptions/$ENCODED" 2>&1)"; then
      echo "$out"; echo "$QUEUE no longer receives refunds."
    elif grep -q NOT_FOUND <<<"$out"; then
      echo "$out"; echo "$QUEUE was not receiving refunds."
    else
      echo "$out"; exit 1
    fi ;;
  *) echo "usage: route.sh <legacy|refunds> <add|remove>" >&2; exit 2 ;;
esac
