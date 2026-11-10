#!/usr/bin/env bash
# Equivalente de measure-cache.ps1 para equipos sin PowerShell 7 (VM Linux,
# Git Bash). Compara la ficha auto-001 con Redis caliente y con Redis detenido
# (fallback a MongoDB). Restaura Redis siempre. Guarda JSON sin secretos en
# output/cache-{warm,fallback,comparison}.json.
# Uso: bash scripts/measure-cache.sh [segundos por etapa, 3-30; predeterminado 5]
set -euo pipefail
export MSYS_NO_PATHCONV=1

cd "$(dirname "$0")/.."
seconds="${1:-5}"
if [[ ! "$seconds" =~ ^[0-9]+$ ]] || ((seconds < 3 || seconds > 30)); then
  echo "Duración entre 3 y 30 segundos" >&2
  exit 1
fi
mkdir -p output tmp
bin="tmp/cache-load$( [[ "${OS:-}" == Windows_NT ]] && echo .exe )"
go build -mod=readonly -o "$bin" ./cmd/load

route=/v1/vehicles/auto-001
metric() {
  # Suma un contador de caché de ambas réplicas de Flota; el token se expande
  # dentro del contenedor y no pasa por este proceso.
  local total=0 value
  for service in fleet-a fleet-b; do
    value=$(docker compose exec -T "$service" sh -c \
      'wget -q -O - -T 5 --header="Authorization: Bearer ${AUTH_SERVICE_TOKEN:?}" http://127.0.0.1:8080/internal/metrics' |
      awk -v key="fleet_cache_requests_total{result=\"$1\"}" '$1 == key {print $2}')
    total=$((total + ${value:-0}))
  done
  echo "$total"
}
counters() { printf '{"hit":%s,"miss":%s,"error":%s}' "$(metric hit)" "$(metric miss)" "$(metric error)"; }
phase() {
  local name=$1 before after
  before=$(counters)
  "$bin" -path "$route" -role client -concurrency 1,8 -duration "${seconds}s" > "output/cache-$name.json"
  after=$(counters)
  printf '{"name":"%s","countersBefore":%s,"countersAfter":%s}' "$name" "$before" "$after"
}
restore() {
  docker compose start redis >/dev/null
  for _ in $(seq 1 90); do
    [[ "$(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q redis)")" == healthy ]] && return 0
    sleep 0.5
  done
  echo "Redis no recuperó estado saludable" >&2
  return 1
}
trap restore EXIT

"$bin" -path "$route" -role client -concurrency 1 -duration 1s > /dev/null
warm=$(phase warm)
docker compose stop --timeout 5 redis >/dev/null
fallback=$(phase fallback)
restore
trap - EXIT

node -e '
const fs = require("fs");
const [warm, fallback] = process.argv.slice(1).map((p) => JSON.parse(p));
const stages = (name) => JSON.parse(fs.readFileSync(`output/cache-${name}.json`, "utf8"));
const summary = (phase) => {
  const load = stages(phase.name);
  const delta = Object.fromEntries(Object.keys(phase.countersAfter).map((k) => [k, phase.countersAfter[k] - phase.countersBefore[k]]));
  return { name: phase.name, revision: load.revision, workingTreeDirty: load.workingTreeDirty, cacheCounterDelta: delta,
    stages: load.stages.map((s) => ({ concurrency: s.concurrency, requests: s.requests, rps: +s.requestsPerSecond.toFixed(1), p50Ms: s.p50Ms, p95Ms: s.p95Ms, p99Ms: s.p99Ms, httpStatuses: s.httpStatuses, transportErrors: s.transportErrors })) };
};
const result = { measuredAt: new Date().toISOString(), route: "/v1/vehicles/auto-001", phases: [summary(warm), summary(fallback)], redisRestored: true };
fs.writeFileSync("output/cache-comparison.json", JSON.stringify(result, null, 2));
for (const p of result.phases) for (const s of p.stages) console.log(`${p.name.padEnd(8)} c=${String(s.concurrency).padEnd(2)} rps=${String(s.rps).padEnd(7)} p50=${s.p50Ms}ms p95=${s.p95Ms}ms p99=${s.p99Ms}ms`);
for (const p of result.phases) console.log(`${p.name.padEnd(8)} contadores de caché: ${JSON.stringify(p.cacheCounterDelta)}`);
' "$warm" "$fallback"
