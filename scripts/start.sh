#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
command -v docker >/dev/null || { echo 'Install Docker with Compose before starting Cold Email.'; exit 1; }
docker compose version >/dev/null
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/app" -w /app python:3.12-alpine python scripts/configure-local.py
mkdir -p .runtime
chmod 700 .runtime
if ! mkdir .runtime/startup.lock 2>/dev/null; then
  echo 'Startup is already running. If it was interrupted, remove .runtime/startup.lock before starting again.'
  exit 1
fi
trap 'rmdir .runtime/startup.lock' EXIT
export COLD_EMAIL_LOCAL_UID="$(id -u)" COLD_EMAIL_LOCAL_GID="$(id -g)"
# The fake profile can only reach the loopback fixture, never the platform.
if [ "${PRAMAAN_FAKE:-$(sed -n 's/^PRAMAAN_FAKE=//p' .env | tail -1)}" = 1 ]; then
  export PRAMAAN_BASE_URL=http://127.0.0.1:59090 PRAMAAN_FAKE=1
  docker compose --env-file .env -f docker-compose.dev.yml run --build --rm --no-deps provision
  docker compose --env-file .env --env-file .runtime/managed.env -f docker-compose.dev.yml --profile fake up --build -d --wait --wait-timeout 600
else
  docker compose --env-file .env -f docker-compose.dev.yml run --build --rm --no-deps provision
  docker compose --env-file .env --env-file .runtime/managed.env -f docker-compose.dev.yml up --build -d --wait --wait-timeout 600
fi
printf '\nCold Email: http://localhost:5173\nAdmin: http://localhost:5174\nFirst-run setup: docker compose -f docker-compose.dev.yml logs backend\nStop: docker compose -f docker-compose.dev.yml --profile fake down\n'
