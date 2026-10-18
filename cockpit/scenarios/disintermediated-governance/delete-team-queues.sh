#!/bin/bash
#
# Delete the queues the teams' apps created for themselves. Terraform did not
# create them, so its destroy does not remove them. A queue that is not
# there is not an error.

cd "$(dirname "${BASH_SOURCE[0]}")"
for q in q.dg.billing.orders q.dg.billing.audit; do
  out="$(bash ../../scripts/semp.sh DELETE "/msgVpns/{vpn}/queues/$q" 2>&1)"
  if [ $? -eq 0 ]; then echo "deleted $q"
  elif grep -q NOT_FOUND <<<"$out"; then echo "$q was not there"
  else echo "$out"; exit 1
  fi
done
