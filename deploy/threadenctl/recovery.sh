#!/usr/bin/env bash

clear_selected_caches() {
  log "clearing Threaden-owned caches only"
  if ((SELECT_BACKEND)); then
    rm -rf -- "$BUILD_CACHE/backend-src" "$BUILD_CACHE/go-cache" "$BUILD_CACHE/go-mod" "$BUILD_CACHE/threaden-backend.new"
    install -d -m 0750 -o threaden -g threaden "$BUILD_CACHE/go-cache" "$BUILD_CACHE/go-mod"
  fi
  if ((SELECT_WEB)); then
    rm -rf -- "$BUILD_CACHE/web-src" "$NGINX_CACHE"/*
    local dir
    for dir in client_temp proxy_temp fastcgi_temp uwsgi_temp scgi_temp; do
      install -d -m 0750 -o threaden -g threaden "$NGINX_CACHE/$dir"
    done
  fi
  ((SELECT_LIVEKIT)) && docker rm -f threaden-livekit >/dev/null 2>&1 || true
}

remove_selected_units() {
  local units=() unit
  mapfile -t units < <(selected_units)
  systemctl disable --now "${units[@]}" >/dev/null 2>&1 || true
  for unit in "${units[@]}"; do rm -f -- "$UNIT_DIR/$unit"; done
  systemctl daemon-reload
  systemctl reset-failed "${units[@]}" >/dev/null 2>&1 || true
}

kill_stale_threaden_processes() {
  log "removing stale Threaden-owned processes"
  local unit
  while IFS= read -r unit; do systemctl kill --kill-who=all --signal=SIGTERM "$unit" >/dev/null 2>&1 || true; done < <(selected_units)
  sleep 1
  while IFS= read -r unit; do systemctl kill --kill-who=all --signal=SIGKILL "$unit" >/dev/null 2>&1 || true; done < <(selected_units)
  if ((SELECT_BACKEND)) && [[ -x "$BACKEND_BIN" ]]; then
    local pid exe
    while read -r pid; do
      [[ -n "$pid" ]] || continue
      exe="$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)"
      [[ "$exe" == "$BACKEND_BIN" ]] && kill -TERM "$pid" 2>/dev/null || true
    done < <(pgrep -f "^$BACKEND_BIN( |$)" 2>/dev/null || true)
  fi
  if ((SELECT_WEB)) && [[ -r /run/threaden-web/nginx.pid ]]; then
    local pid="$(cat /run/threaden-web/nginx.pid 2>/dev/null || true)"
    if [[ "$pid" =~ ^[0-9]+$ ]] && tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null | grep -Fq "$CONFIG_DIR/nginx.conf"; then
      kill -QUIT "$pid" 2>/dev/null || true
    fi
  fi
  ((SELECT_LIVEKIT)) && docker rm -f threaden-livekit >/dev/null 2>&1 || true
}

repair_permissions() {
  log "repairing Threaden runtime permissions"
  ensure_threaden_user; ensure_directories
  chown threaden:threaden "$STATE_DIR"; chmod 0750 "$STATE_DIR"
  find "$STATE_DIR" -maxdepth 1 -type f -name 'app.db*' -exec chown threaden:threaden {} + -exec chmod 0600 {} + 2>/dev/null || true
  [[ -f "$ENV_FILE" ]] && { chown root:threaden "$ENV_FILE"; chmod 0600 "$ENV_FILE"; }
  [[ -f "$LIVEKIT_CONFIG" ]] && { chown root:threaden "$LIVEKIT_CONFIG"; chmod 0640 "$LIVEKIT_CONFIG"; }
  chmod 0755 "$SELF_PATH"
}

install_os_dependencies() {
  log "installing missing runtime dependencies where supported"
  if command_exists apt-get; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y ca-certificates curl rsync nginx util-linux procps iproute2
    command_exists docker || apt-get install -y docker.io
  elif command_exists dnf; then
    dnf install -y ca-certificates curl rsync nginx util-linux procps-ng iproute
    command_exists docker || { dnf install -y moby-engine || dnf install -y docker; }
  elif command_exists pacman; then
    pacman -Sy --noconfirm ca-certificates curl rsync nginx util-linux procps-ng iproute2
    command_exists docker || pacman -S --noconfirm docker
  else
    warn "unsupported package manager; install curl, rsync, nginx, Docker and util-linux manually"
  fi
  command_exists docker && systemctl enable --now docker.service >/dev/null 2>&1 || true
}

confirm_recovery() {
  ((ASSUME_YES)) && return 0
  printf 'Recreate Threaden services/caches for %s; preserve %s/app.db? [y/N] ' "$(selected_names)" "$STATE_DIR"
  local answer; read -r answer
  [[ "$answer" =~ ^[Yy]$ ]] || die "recovery cancelled"
}

show_status() {
  local units=(); mapfile -t units < <(selected_units)
  systemctl --no-pager --full status "${units[@]}" || true
  ((SELECT_LIVEKIT)) && command_exists docker && docker ps -a --filter name='^/threaden-livekit$' --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}' || true
}

show_logs() {
  local args=(-f -n 100) unit
  while IFS= read -r unit; do args+=(-u "$unit"); done < <(selected_units)
  exec journalctl "${args[@]}"
}
