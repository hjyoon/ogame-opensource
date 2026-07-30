#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE=(
  docker compose
  --ansi never
  --progress plain
)
if [[ -f "$ROOT_DIR/.env" ]]; then
  COMPOSE+=(--env-file "$ROOT_DIR/.env")
fi
COMPOSE+=(-f "$ROOT_DIR/docker-compose.yml")

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  RED=$'\033[31m'
  GREEN=$'\033[32m'
  YELLOW=$'\033[33m'
  BLUE=$'\033[34m'
  MAGENTA=$'\033[35m'
  CYAN=$'\033[36m'
  BOLD=$'\033[1m'
  RESET=$'\033[0m'
else
  RED=""
  GREEN=""
  YELLOW=""
  BLUE=""
  MAGENTA=""
  CYAN=""
  BOLD=""
  RESET=""
fi

restore_terminal() {
  if [[ -t 1 ]]; then
    printf '\033[0m\033[?25h'
  fi
}

trap restore_terminal EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

info() {
  printf '%s%s[INFO]%s %s\n' "$BLUE" "$BOLD" "$RESET" "$*"
}

success() {
  printf '%s%s[OK]%s %s\n' "$GREEN" "$BOLD" "$RESET" "$*"
}

warn() {
  printf '%s%s[WARN]%s %s\n' "$YELLOW" "$BOLD" "$RESET" "$*"
}

die() {
  printf '%s%s[ERROR]%s %s\n' "$RED" "$BOLD" "$RESET" "$*" >&2
  exit 1
}

usage() {
  cat <<EOF
${BOLD}${CYAN}OGame development stack runner${RESET}

${BOLD}Usage:${RESET}
  ./run.sh
  ./run.sh [command] [options]

Running without arguments opens an interactive menu.

${BOLD}Commands:${RESET}
  start                 Start the selected application stack (default)
  init                  Initialize a fresh universe and start its stack
  stop                  Stop the selected application service
  down                  Stop and remove all project containers and networks
  status                Show the status of all project services
  logs                  Follow logs for the selected application service

${BOLD}Options:${RESET}
  -m, --mode MODE       Runtime mode: golang, sqlite, or legacy
  -d, --detach          Run containers in the background (default)
  -f, --foreground      Run containers in the foreground
  -b, --build           Build images before starting (default)
      --no-build        Start without rebuilding images
  -y, --yes             Confirm destructive universe initialization
  -h, --help            Show this help message

${BOLD}Modes:${RESET}
  golang                Go/React application with MySQL
  sqlite                Go/React application with SQLite
  legacy                Legacy PHP oracle with MySQL

${BOLD}Examples:${RESET}
  ./run.sh
  ./run.sh start --mode golang --detach --build
  ./run.sh init --mode sqlite --yes
  ./run.sh start --mode sqlite --no-build
  ./run.sh logs --mode legacy
  ./run.sh status
  ./run.sh down

Set NO_COLOR=1 to disable colored output.

The init command permanently deletes the selected mode's existing universe
data before creating a fresh universe. Interactive use requires typing RESET;
non-interactive use requires --yes.
EOF
}

require_docker() {
  command -v docker >/dev/null 2>&1 ||
    die "Docker is required but was not found in PATH."
  docker compose version >/dev/null 2>&1 ||
    die "Docker Compose v2 is required. Install the docker compose plugin."
}

ensure_mail_network() {
  if ! docker network inspect mail_backend >/dev/null 2>&1; then
    info "Creating the external Docker network: mail_backend"
    docker network create mail_backend >/dev/null
    success "Created Docker network mail_backend."
  fi
}

compose_all() {
  "${COMPOSE[@]}" --profile golang --profile sqlite "$@"
}

validate_mode() {
  case "${1:-}" in
    golang | sqlite | legacy) ;;
    *) die "Invalid mode '${1:-}'. Expected golang, sqlite, or legacy." ;;
  esac
}

service_for_mode() {
  case "$1" in
    golang) printf '%s\n' "goapp" ;;
    sqlite) printf '%s\n' "goapp-sqlite" ;;
    legacy) printf '%s\n' "server" ;;
  esac
}

