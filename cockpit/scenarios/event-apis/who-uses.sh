#!/bin/bash
#
# Ask the broker who still consumes a version of the shipping API: every
# queue in the VPN with a live subscription on that version's topic. This is
# the check to make before retiring a version.
#
#   who-uses.sh <v1|v2>
#
# Any answer the broker does not give (unreachable, refused, an error in
# SEMP's meta) ends with "could not ask the broker" and exit 1, never with
# the all-clear: a check whose failure reads as "safe" is worse than none.

set -uo pipefail
VERSION="${1:-}"
[ -n "$VERSION" ] || { echo "usage: who-uses.sh <v1|v2>" >&2; exit 2; }
TOPIC="ea/shipping/parcel/dispatched/$VERSION"
VPN="${SOLACE_MSG_VPN:-default}"
BASE="${SOLACE_SEMP_URL:-http://localhost:8080}/SEMP/v2/config/msgVpns/$VPN"
AUTH="${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}"

echo "GET $BASE/queues/<each queue>/subscriptions"
echo "asking the broker which queues subscribe to $TOPIC/..."
python3 - "$BASE" "$AUTH" "$TOPIC" "$VERSION" <<'PY'
import base64, json, sys, urllib.error, urllib.parse, urllib.request

base, auth, topic, version = sys.argv[1:]
headers = {"Authorization": "Basic " + base64.b64encode(auth.encode()).decode()}
ours = urllib.parse.urlsplit(base)

def fail(why):
    print(f"could not ask the broker: {why}")
    print(f"no answer about {version}: do not retire it until this check runs cleanly")
    sys.exit(1)

def get_all(url):
    rows = []
    while url:
        try:
            with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=10) as r:
                body = json.load(r)
        except urllib.error.HTTPError as e:
            # SEMP explains a refusal in the body's meta, as on a 200.
            try:
                body = json.load(e)
            except ValueError:
                fail(f"HTTP {e.code} {e.reason}")
        except (OSError, ValueError) as e:
            fail(e)
        meta = body.get("meta", {})
        if meta.get("responseCode") != 200:
            err = meta.get("error", {})
            fail(f'{meta.get("responseCode")} {err.get("status")}: {err.get("description")}')
        data = body.get("data", [])
        rows += data if isinstance(data, list) else [data]
        # A long list comes in pages. The broker writes the next page's link
        # with its own host and port, which differ from ours when the port is
        # mapped, so keep only its path and query.
        nxt = meta.get("paging", {}).get("nextPageUri")
        if nxt:
            nxt = urllib.parse.urlsplit(nxt)
            url = urllib.parse.urlunsplit(ours._replace(path=nxt.path, query=nxt.query))
        else:
            url = None
    return rows

def level_matches(sub, level):
    # "*" alone is any one level; "abc*" is a level starting with abc.
    return sub == level or (sub.endswith("*") and level.startswith(sub[:-1]))

def receives(sub, topic):
    """True if a queue subscribed to sub gets events on topic or below it.
    Wildcards count: ea/shipping/> takes v1 as surely as the v1 topic does."""
    s, t = sub.split("/"), topic.split("/")
    for i, level in enumerate(s):
        if level == ">" and i == len(s) - 1:
            return True
        if i >= len(t):
            return True  # deeper than the version's topic: some parcel topic matches it
        if not level_matches(level, t[i]):
            return False
    return len(s) == len(t)

# A VPN that does not exist still answers the queue list with an empty
# one, which would read as "nobody", so ask for the VPN first.
get_all(f"{base}?select=msgVpnName")

users = set()
for q in get_all(f"{base}/queues?count=100&select=queueName"):
    name = q["queueName"]
    path = urllib.parse.quote(name, safe="")
    for s in get_all(f"{base}/queues/{path}/subscriptions?count=100&select=subscriptionTopic"):
        if receives(s["subscriptionTopic"], topic):
            print(f'  {name} subscribes to {s["subscriptionTopic"]}')
            users.add(name)

if users:
    print(f"{len(users)} queue(s) still subscribe to {version}: retiring it now leaves them with no events")
else:
    print(f"nobody subscribes to {version}: safe to retire")
PY
