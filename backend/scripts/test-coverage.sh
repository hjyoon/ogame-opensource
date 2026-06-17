#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BACKEND_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$BACKEND_DIR/.." && pwd)"
OUT="${OGAME_GO_COVERAGE_OUT:-$ROOT_DIR/.tmp/go-coverage.out}"
MIN="${OGAME_GO_COVERAGE_MIN:-97}"

mkdir -p "$(dirname "$OUT")"

cd "$BACKEND_DIR"
export CGO_ENABLED="${CGO_ENABLED:-0}"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.tmp/go-cache}"
mkdir -p "$GOCACHE"
go test ./...
go test -coverpkg=./internal/... -coverprofile="$OUT" ./internal/...

coverage="$(go tool cover -func="$OUT" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')"
awk -v actual="$coverage" -v minimum="$MIN" 'BEGIN { exit !(actual + 0 >= minimum + 0) }'
printf 'Go internal coverage %s%% >= %s%%\n' "$coverage" "$MIN"