label_for_mode() {
  case "$1" in
    golang) printf '%s\n' "Go/React + MySQL" ;;
    sqlite) printf '%s\n' "Go/React + SQLite" ;;
    legacy) printf '%s\n' "Legacy PHP + MySQL" ;;
  esac
}

compose_environment_value() {
  local wanted="$1"
  local fallback="$2"
  local key
  local value

  while IFS='=' read -r key value; do
    if [[ "$key" == "$wanted" ]]; then
      printf '%s\n' "$value"
      return
    fi
  done < <("${COMPOSE[@]}" config --environment 2>/dev/null || true)
  printf '%s\n' "$fallback"
}

url_for_mode() {
  case "$1" in
    golang) printf 'http://localhost:%s\n' "$(compose_environment_value OGAME_GO_PORT 8890)" ;;
    sqlite) printf 'http://localhost:%s\n' "$(compose_environment_value OGAME_SQLITE_PORT 8891)" ;;
    legacy) printf 'http://localhost:%s\n' "$(compose_environment_value OGAME_LEGACY_PORT 8888)" ;;
  esac
}

prompt_mode() {
  printf '\n%s%sSelect a runtime mode:%s\n' "$MAGENTA" "$BOLD" "$RESET"
  printf '  %s1)%s Go/React + MySQL\n' "$CYAN" "$RESET"
  printf '  %s2)%s Go/React + SQLite\n' "$CYAN" "$RESET"
  printf '  %s3)%s Legacy PHP + MySQL\n' "$CYAN" "$RESET"
  while true; do
    printf '%sChoice [1-3]:%s ' "$BLUE" "$RESET"
    read -r choice
    case "$choice" in
      1) MODE="golang"; return ;;
      2) MODE="sqlite"; return ;;
      3) MODE="legacy"; return ;;
      *) warn "Please enter 1, 2, or 3." ;;
    esac
  done
}

prompt_yes_no() {
  local prompt="$1"
  local default="${2:-yes}"
  local answer
  local suffix="[y/N]"
  [[ "$default" == "yes" ]] && suffix="[Y/n]"

  while true; do
    printf '%s%s %s:%s ' "$BLUE" "$prompt" "$suffix" "$RESET"
    read -r answer
    if [[ -z "$answer" ]]; then
      [[ "$default" == "yes" ]]
      return
    fi
    case "${answer,,}" in
      y | yes) return 0 ;;
      n | no) return 1 ;;
      *) warn "Please answer yes or no." ;;
    esac
  done
}

confirm_universe_reset() {
  if (( ASSUME_YES == 1 )); then
    return 0
  fi

  printf '\n%s%sDANGER: Universe initialization is destructive.%s\n' \
    "$RED" "$BOLD" "$RESET"
  warn "All existing persistent data for the selected mode will be deleted."
  if [[ "$MODE" == "golang" || "$MODE" == "legacy" ]]; then
    warn "The MySQL master and universe databases are stored together and will both be reset."
  fi
  printf '%sType RESET to continue:%s ' "$RED" "$RESET"
  read -r confirmation
  [[ "$confirmation" == "RESET" ]] || {
    success "Universe initialization cancelled. Nothing changed."
    return 1
  }
}

interactive_menu() {
  printf '%s%s\n' "$CYAN" "$BOLD"
  printf '  OGame Development Stack\n'
  printf '  =======================\n'
  printf '%s' "$RESET"
  printf '\n%s%sWhat would you like to do?%s\n' "$MAGENTA" "$BOLD" "$RESET"
  printf '  %s1)%s Start a stack\n' "$CYAN" "$RESET"
  printf '  %s2)%s Initialize a fresh universe\n' "$CYAN" "$RESET"
  printf '  %s3)%s Stop one application service\n' "$CYAN" "$RESET"
  printf '  %s4)%s Stop all project services\n' "$CYAN" "$RESET"
  printf '  %s5)%s Show service status\n' "$CYAN" "$RESET"
  printf '  %s6)%s Follow application logs\n' "$CYAN" "$RESET"
  printf '  %s7)%s Exit\n' "$CYAN" "$RESET"

  while true; do
    printf '%sChoice [1-7]:%s ' "$BLUE" "$RESET"
    read -r choice
    case "$choice" in
      1)
        ACTION="start"
        prompt_mode
        if prompt_yes_no "Run in the background" "yes"; then
          DETACH=1
        else
          DETACH=0
        fi
        if prompt_yes_no "Build images before starting" "yes"; then
          BUILD=1
        else
          BUILD=0
        fi
        return
        ;;
      2)
        ACTION="init"
        prompt_mode
        if prompt_yes_no "Build images before initialization" "yes"; then
          BUILD=1
        else
          BUILD=0
        fi
        return
        ;;
      3) ACTION="stop"; prompt_mode; return ;;
      4) ACTION="down"; return ;;
      5) ACTION="status"; return ;;
      6) ACTION="logs"; prompt_mode; return ;;
      7) success "Nothing changed. Goodbye."; exit 0 ;;
      *) warn "Please enter a number from 1 to 7." ;;
    esac
  done
}

