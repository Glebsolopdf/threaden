#!/usr/bin/env bash
set -Eeuo pipefail
IFS=$'\n\t'

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=deploy/threadenctl/core.sh
source "$ROOT_DIR/deploy/threadenctl/core.sh"
load_config
refresh_paths
# shellcheck source=deploy/threadenctl/build.sh
source "$ROOT_DIR/deploy/threadenctl/build.sh"
# shellcheck source=deploy/threadenctl/units.sh
source "$ROOT_DIR/deploy/threadenctl/units.sh"
# shellcheck source=deploy/threadenctl/recovery.sh
source "$ROOT_DIR/deploy/threadenctl/recovery.sh"

on_error() {
  local code=$?
  printf '[threadenctl] FAILED at line %s (exit %s).\n' "${BASH_LINENO[0]:-?}" "$code" >&2
  exit "$code"
}

main() {
  trap on_error ERR
  parse_args "$@"
  require_root
  mkdir -p -- "$(dirname "$LOCK_FILE")"
  chmod 0755 "$(dirname "$LOCK_FILE")"
  exec 9>"$LOCK_FILE"
  flock -n 9 || die "another threadenctl operation is running"

  case "$ACTION" in
    doctor) preflight_selected; log "all checks passed for: $(selected_names)" ;;
    install) preflight_selected; prepare_installation; log "installed: $(selected_names)" ;;
    start) preflight_selected; prepare_installation; start_selected ;;
    stop) preflight_minimal; stop_selected ;;
    restart)
      preflight_selected; ensure_threaden_user; ensure_directories; stop_selected
      clear_selected_caches; install_environment; build_selected; install_units; start_selected
      ;;
    recovery)
      confirm_recovery; install_os_dependencies; preflight_selected; stop_selected
      remove_selected_units; kill_stale_threaden_processes; ensure_threaden_user; ensure_directories
      clear_selected_caches; rm -f -- "$BACKEND_BIN.new" "$STATE_DIR/web.new"
      repair_permissions; write_default_config; install_environment; build_selected; install_units; start_selected
      ;;
    status) preflight_minimal; show_status ;;
    logs) preflight_minimal; flock -u 9; show_logs ;;
    *) usage; die "unknown command: $ACTION" ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
