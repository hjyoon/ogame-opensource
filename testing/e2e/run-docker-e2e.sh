#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
CONTAINER_DIR="${OGAME_E2E_CONTAINER_DIR:-/tmp/ogame-e2e}"

cd "$ROOT_DIR"

docker compose cp "$ROOT_DIR/wwwroot/." server:/var/www/html
docker compose cp "$ROOT_DIR/download/." server:/var/www/html/download
docker compose cp "$ROOT_DIR/game/." server:/var/www/html/game
docker compose exec -T server chown -R www-data:www-data /var/www/html
docker compose exec -T server mkdir -p "$CONTAINER_DIR"
docker compose cp "$SCRIPT_DIR/." "server:$CONTAINER_DIR"
docker compose exec -T server chmod +x "$CONTAINER_DIR/container-run-all.sh"
docker compose exec -T server "$CONTAINER_DIR/container-run-all.sh"
