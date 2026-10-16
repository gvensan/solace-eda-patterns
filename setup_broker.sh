#!/bin/bash

# Setup script for the Solace broker in GitHub Codespaces or a local devcontainer.
# This script assumes Docker is properly configured via devcontainer features.
#
# The broker's host ports and admin credentials come from the same SOLACE_*
# variables the dashboard reads (cockpit/app/config.py), so a developer who
# already has something on 8080 or 55555 can start this broker elsewhere and
# point the dashboard at it with the same values:
#
#   SOLACE_SEMP_PORT=18080 SOLACE_SMF_PORT=15555 SOLACE_MAP_EXTRA_PORTS=false bash setup_broker.sh
#   SOLACE_SEMP_PORT=18080 SOLACE_SMF_PORT=15555 bash cockpit/start_cockpit.sh

echo "Setting up Solace broker..."

# Check if Docker is working. On a Codespace that is resuming, this runs while
# the docker-in-docker daemon is still starting, so give it a moment first.
echo "Checking if Docker is accessible..."
for _ in $(seq 1 30); do
  docker info > /dev/null 2>&1 && break
  sleep 2
done
if ! docker info > /dev/null 2>&1; then
  echo "Error: Docker is not accessible. Please rebuild your codespace with the updated devcontainer.json"
  echo "To rebuild: Command Palette (F1) -> Codespaces: Rebuild Container"
  exit 1
fi

echo "Docker is running and accessible!"

# One name, used for every operation. Matching on a substring instead let a
# stopped "solace_10.8.1" satisfy a check that then ran `docker start solace`,
# which is a different container that does not exist.
BROKER_VERSION="${SOLACE_BROKER_VERSION:-10.26.0}"
BROKER_NAME="${SOLACE_BROKER_NAME:-solace_${BROKER_VERSION}}"
BROKER_IMAGE="solace/solace-pubsub-standard:${BROKER_VERSION}"

SEMP_PORT="${SOLACE_SEMP_PORT:-8080}"
SMF_PORT="${SOLACE_SMF_PORT:-55555}"
SEMP_USER="${SOLACE_SEMP_USER:-admin}"
SEMP_PASSWORD="${SOLACE_SEMP_PASSWORD:-admin}"
VPN="${SOLACE_MSG_VPN:-default}"

# The other protocol ports are not used by the dashboard. They are mapped for
# attendees who want to try MQTT, REST or AMQP by hand, and can be left off on
# a machine where something else already holds them.
EXTRA_PORTS=(-p 1443:1443 -p 8008:8008 -p 1883:1883 -p 5672:5672 -p 9000:9000 -p 2223:2222)
if [ "${SOLACE_MAP_EXTRA_PORTS:-true}" = "false" ]; then
  EXTRA_PORTS=()
fi

# --filter/--format matches the name exactly, unlike a grep over ps output.
running() { [ -n "$(docker ps -q -f "name=^${BROKER_NAME}$")" ]; }
exists()  { [ -n "$(docker ps -aq -f "name=^${BROKER_NAME}$")" ]; }

if running; then
  echo "Solace broker is already running!"
else
  if exists; then
    echo "Starting existing Solace container..."
    docker start "$BROKER_NAME"
  else
    echo "Installing Solace broker..."
    docker run -d -p "${SEMP_PORT}:8080" -p "${SMF_PORT}:55555" "${EXTRA_PORTS[@]}" \
      --shm-size=2g --restart unless-stopped \
      --env username_admin_globalaccesslevel=admin \
      --env "username_admin_password=${SEMP_PASSWORD}" \
      --name="$BROKER_NAME" "$BROKER_IMAGE"
  fi

  echo "Verifying Solace broker is running..."
  if running; then
    echo "Solace broker is running!"
  else
    echo "Error: Solace broker failed to start"
    echo "Docker logs:"
    docker logs "$BROKER_NAME" 2>&1 | tail -30
    exit 1
  fi
fi

# The container reports "running" long before SEMP accepts requests, so wait for
# the management interface rather than handing back a broker nothing can talk to.
echo "Waiting for the management interface..."
for _ in $(seq 1 60); do
  if curl -sf -u "${SEMP_USER}:${SEMP_PASSWORD}" "http://localhost:${SEMP_PORT}/SEMP/v2/config/msgVpns/${VPN}" >/dev/null 2>&1; then
    echo "Management interface is ready."
    break
  fi
  sleep 5
done

# Print access information
echo ""
echo "Solace Broker is ready!"
echo "Management UI: http://localhost:${SEMP_PORT}"
echo "Username: ${SEMP_USER}"
echo ""
echo "SEMP port: ${SEMP_PORT}"
echo "SMF port: ${SMF_PORT}"
if [ ${#EXTRA_PORTS[@]} -gt 0 ]; then
  echo "Web Messaging port: 8008"
  echo "MQTT port: 1883"
  echo "REST port: 9000"
  echo "AMQP port: 5672"
fi
echo ""
