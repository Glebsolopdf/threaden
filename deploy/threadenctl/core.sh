#!/usr/bin/env bash

readonly SCRIPT_VERSION="1.0.0"
readonly SELF_PATH="$(readlink -f -- "${BASH_SOURCE[1]}")"
readonly DEFAULT_ROOT="$(cd -- "$(dirname -- "$SELF_PATH")" && pwd -P)"
readonly CONFIG_DIR="/etc/threaden"
readonly CONFIG_FILE="$CONFIG_DIR/threadenctl.conf"
readonly ENV_FILE="$CONFIG_DIR/backend.env"
readonly LIVEKIT_CONFIG="$CONFIG_DIR/livekit.yaml"
readonly INSTALL_DIR="/usr/local/lib/threaden"
readonly BACKEND_BIN="$INSTALL_DIR/threaden-backend"
readonly STATE_DIR="/var/lib/threaden"
readonly WEB_ROOT="$STATE_DIR/web"
readonly BUILD_CACHE="/var/cache/threaden-build"
readonly NGINX_CACHE="/var/cache/threaden-nginx"
readonly UNIT_DIR="/etc/systemd/system"
readonly BACKEND_UNIT="threaden-backend.service"
readonly WEB_UNIT="threaden-web.service"
readonly PUBLIC_WEB_UNIT="threaden-public-web.service"
readonly LIVEKIT_UNIT="threaden-livekit.service"
readonly LOCK_FILE="/run/lock/threadenctl.lock"

PROJECT_ROOT="${THREADEN_ROOT:-$DEFAULT_ROOT}"
BACKEND_BIND="127.0.0.1:18080"
WEB_BIND="127.0.0.1:18081"
PUBLIC_WEB_BIND="127.0.0.1:18082"
LIVEKIT_IMAGE="livekit/livekit-server:v1.13.4"
GO_IMAGE="golang:1.26-alpine"
NODE_IMAGE="node:24-alpine"
MIN_GO_VERSION="1.26"
MIN_NODE_VERSION="24.0"
HEALTH_TIMEOUT_SECONDS="45"
ACTION=""
SELECT_BACKEND=0
SELECT_WEB=0
SELECT_PUBLIC=0
SELECT_LIVEKIT=0
EXPLICIT_SELECTION=0
ASSUME_YES=0
FULL_RESTART=0

log() { printf '[threadenctl] %s\n' "$*"; }
warn() { printf '[threadenctl] WARNING: %s\n' "$*" >&2; }
die() { printf '[threadenctl] ERROR: %s\n' "$*" >&2; exit 1; }
command_exists() { command -v "$1" >/dev/null 2>&1; }

