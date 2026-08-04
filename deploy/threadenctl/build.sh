#!/usr/bin/env bash

ensure_threaden_user() {
  getent group threaden >/dev/null || groupadd --system threaden
  id threaden >/dev/null 2>&1 || useradd --system --gid threaden --home-dir "$STATE_DIR" --shell /usr/sbin/nologin threaden
}

ensure_directories() {
  install -d -m 0755 -o root -g root "$CONFIG_DIR" "$INSTALL_DIR"
  install -d -m 0750 -o threaden -g threaden "$STATE_DIR" "$BUILD_CACHE" "$NGINX_CACHE" /run/threaden-web /run/threaden-public-web
  install -d -m 0750 -o threaden -g threaden "$BUILD_CACHE/home" "$BUILD_CACHE/go-cache" "$BUILD_CACHE/go-mod"
  local dir
  for dir in client_temp proxy_temp fastcgi_temp uwsgi_temp scgi_temp; do
    install -d -m 0750 -o threaden -g threaden "$NGINX_CACHE/$dir"
  done
}

write_default_config() {
  [[ -e "$CONFIG_FILE" ]] && return 0
  cat >"$CONFIG_FILE" <<EOF_CONFIG
PROJECT_ROOT=$(printf '%q' "$PROJECT_ROOT")
BACKEND_BIND=$(printf '%q' "$BACKEND_BIND")
WEB_BIND=$(printf '%q' "$WEB_BIND")
PUBLIC_WEB_BIND=$(printf '%q' "$PUBLIC_WEB_BIND")
LIVEKIT_IMAGE=$(printf '%q' "$LIVEKIT_IMAGE")
GO_IMAGE=$(printf '%q' "$GO_IMAGE")
NODE_IMAGE=$(printf '%q' "$NODE_IMAGE")
HEALTH_TIMEOUT_SECONDS=$HEALTH_TIMEOUT_SECONDS
EOF_CONFIG
  chmod 0600 "$CONFIG_FILE"
  chown root:root "$CONFIG_FILE"
}

install_environment() {
  if ((SELECT_BACKEND || SELECT_LIVEKIT)); then
    install -m 0600 -o root -g threaden "$PROJECT_ENV" "$ENV_FILE"
  fi
  if ((SELECT_LIVEKIT)); then
    install -m 0640 -o root -g threaden "$PROJECT_LIVEKIT_CONFIG" "$LIVEKIT_CONFIG"
  fi
}

prepare_build_tree() {
  local source="$1" destination="$2"
  rm -rf -- "$destination"
  install -d -m 0750 -o threaden -g threaden "$destination"
  rsync -a --delete --exclude .git --exclude node_modules --exclude dist --exclude data "$source/" "$destination/"
  chown -R threaden:threaden "$destination"
}

build_backend() {
  log "building backend"
  local src="$BUILD_CACHE/backend-src" out="$BUILD_CACHE/threaden-backend.new"
  prepare_build_tree "$BACKEND_DIR" "$src"
  rm -f -- "$out"
  if go_version_ok; then
    runuser -u threaden -- env HOME="$BUILD_CACHE/home" GOCACHE="$BUILD_CACHE/go-cache" \
      GOMODCACHE="$BUILD_CACHE/go-mod" CGO_ENABLED=0 \
      go -C "$src" build -trimpath -ldflags='-s -w' -o "$out" ./cmd/api
  else
    log "using container build: $GO_IMAGE"
    docker run --rm --user "$(id -u threaden):$(id -g threaden)" -e HOME=/tmp -e CGO_ENABLED=0 \
      -v "$src:/src:ro" -v "$BUILD_CACHE:/out" -w /src "$GO_IMAGE" \
      go build -trimpath -ldflags='-s -w' -o /out/threaden-backend.new ./cmd/api
  fi
  [[ -s "$out" ]] || die "backend build produced no binary"
  install -m 0755 -o root -g root "$out" "$BACKEND_BIN.new"
  mv -f -- "$BACKEND_BIN.new" "$BACKEND_BIN"
}

build_web() {
  log "building web client"
  local src="$BUILD_CACHE/web-src" staged="$STATE_DIR/web.new" previous="$STATE_DIR/web.previous"
  prepare_build_tree "$WEB_DIR" "$src"
  if node_version_ok && command_exists npm; then
    runuser -u threaden -- env HOME="$BUILD_CACHE/home" npm --prefix "$src" install --no-audit --no-fund
    runuser -u threaden -- env HOME="$BUILD_CACHE/home" npm --prefix "$src" run build
  else
    log "using container build: $NODE_IMAGE"
    docker run --rm --user "$(id -u threaden):$(id -g threaden)" -e HOME=/tmp \
      -v "$src:/src" -w /src "$NODE_IMAGE" sh -ceu 'npm install --no-audit --no-fund && npm run build'
  fi
  [[ -f "$src/dist/index.html" ]] || die "web build did not produce dist/index.html"
  rm -rf -- "$staged" "$previous"
  install -d -m 0755 -o root -g root "$staged"
  rsync -a --delete "$src/dist/" "$staged/"
  chown -R root:root "$staged"
  find "$staged" -type d -exec chmod 0755 {} +
  find "$staged" -type f -exec chmod 0644 {} +
  [[ ! -d "$WEB_ROOT" ]] || mv -- "$WEB_ROOT" "$previous"
  mv -- "$staged" "$WEB_ROOT"
  rm -rf -- "$previous"
}

build_selected() {
  if ((SELECT_BACKEND)); then build_backend; fi
  if web_selected; then build_web; fi
}

prepare_installation() {
  ensure_threaden_user
  ensure_directories
  write_default_config
  install_environment
  build_selected
  install_units
}
