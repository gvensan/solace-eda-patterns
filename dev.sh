#!/bin/bash
#
# Run the workshop on your own machine: the broker in Docker, the Solace
# Workshop Dashboard on the host, in development mode.
#
#   ./dev.sh start            start the broker (if needed) and the dashboard
#   ./dev.sh stop             stop the dashboard and every app it runs
#   ./dev.sh stop --broker    the same, and stop the broker container too
#   ./dev.sh restart          stop and start the dashboard
#   ./dev.sh status           what is running, and where
#   ./dev.sh logs             follow the dashboard log
#   ./dev.sh check [id...]    run the scenario checker, then the smoke test
#
# Broker settings come from .env.dev in the repo root if it exists (copy
# .env.dev.example), otherwise from your environment, otherwise the defaults
# in cockpit/app/config.py. Every command is safe to run again.
#
# Development mode: the open page reloads when a static file or any
# scenario.yaml changes, a scenario.yaml change is picked up without a
# restart, and the server restarts when the dashboard's Python changes (which
# stops the apps it is running). Go apps rebuild the next time they start.
#
# You need Docker, Go with a C toolchain, Python 3 and curl. If terraform is
# not on your PATH, a pinned copy is downloaded into .tools/ (git-ignored).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

if [ -f .env.dev ]; then
  set -a
  # shellcheck disable=SC1091
  source .env.dev
  set +a
  # Only narrate.py uses it, and it reads .env.dev itself: keep it out of
  # the environment every workshop app inherits.
  unset ELEVENLABS_API_KEY
fi

export COCKPIT_DEV=1
PORT="${COCKPIT_PORT:-3000}"
SEMP_PORT="${SOLACE_SEMP_PORT:-8080}"
SMF_PORT="${SOLACE_SMF_PORT:-55555}"
BROKER_NAME="${SOLACE_BROKER_NAME:-solace_${SOLACE_BROKER_VERSION:-10.26.0}}"
LOG="$ROOT/cockpit/.state/cockpit.log"
TERRAFORM_VERSION="1.16.6"

dashboard_up() { curl -sf "http://localhost:$PORT/api/env" >/dev/null 2>&1; }

need() {
  command -v "$1" >/dev/null 2>&1 && return 0
  echo "Missing: $1. $2" >&2
  exit 1
}

# A pinned terraform in .tools/bin, verified against HashiCorp's checksums,
# unless one is already on PATH.
ensure_terraform() {
  export PATH="$ROOT/.tools/bin:$PATH"
  command -v terraform >/dev/null 2>&1 && return 0

  local os arch zip tmp
  case "$(uname -s)" in
    Darwin) os=darwin ;; Linux) os=linux ;;
    *) echo "Install terraform yourself on $(uname -s)." >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;;
    *) echo "Install terraform yourself on $(uname -m)." >&2; exit 1 ;;
  esac
  need unzip "Install unzip, or install terraform yourself."
  # GNU coreutils has sha256sum, macOS has shasum; either checks the file.
  local sha
  if command -v sha256sum >/dev/null 2>&1; then sha="sha256sum"
  elif command -v shasum >/dev/null 2>&1; then sha="shasum -a 256"
  else echo "Need sha256sum or shasum to verify the terraform download." >&2; exit 1
  fi
  zip="terraform_${TERRAFORM_VERSION}_${os}_${arch}.zip"
  tmp="$(mktemp -d)"
  echo "Downloading terraform $TERRAFORM_VERSION into .tools/bin..."
  curl -fsSL -o "$tmp/$zip" "https://releases.hashicorp.com/terraform/$TERRAFORM_VERSION/$zip"
  curl -fsSL -o "$tmp/sums" "https://releases.hashicorp.com/terraform/$TERRAFORM_VERSION/terraform_${TERRAFORM_VERSION}_SHA256SUMS"
  (cd "$tmp" && grep " $zip\$" sums | $sha -c - >/dev/null) || {
    echo "terraform download failed its checksum; not installing it." >&2
    rm -rf "$tmp"; exit 1
  }
  mkdir -p "$ROOT/.tools/bin"
  unzip -o -q "$tmp/$zip" terraform -d "$ROOT/.tools/bin"
  rm -rf "$tmp"
  terraform version | head -1
}