load_config() {
  [[ -r "$CONFIG_FILE" ]] || return 0
  local uid mode
  uid="$(stat -c '%u' "$CONFIG_FILE" 2>/dev/null || printf unknown)"
  mode="$(stat -c '%a' "$CONFIG_FILE" 2>/dev/null || printf unknown)"
  [[ "$uid" == 0 && "$mode" =~ ^[0-7]+$ ]] || die "$CONFIG_FILE must be root-owned"
  (( (8#$mode & 8#022) == 0 )) || die "$CONFIG_FILE must not be group/world-writable"
  # shellcheck disable=SC1090
  source "$CONFIG_FILE"
}

refresh_paths() {
  PROJECT_ROOT="$(readlink -f -- "$PROJECT_ROOT" 2>/dev/null || printf '%s' "$PROJECT_ROOT")"
  BACKEND_DIR="$PROJECT_ROOT/backend"
  WEB_DIR="$PROJECT_ROOT/web-client"
  DEPLOY_DIR="$PROJECT_ROOT/deploy"
  PROJECT_ENV="$DEPLOY_DIR/production.env"
  PROJECT_LIVEKIT_CONFIG="$DEPLOY_DIR/livekit.yaml"
}

usage() {
  cat <<'HELP'
Threaden server manager

Usage: sudo ./threadenctl.sh <command> [components] [options]

Commands:
  start       Check, build/install and start.
  recovery    Recreate Threaden units/processes/caches, repair rights, start.
  restart     Stop and start selected services without rebuilding.
  stop        Stop selected services.  shutdown is an alias.
  status      Show service status.
  logs        Follow journal logs.
  doctor      Run checks without changes.
  install     Build/install without starting.

Components: --backend, --web, --public, --livekit, --all (default: backend + public + livekit)
Options:    --full, --root PATH, --yes, --version, --help

--public selects the separate public web service on port 18082.

Examples:
  sudo ./threadenctl.sh start
  sudo ./threadenctl.sh restart --backend
  sudo ./threadenctl.sh restart --full
  sudo ./threadenctl.sh restart --full --public
  sudo ./threadenctl.sh recovery --web --yes
  sudo ./threadenctl.sh stop --backend --web
HELP
}

parse_args() {
  (($#)) || { usage; exit 2; }
  ACTION="$1"; shift
  [[ "$ACTION" == shutdown ]] && ACTION=stop
  case "$ACTION" in
    -h|--help|help) usage; exit 0 ;;
    --version) printf '%s\n' "$SCRIPT_VERSION"; exit 0 ;;
  esac
  while (($#)); do
    case "$1" in
      --backend) SELECT_BACKEND=1; EXPLICIT_SELECTION=1 ;;
      --web) SELECT_WEB=1; EXPLICIT_SELECTION=1 ;;
      --public) SELECT_PUBLIC=1; EXPLICIT_SELECTION=1 ;;
      --livekit) SELECT_LIVEKIT=1; EXPLICIT_SELECTION=1 ;;
      --all) SELECT_BACKEND=1; SELECT_PUBLIC=1; SELECT_LIVEKIT=1; EXPLICIT_SELECTION=1 ;;
      --full) FULL_RESTART=1 ;;
      --root) shift; (($#)) || die "--root requires a path"; PROJECT_ROOT="$1" ;;
      --yes|-y) ASSUME_YES=1 ;;
      --version) printf '%s\n' "$SCRIPT_VERSION"; exit 0 ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown argument: $1" ;;
    esac
    shift
  done
  [[ $FULL_RESTART -eq 0 || $ACTION == restart ]] || die "--full is only valid with restart"
  if ((EXPLICIT_SELECTION == 0)); then
    SELECT_BACKEND=1; SELECT_PUBLIC=1; SELECT_LIVEKIT=1
  fi
  refresh_paths
}

selected_units() {
  if ((SELECT_BACKEND)); then printf '%s\n' "$BACKEND_UNIT"; fi
  if ((SELECT_WEB)); then printf '%s\n' "$WEB_UNIT"; fi
  if ((SELECT_PUBLIC)); then printf '%s\n' "$PUBLIC_WEB_UNIT"; fi
  if ((SELECT_LIVEKIT)); then printf '%s\n' "$LIVEKIT_UNIT"; fi
  return 0
}

selected_names() {
  local names=()
  ((SELECT_BACKEND)) && names+=(backend)
  ((SELECT_WEB)) && names+=(web)
  ((SELECT_PUBLIC)) && names+=(public)
  ((SELECT_LIVEKIT)) && names+=(livekit)
  local joined="${names[*]}"
  printf '%s' "${joined//$'\n'/, }"
}

web_selected() { ((SELECT_WEB || SELECT_PUBLIC)); }

require_root() { [[ ${EUID:-$(id -u)} -eq 0 ]] || die "run with sudo"; }
version_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

go_version_ok() {
  command_exists go || return 1
  local v="$(go version | sed -nE 's/.* go([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/p')"
  [[ -n "$v" ]] && version_ge "$v" "$MIN_GO_VERSION"
}

node_version_ok() {
  command_exists node || return 1
  local v="$(node --version | sed 's/^v//')"
  [[ -n "$v" ]] && version_ge "$v" "$MIN_NODE_VERSION"
}

docker_usable() { command_exists docker && docker info >/dev/null 2>&1; }
systemd_running() { [[ -d /run/systemd/system ]] && systemctl show --property=Version >/dev/null 2>&1; }

read_env_value() {
  awk -v key="$1" '$0 ~ "^[[:space:]]*" key "=" {sub("^[[:space:]]*" key "=", ""); sub("[[:space:]]+$", ""); print; exit}' "$2"
}

