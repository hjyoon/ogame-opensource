#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"

# Deliberately avoid `down` and `--remove-orphans`: MySQL and the PHP oracle
# share this Compose project during migration QA.
docker compose -f "$ROOT_DIR/compose.golang.yaml" up -d --build goapp
docker compose -f "$ROOT_DIR/compose.golang.yaml" ps goapp mysql mailhog
