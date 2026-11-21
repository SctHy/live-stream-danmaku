#!/bin/bash
# Run the gateway and worker natively, using the project's Compose containers.
set -euo pipefail
cd "$(dirname "$0")/.."
project="${COMPOSE_PROJECT_NAME:-live-stream-danmaku}"
if [[ -z "${DOCKER_CONTEXT:-}${DOCKER_HOST:-}" ]]; then
  # Prefer the context already hosting this project, including named Colima profiles.
  fallback=""
  while read -r candidate endpoint; do
    [[ "$endpoint" == unix://* ]] || continue
    if ! ids="$(docker --context "$candidate" ps -q --filter "label=com.docker.compose.project=$project" 2>/dev/null)"; then
      continue
    fi
    if [[ -n "$ids" ]]; then
      export DOCKER_CONTEXT="$candidate"
      break
    fi
    [[ -n "$fallback" ]] || fallback="$candidate"
  done < <(docker context ls --format '{{.Name}} {{.DockerEndpoint}}')
  if [[ -z "${DOCKER_CONTEXT:-}" ]]; then
    [[ -n "$fallback" ]] || { echo '请先启动 Colima 或 Docker Desktop。' >&2; exit 1; }
    export DOCKER_CONTEXT="$fallback"
  fi
fi
compose=(docker compose -p "$project")
for service in redis postgres kafka; do
  if [[ -z "$("${compose[@]}" ps --status running -q "$service")" ]]; then
    export REDIS_PORT="${REDIS_PORT:-26379}"
    "${compose[@]}" up -d "$service"
  fi
done
export REDIS_ADDR="$("${compose[@]}" port redis 6379)"
export KAFKA_BROKERS="$("${compose[@]}" port kafka 9092)"
pg="$("${compose[@]}" port postgres 5432)"
export DATABASE_URL="${DATABASE_URL:-postgres://danmaku:danmaku@${pg}/danmaku_db?sslmode=disable}"
port="${PORT:-8080}"
tmp="$(mktemp -d)"
children=()
cleanup(){
  trap - EXIT INT TERM
  for pid in "${children[@]:-}"; do [[ -z "$pid" ]] || kill -TERM "$pid" 2>/dev/null || true; done
  wait || true
  rm -f "$tmp/server" "$tmp/consumer"
  rmdir "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT TERM
go build -o "$tmp/server" ./cmd/server
go build -o "$tmp/consumer" ./cmd/consumer
available=false
for _ in {1..60}; do
  if "${compose[@]}" exec -T postgres pg_isready -q -U danmaku && "${compose[@]}" exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092 --list >/dev/null 2>&1; then available=true; break; fi
  sleep 1
done
[[ "$available" == true ]] || { echo "PostgreSQL 或 Kafka 未就绪" >&2; exit 1; }
"$tmp/server" -port "$port" & children+=("$!")
"$tmp/consumer" & children+=("$!")
url="http://127.0.0.1:$port"
for _ in {1..50}; do
  for pid in "${children[@]}"; do kill -0 "$pid" || exit 1; done
  if curl -fsS "$url/readyz" >/dev/null 2>&1; then break; fi
  sleep .2
done
curl -fsS "$url/readyz" >/dev/null
echo "$url"
if [[ "${1:-}" == "--open" ]]; then
  if command -v ardia >/dev/null; then ardia browse open "$url"; else open "$url"; fi
fi
while kill -0 "${children[0]}" 2>/dev/null && kill -0 "${children[1]}" 2>/dev/null; do sleep 1; done
exit 1