start_stack() {
  validate_mode "$MODE"
  ensure_mail_network

  local service label url
  local -a args=(up)
  service="$(service_for_mode "$MODE")"
  label="$(label_for_mode "$MODE")"
  url="$(url_for_mode "$MODE")"

  (( DETACH == 1 )) && args+=(-d)
  (( BUILD == 1 )) && args+=(--build)
  args+=("$service")

  info "Starting $label..."
  info "Application URL: $url"
  "${COMPOSE[@]}" "${args[@]}"

  if (( DETACH == 1 )); then
    success "$label is running in the background."
    "${COMPOSE[@]}" ps "$service"
  fi
}

volume_for_service_mount() {
  local service="$1"
  local destination="$2"
  local container_id
  local volume

  container_id="$(compose_all ps -aq "$service" | head -n 1)"
  if [[ -z "$container_id" ]]; then
    compose_all create "$service" >/dev/null
    container_id="$(compose_all ps -aq "$service" | head -n 1)"
  fi
  [[ -n "$container_id" ]] ||
    die "Unable to create the $service container for volume discovery."

  volume="$(
    docker inspect \
      --format "{{range .Mounts}}{{if eq .Destination \"$destination\"}}{{.Name}}{{end}}{{end}}" \
      "$container_id"
  )"
  [[ -n "$volume" ]] ||
    die "Unable to find the volume mounted at $destination for $service."
  printf '%s\n' "$volume"
}

wait_for_legacy_install() {
  local attempt
  for (( attempt = 1; attempt <= 60; attempt++ )); do
    if "${COMPOSE[@]}" exec -T server \
      test -f /var/www/html/persistent_configs/game_config.php \
      >/dev/null 2>&1; then
      success "The MySQL universe schema and configuration were created."
      return 0
    fi
    sleep 1
  done

  "${COMPOSE[@]}" logs --tail=100 server >&2 || true
  die "Universe initialization did not finish within 60 seconds."
}

wait_for_healthy_service() {
  local service="$1"
  local attempt
  local container_id
  local status

  container_id="$(compose_all ps -q "$service" | head -n 1)"
  [[ -n "$container_id" ]] || die "The $service container was not created."

  for (( attempt = 1; attempt <= 60; attempt++ )); do
    status="$(
      docker inspect \
        --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \
        "$container_id"
    )"
    case "$status" in
      healthy)
        success "$service is healthy."
        return 0
        ;;
      exited | dead)
        compose_all logs --tail=100 "$service" >&2 || true
        die "$service stopped before becoming healthy."
        ;;
    esac
    sleep 1
  done

  compose_all logs --tail=100 "$service" >&2 || true
  die "$service did not become healthy within 60 seconds."
}

initialize_mysql_universe() {
  local mysql_volume
  local web_config_volume
  local -a args=(up -d)

  ensure_mail_network
  mysql_volume="$(volume_for_service_mount mysql /var/lib/mysql)"
  web_config_volume="$(
    volume_for_service_mount server /var/www/html/persistent_configs
  )"

  info "Stopping MySQL-backed OGame services..."
  compose_all stop server goapp mysql phpmyadmin >/dev/null 2>&1 || true
  compose_all rm -sf server goapp mysql phpmyadmin >/dev/null

  info "Deleting the MySQL and legacy configuration volumes..."
  docker volume rm "$mysql_volume" "$web_config_volume" >/dev/null
  success "Deleted volumes $mysql_volume and $web_config_volume."

  (( BUILD == 1 )) && args+=(--build)
  args+=(server)
  info "Starting the legacy installer to create a fresh universe..."
  "${COMPOSE[@]}" "${args[@]}"
  wait_for_legacy_install

  if [[ "$MODE" == "golang" ]]; then
    args=(up -d)
    (( BUILD == 1 )) && args+=(--build)
    args+=(goapp)
    info "Starting Go/React against the initialized MySQL universe..."
    "${COMPOSE[@]}" "${args[@]}"
    wait_for_healthy_service goapp
  fi
}

