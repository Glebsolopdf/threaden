#!/usr/bin/env bash

PUBLIC_WEB_CONFIG="$CONFIG_DIR/nginx-public.conf"

render_public_nginx_config() {
  cat >"$PUBLIC_WEB_CONFIG" <<EOF_NGINX
worker_processes auto;
pid /run/threaden-public-web/nginx.pid;
error_log stderr notice;
events { worker_connections 1024; }
http {
  include /etc/nginx/mime.types;
  default_type application/octet-stream;
  types { application/manifest+json webmanifest; }
  access_log off;
  sendfile on;
  keepalive_timeout 65;
  server_tokens off;
  client_body_temp_path $NGINX_CACHE/client_temp;
  proxy_temp_path $NGINX_CACHE/proxy_temp;
  fastcgi_temp_path $NGINX_CACHE/fastcgi_temp;
  uwsgi_temp_path $NGINX_CACHE/uwsgi_temp;
  scgi_temp_path $NGINX_CACHE/scgi_temp;
  limit_req_zone \$binary_remote_addr zone=threaden_public_api:10m rate=10r/s;
  limit_req_zone \$binary_remote_addr zone=threaden_public_auth:10m rate=2r/s;
  limit_conn_zone \$binary_remote_addr zone=threaden_public_connections:10m;
  server {
    listen $PUBLIC_WEB_BIND;
    server_name _;
    root $WEB_ROOT;
    index index.html;
    client_max_body_size 10m;
    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy strict-origin-when-cross-origin always;
    add_header X-Frame-Options DENY always;
    location ~ ^/v1/auth/(register|login)\$ {
      limit_req zone=threaden_public_auth burst=5 nodelay;
      proxy_pass http://$BACKEND_BIND;
      proxy_http_version 1.1;
      proxy_set_header Host \$host;
      proxy_set_header X-Real-IP \$remote_addr;
      proxy_set_header X-Forwarded-For \$remote_addr;
      proxy_set_header X-Forwarded-Proto \$scheme;
    }
    location /v1/ {
      limit_conn threaden_public_connections 20;
      limit_req zone=threaden_public_api burst=40 nodelay;
      proxy_pass http://$BACKEND_BIND;
      proxy_http_version 1.1;
      proxy_buffering off;
      proxy_read_timeout 3600s;
      proxy_send_timeout 3600s;
      proxy_set_header Host \$host;
      proxy_set_header X-Real-IP \$remote_addr;
      proxy_set_header X-Forwarded-For \$remote_addr;
      proxy_set_header X-Forwarded-Proto \$scheme;
    }
    location = /healthz { proxy_pass http://$BACKEND_BIND; }
    location = /readyz { proxy_pass http://$BACKEND_BIND; }
    location = /index.html {
      add_header Cache-Control "no-store, no-cache, must-revalidate" always;
      add_header Pragma "no-cache" always;
      add_header Expires "0" always;
      try_files \$uri =404;
    }
    location = /runtime-config.js {
      add_header Cache-Control "no-store, no-cache, must-revalidate" always;
      add_header Pragma "no-cache" always;
      add_header Expires "0" always;
      try_files \$uri =404;
    }
    location = /manifest.webmanifest {
      add_header Cache-Control "no-cache, must-revalidate" always;
      try_files \$uri =404;
    }
    location ~* "-[A-Za-z0-9]{8,}\.(?:js|css)$" {
      add_header Cache-Control "public, max-age=31536000, immutable" always;
      try_files \$uri =404;
    }
    location / { try_files \$uri \$uri/ /index.html; }
  }
}
EOF_NGINX
  chmod 0640 "$PUBLIC_WEB_CONFIG"
  chown root:threaden "$PUBLIC_WEB_CONFIG"
}

render_public_web_unit() {
  local nginx_bin="$(command -v nginx)"
  cat >"$UNIT_DIR/$PUBLIC_WEB_UNIT" <<EOF_UNIT
[Unit]
Description=Threaden public static web server
After=network-online.target $BACKEND_UNIT
Wants=network-online.target
[Service]
Type=simple
User=threaden
Group=threaden
ExecStartPre=$nginx_bin -t -q -c $PUBLIC_WEB_CONFIG
ExecStart=$nginx_bin -c $PUBLIC_WEB_CONFIG -g "daemon off;"
ExecReload=$nginx_bin -s reload -c $PUBLIC_WEB_CONFIG
KillSignal=SIGQUIT
TimeoutStopSec=20s
Restart=on-failure
RestartSec=3s
UMask=0027
RuntimeDirectory=threaden-public-web
RuntimeDirectoryMode=0750
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectHostname=yes
ProtectProc=invisible
ProcSubset=pid
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
ReadWritePaths=$NGINX_CACHE /run/threaden-public-web
ReadOnlyPaths=$WEB_ROOT $PUBLIC_WEB_CONFIG /etc/nginx/mime.types
[Install]
WantedBy=multi-user.target
EOF_UNIT
}