stop_dashboard() {
  if ! pgrep -f "[u]vicorn cockpit.app" >/dev/null; then
    echo "Dashboard is not running."
    return 0
  fi
  echo "Stopping the dashboard and the apps it runs..."
  pkill -TERM -f "[u]vicorn cockpit.app" || true
  # It stops its apps on the way out, which can take a few seconds each.
  for _ in $(seq 1 30); do
    pgrep -f "[u]vicorn cockpit.app" >/dev/null || { echo "Dashboard stopped."; return 0; }
    sleep 0.5
  done
  echo "Dashboard did not stop in 15s; forcing it." >&2
  pkill -KILL -f "[u]vicorn cockpit.app" || true
}

start() {
  need docker "Install Docker Desktop or another Docker engine."
  need go "Install Go (the Solace Go API also needs a C compiler; on macOS: xcode-select --install)."
  need python3 "Install Python 3."
  ensure_terraform
  bash setup_broker.sh
  if dashboard_up; then
    echo "Dashboard already running at http://localhost:$PORT (./dev.sh restart to pick up new settings)."
  else
    bash cockpit/start_cockpit.sh
  fi
  echo ""
  echo "Dashboard:             http://localhost:$PORT"
  echo "Solace Broker Manager: http://localhost:$SEMP_PORT  (admin / admin)"
}

status() {
  local state
  state="$(docker ps -a -f "name=^${BROKER_NAME}$" --format '{{.Status}}' 2>/dev/null || true)"
  echo "Broker container $BROKER_NAME: ${state:-not created}"
  if curl -sf -u "${SOLACE_SEMP_USER:-admin}:${SOLACE_SEMP_PASSWORD:-admin}" \
       "http://localhost:$SEMP_PORT/SEMP/v2/config/about/api" >/dev/null 2>&1; then
    echo "Broker SEMP:      http://localhost:$SEMP_PORT reachable (SMF port $SMF_PORT)"
  else
    echo "Broker SEMP:      http://localhost:$SEMP_PORT not reachable"
  fi
  if dashboard_up; then
    local env runs
    env="$(curl -s "http://localhost:$PORT/api/env")"
    runs="$(curl -s "http://localhost:$PORT/api/runs" | python3 -c 'import json,sys; print(json.load(sys.stdin)["activeCount"])')"
    echo "Dashboard:        http://localhost:$PORT up, $runs app(s) or step(s) running"
    echo "                  $env"
  else
    echo "Dashboard:        not running"
  fi
}

case "${1:-}" in
  start)
    start
    ;;
  stop)
    stop_dashboard
    if [ "${2:-}" = "--broker" ]; then
      docker stop "$BROKER_NAME" >/dev/null 2>&1 && echo "Broker $BROKER_NAME stopped (its configuration is kept)." \
        || echo "Broker $BROKER_NAME was not running."
    fi
    ;;
  restart)
    stop_dashboard
    start
    ;;
  status)
    status
    ;;
  logs)
    touch "$LOG"
    exec tail -f "$LOG"
    ;;
  check)
    shift
    ensure_terraform
    python3 cockpit/scripts/check_scenarios.py
    dashboard_up || { echo "Start the dashboard first: ./dev.sh start" >&2; exit 1; }
    "${COCKPIT_VENV:-$ROOT/cockpit/.venv}/bin/python" cockpit/scripts/smoke.py "$@"
    ;;
  *)
    sed -n '3,25p' "$0" | sed 's/^# \{0,1\}//'
    exit 2
    ;;
esac