initialize_sqlite_universe() {
  local sqlite_volume
  local -a args=(up -d)

  ensure_mail_network
  sqlite_volume="$(
    volume_for_service_mount goapp-sqlite /srv/ogame/data
  )"

  info "Stopping the SQLite application..."
  compose_all stop goapp-sqlite >/dev/null 2>&1 || true
  compose_all rm -sf goapp-sqlite >/dev/null

  info "Deleting the SQLite universe volume..."
  docker volume rm "$sqlite_volume" >/dev/null
  success "Deleted volume $sqlite_volume."

  (( BUILD == 1 )) && args+=(--build)
  args+=(goapp-sqlite)
  info "Starting Go/React to bootstrap a fresh SQLite universe..."
  "${COMPOSE[@]}" "${args[@]}"
  wait_for_healthy_service goapp-sqlite
}

initialize_universe() {
  validate_mode "$MODE"
  confirm_universe_reset || return 0

  case "$MODE" in
    golang | legacy) initialize_mysql_universe ;;
    sqlite) initialize_sqlite_universe ;;
  esac

  success "Fresh universe initialization completed."
  info "Application URL: $(url_for_mode "$MODE")"
}

stop_stack() {
  validate_mode "$MODE"
  local service label
  service="$(service_for_mode "$MODE")"
  label="$(label_for_mode "$MODE")"
  info "Stopping $label..."
  "${COMPOSE[@]}" stop "$service"
  success "$label has been stopped. Shared dependencies were left running."
}

down_all() {
  warn "Stopping all OGame project services. Persistent volumes will be kept."
  "${COMPOSE[@]}" --profile golang --profile sqlite down
  success "All project services have been stopped."
}

show_status() {
  "${COMPOSE[@]}" --profile golang --profile sqlite ps
}

follow_logs() {
  validate_mode "$MODE"
  local service label
  service="$(service_for_mode "$MODE")"
  label="$(label_for_mode "$MODE")"
  info "Following $label logs. Press Ctrl+C to stop following."
  "${COMPOSE[@]}" logs --tail=200 -f "$service"
}

ACTION=""
MODE=""
DETACH=1
BUILD=1
ASSUME_YES=0
INTERACTIVE=0

if (( $# == 0 )); then
  INTERACTIVE=1
  interactive_menu
else
  while (( $# > 0 )); do
    case "$1" in
      start | init | stop | down | status | logs)
        [[ -z "$ACTION" ]] || die "Only one command can be specified."
        ACTION="$1"
        shift
        ;;
      -m | --mode)
        (( $# >= 2 )) || die "$1 requires a value."
        MODE="$2"
        shift 2
        ;;
      --mode=*)
        MODE="${1#*=}"
        shift
        ;;
      -d | --detach)
        DETACH=1
        shift
        ;;
      -f | --foreground)
        DETACH=0
        shift
        ;;
      -b | --build)
        BUILD=1
        shift
        ;;
      --no-build)
        BUILD=0
        shift
        ;;
      -y | --yes)
        ASSUME_YES=1
        shift
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        die "Unknown argument: $1. Run ./run.sh --help for usage."
        ;;
    esac
  done
fi

ACTION="${ACTION:-start}"

case "$ACTION" in
  start | init | stop | logs)
    [[ -n "$MODE" ]] ||
      die "The $ACTION command requires --mode golang, sqlite, or legacy."
    ;;
esac

if [[ "$ACTION" == "init" && "$INTERACTIVE" == "0" && "$ASSUME_YES" == "0" ]]; then
  die "The init command deletes persistent data and requires --yes."
fi

require_docker

case "$ACTION" in
  start) start_stack ;;
  init) initialize_universe ;;
  stop) stop_stack ;;
  down) down_all ;;
  status) show_status ;;
  logs) follow_logs ;;
esac