validate_env_file() {
  [[ -f "$PROJECT_ENV" && ! -L "$PROJECT_ENV" ]] || die "missing regular file $PROJECT_ENV"
  grep -Eqi 'replace-with|(^|[./])example\.(com|org|net)([:/]|$)' "$PROJECT_ENV" && die "$PROJECT_ENV contains placeholders"
  local key value
  for key in LIVEKIT_PUBLIC_URL LIVEKIT_API_KEY LIVEKIT_API_SECRET SESSION_TTL SESSION_IDLE_TTL CORS_ALLOWED_ORIGINS; do
    value="$(read_env_value "$key" "$PROJECT_ENV")"
    [[ -n "$value" ]] || die "$PROJECT_ENV: missing $key"
  done
  [[ "$(read_env_value SESSION_COOKIE_SECURE "$PROJECT_ENV")" == true ]] || warn "SESSION_COOKIE_SECURE is not true"
}

validate_livekit_config() {
  [[ -f "$PROJECT_LIVEKIT_CONFIG" && ! -L "$PROJECT_LIVEKIT_CONFIG" ]] || die "missing regular file $PROJECT_LIVEKIT_CONFIG"
  grep -Eqi 'replace-with|(^|[./])example\.(com|org|net)([:/]|$)' "$PROJECT_LIVEKIT_CONFIG" && die "$PROJECT_LIVEKIT_CONFIG contains placeholders"
  local key secret yaml
  key="$(read_env_value LIVEKIT_API_KEY "$PROJECT_ENV")"
  secret="$(read_env_value LIVEKIT_API_SECRET "$PROJECT_ENV")"
  yaml="$(<"$PROJECT_LIVEKIT_CONFIG")"
  [[ "$yaml" == *"$key"* ]] || die "LiveKit YAML key differs from production.env"
  [[ "$yaml" == *"$secret"* ]] || die "LiveKit YAML secret differs from production.env"
  if grep -Eq '^[[:space:]]*enabled:[[:space:]]*true' "$PROJECT_LIVEKIT_CONFIG"; then
    local cert private_key
    cert="$(awk '$1 == "cert_file:" {gsub(/"/, "", $2); print $2; exit}' "$PROJECT_LIVEKIT_CONFIG")"
    private_key="$(awk '$1 == "key_file:" {gsub(/"/, "", $2); print $2; exit}' "$PROJECT_LIVEKIT_CONFIG")"
    [[ -z "$cert" || -r "$cert" ]] || die "LiveKit certificate is missing: $cert"
    [[ -z "$private_key" || -r "$private_key" ]] || die "LiveKit key is missing: $private_key"
  fi
}

preflight_minimal() {
  [[ "$(uname -s)" == Linux ]] || die "Linux is required"
  systemd_running || die "systemd is not the active system manager"
  command_exists systemctl || die "missing systemctl"
  command_exists flock || die "missing flock"
}

preflight_selected() {
  preflight_minimal
  local cmd
  for cmd in systemd-analyze curl rsync install awk sed grep stat sort timeout runuser getent groupadd useradd; do
    command_exists "$cmd" || die "missing command: $cmd"
  done
  [[ -d "$PROJECT_ROOT" ]] || die "project root not found: $PROJECT_ROOT"
  if ((SELECT_BACKEND)); then
    [[ -f "$BACKEND_DIR/go.mod" && -f "$BACKEND_DIR/cmd/api/main.go" ]] || die "backend source is incomplete"
    validate_env_file
    go_version_ok || docker_usable || die "need Go >= $MIN_GO_VERSION or Docker"
  fi
  if web_selected; then
    [[ -f "$WEB_DIR/package.json" && -f "$WEB_DIR/package-lock.json" ]] || die "web source is incomplete"
    command_exists nginx || die "nginx is required"
    { node_version_ok && command_exists npm; } || docker_usable || die "need Node >= $MIN_NODE_VERSION or Docker"
  fi
  if ((SELECT_LIVEKIT)); then
    docker_usable || die "LiveKit requires Docker"
    validate_env_file
    validate_livekit_config
  fi
}
