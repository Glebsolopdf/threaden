#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
compose_file="$root_dir/backend/docker-compose.yml"

cleanup() {
  docker compose -f "$compose_file" down
}

trap cleanup EXIT INT TERM

docker compose -f "$compose_file" up --build --detach

for _ in {1..30}; do
  if curl --fail --silent http://localhost:8080/readyz >/dev/null; then
    break
  fi
  sleep 1
done

curl --fail --silent http://localhost:8080/readyz >/dev/null || {
  echo "Backend did not become ready; see: docker compose -f backend/docker-compose.yml logs" >&2
  exit 1
}

cd "$root_dir/web-client"
if [[ ! -d node_modules ]]; then
  npm install
fi

echo "Open http://127.0.0.1:4200"
npm run dev
