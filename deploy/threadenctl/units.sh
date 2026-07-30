#!/usr/bin/env bash

render_nginx_config() {
  cat >"$CONFIG_DIR/nginx.conf" <<EOF_NGINX
worker_processes auto;
pid /run/threaden-web/nginx.pid;
error_log stderr notice;
events { worker_connections 1024; }
http {
  include /etc/nginx/mime.types;
  default_type application/octet-stream;
  access_log off;
  sendfile on;
  keepalive_timeout 65;
  server_tokens off;
  client_body_temp_path $NGINX_CACHE/client_temp;
  proxy_temp_path $NGINX_CACHE/proxy_temp;
  fastcgi_temp_path $NGINX_CACHE/fastcgi_temp;
  uwsgi_temp_path $NGINX_CACHE/uwsgi_temp;
  scgi_temp_path $NGINX_CACHE/scgi_temp;
  limit_req_zone \$binary_remote_addr zone=threaden_api:10m rate=10r/s;
  limit_req_zone \$binary_remote_addr zone=threaden_auth:10m rate=2r/s;
  limit_conn_zone \$binary_remote_addr zone=threaden_connections:10m;
  server {
    listen $WEB_BIND;
    server_name _;
    root $WEB_ROOT;
    index index.html;
    client_max_body_size 10m;
    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy strict-origin-when-cross-origin always;
    add_header X-Frame-Options DENY always;
    location ~ ^/v1/auth/(register|login)\$ {
      limit_req zone=threaden_auth burst=5 nodelay;
      proxy_pass http://$BACKEND_BIND;
      proxy_http_version 1.1;
      proxy_set_header Host \$host;
      proxy_set_header X-Real-IP \$remote_addr;
      proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto \$scheme;
    }
    location /v1/ {
      limit_conn threaden_connections 20;
      limit_req zone=threaden_api burst=40 nodelay;
      proxy_pass http://$BACKEND_BIND;
      proxy_http_version 1.1;
      proxy_buffering off;
      proxy_read_timeout 3600s;
      proxy_send_timeout 3600s;
      proxy_set_header Host \$host;
      proxy_set_header X-Real-IP \$remote_addr;
      proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto \$scheme;
    }
    location = /healthz { proxy_pass http://$BACKEND_BIND; }
    location = /readyz { proxy_pass http://$BACKEND_BIND; }
    location / { try_files \$uri \$uri/ /index.html; }
  }
}
EOF_NGINX
  chmod 0640 "$CONFIG_DIR/nginx.conf"
  chown root:threaden "$CONFIG_DIR/nginx.conf"
}

render_backend_unit() {
  cat >"$UNIT_DIR/$BACKEND_UNIT" <<EOF_UNIT
[Unit]
Description=Threaden Go API
After=network-online.target $LIVEKIT_UNIT
Wants=network-online.target
[Service]
Type=simple
User=threaden
Group=threaden
WorkingDirectory=$STATE_DIR
EnvironmentFile=$ENV_FILE
Environment=HTTP_ADDR=$BACKEND_BIND
Environment=DATABASE_PATH=$STATE_DIR/app.db
ExecStart=$BACKEND_BIN
Restart=on-failure
RestartSec=3s
TimeoutStopSec=25s
KillSignal=SIGTERM
UMask=0077
LimitNOFILE=65536
StateDirectory=threaden
RuntimeDirectory=threaden-backend
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectHostname=yes
ProtectProc=invisible
ProcSubset=pid
RemoveIPC=yes
ProtectSystem=strict
ProtectHome=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
ProtectClock=yes
RestrictSUIDSGID=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
CapabilityBoundingSet=
AmbientCapabilities=
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
SystemCallArchitectures=native
ReadWritePaths=$STATE_DIR
[Install]
WantedBy=multi-user.target
EOF_UNIT
}

render_web_unit() {
  local nginx_bin="$(command -v nginx)"
  cat >"$UNIT_DIR/$WEB_UNIT" <<EOF_UNIT
[Unit]
Description=Threaden static web server
After=network-online.target $BACKEND_UNIT
Wants=network-online.target
[Service]
Type=simple
User=threaden
Group=threaden
ExecStartPre=$nginx_bin -t -q -c $CONFIG_DIR/nginx.conf
ExecStart=$nginx_bin -c $CONFIG_DIR/nginx.conf -g "daemon off;"
ExecReload=$nginx_bin -s reload -c $CONFIG_DIR/nginx.conf
KillSignal=SIGQUIT
TimeoutStopSec=20s
Restart=on-failure
RestartSec=3s
UMask=0027
RuntimeDirectory=threaden-web
RuntimeDirectoryMode=0750
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectHostname=yes
ProtectProc=invisible
ProcSubset=pid
RemoveIPC=yes
ProtectSystem=strict
ProtectHome=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
ProtectClock=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
SystemCallArchitectures=native
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
LimitNOFILE=65536
ReadWritePaths=$NGINX_CACHE /run/threaden-web
ReadOnlyPaths=$WEB_ROOT $CONFIG_DIR/nginx.conf /etc/nginx/mime.types
[Install]
WantedBy=multi-user.target
EOF_UNIT
}

render_livekit_unit() {
  local docker_bin="$(command -v docker)"
  cat >"$UNIT_DIR/$LIVEKIT_UNIT" <<EOF_UNIT
[Unit]
Description=Threaden LiveKit server container
After=network-online.target docker.service
Wants=network-online.target docker.service
[Service]
Type=simple
ExecStartPre=-$docker_bin rm -f threaden-livekit
ExecStart=$docker_bin run --name threaden-livekit --network host -v $LIVEKIT_CONFIG:/etc/livekit.yaml:ro -v /etc/letsencrypt:/etc/letsencrypt:ro $LIVEKIT_IMAGE --config /etc/livekit.yaml
ExecStop=-$docker_bin stop -t 20 threaden-livekit
ExecStopPost=-$docker_bin rm -f threaden-livekit
Restart=on-failure
RestartSec=5s
TimeoutStartSec=120s
TimeoutStopSec=30s
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=full
[Install]
WantedBy=multi-user.target
EOF_UNIT
}

install_units() {
  log "installing systemd units"
  ((SELECT_BACKEND)) && render_backend_unit
  ((SELECT_WEB)) && { render_nginx_config; render_web_unit; }
  ((SELECT_LIVEKIT)) && render_livekit_unit
  local units=() paths=() unit
  mapfile -t units < <(selected_units)
  for unit in "${units[@]}"; do paths+=("$UNIT_DIR/$unit"); done
  systemd-analyze verify "${paths[@]}" >/dev/null
  ((SELECT_WEB)) && runuser -u threaden -- nginx -t -q -c "$CONFIG_DIR/nginx.conf"
  systemctl daemon-reload
  systemctl reset-failed "${units[@]}" >/dev/null 2>&1 || true
  systemctl enable "${units[@]}" >/dev/null
}

port_from_bind() {
  local port="${1##*:}"; port="${port%]}"
  [[ "$port" =~ ^[0-9]+$ ]] || die "unsupported bind: $1"
  printf '%s' "$port"
}

wait_http() {
  local unit="$1" url="$2" deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  until curl --fail --silent --max-time 3 "$url" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || { journalctl --no-pager -n 80 -u "$unit" || true; die "$unit failed health check: $url"; }
    sleep 1
  done
}

wait_tcp() {
  local unit="$1" host="$2" port="$3" deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  until timeout 2 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || { journalctl --no-pager -n 80 -u "$unit" || true; die "$unit did not open $host:$port"; }
    sleep 1
  done
}

start_selected() {
  log "starting: $(selected_names)"
  if ((SELECT_LIVEKIT)); then
    local livekit_port="$(awk '$1 == "port:" {print $2; exit}' "$PROJECT_LIVEKIT_CONFIG")"
    [[ "$livekit_port" =~ ^[0-9]+$ ]] || die "invalid LiveKit port"
    systemctl start "$LIVEKIT_UNIT"; wait_tcp "$LIVEKIT_UNIT" 127.0.0.1 "$livekit_port"
  fi
  ((SELECT_BACKEND)) && { systemctl start "$BACKEND_UNIT"; wait_http "$BACKEND_UNIT" "http://127.0.0.1:$(port_from_bind "$BACKEND_BIND")/readyz"; }
  ((SELECT_WEB)) && { systemctl start "$WEB_UNIT"; wait_http "$WEB_UNIT" "http://127.0.0.1:$(port_from_bind "$WEB_BIND")/"; }
  log "healthy: $(selected_names)"
}

stop_selected() {
  log "stopping: $(selected_names)"
  ((SELECT_WEB)) && systemctl stop "$WEB_UNIT" || true
  ((SELECT_BACKEND)) && systemctl stop "$BACKEND_UNIT" || true
  ((SELECT_LIVEKIT)) && systemctl stop "$LIVEKIT_UNIT" || true
}
