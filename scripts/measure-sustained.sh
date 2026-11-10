#!/usr/bin/env bash
# Mide caché fría/caliente/fallback y frescura de Solr bajo carga local.
set -euo pipefail
export MSYS_NO_PATHCONV=1

cd "$(dirname "$0")/.."
seconds=${1:-30}
concurrency=${2:-8}
if [[ ! $seconds =~ ^[0-9]+$ ]] || ((seconds < 10 || seconds > 30)); then
  echo "Duración por fase entre 10 y 30 segundos" >&2
  exit 2
fi
if [[ ! $concurrency =~ ^[0-9]+$ ]] || ((concurrency < 1 || concurrency > 64)); then
  echo "Concurrencia entre 1 y 64" >&2
  exit 2
fi
for command in docker go node; do
  command -v "$command" >/dev/null || { echo "Falta $command" >&2; exit 2; }
done
[[ -f .env ]] || { echo "Falta .env; ejecutar go run ./cmd/devsetup" >&2; exit 2; }

mkdir -p output tmp
suffix=$( [[ "${OS:-}" == Windows_NT ]] && echo .exe )
bin="tmp/sustained-load$suffix"
smoke="tmp/sustained-smoke$suffix"
go build -mod=readonly -o "$bin" ./cmd/load
# Compilar antes de iniciar la carga: con caché fría, `go run` consumiría la ventana de búsqueda.
go build -mod=readonly -o "$smoke" ./cmd/smoke
route=/v1/vehicles/auto-001
redis_restored=false
search_pid=

restore() {
  local status=$?
  if [[ -n ${search_pid:-} ]]; then
    kill "$search_pid" 2>/dev/null || true
    wait "$search_pid" 2>/dev/null || true
  fi
  docker compose start redis >/dev/null 2>&1 || true
  for _ in $(seq 1 90); do
    if [[ $(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q redis)" 2>/dev/null || true) == healthy ]]; then
      redis_restored=true
      break
    fi
    sleep 0.5
  done
  if [[ $redis_restored != true ]]; then
    echo "Redis no recuperó estado saludable" >&2
    return 1
  fi
  return "$status"
}
trap restore EXIT

metric() {
  local key=$1 total=0 value service
  for service in fleet-a fleet-b; do
    value=$(docker compose exec -T "$service" sh -c \
      'wget -q -O - -T 5 --header="Authorization: Bearer ${AUTH_SERVICE_TOKEN:?}" http://127.0.0.1:8080/internal/metrics' |
      awk -v key="$key" '$1 == key {print $2}')
    total=$((total + ${value:-0}))
  done
  echo "$total"
}
counters() {
  printf '{"hit":%s,"miss":%s,"error":%s,"bypass":%s,"sourceDetail":%s}' \
    "$(metric 'fleet_cache_requests_total{result="hit"}')" \
    "$(metric 'fleet_cache_requests_total{result="miss"}')" \
    "$(metric 'fleet_cache_requests_total{result="error"}')" \
    "$(metric 'fleet_cache_requests_total{result="bypass"}')" \
    "$(metric 'fleet_source_reads_total{operation="detail"}')"
}
phase() {
  local name=$1 before after
  before=$(counters)
  "$bin" -path "$route" -role client -concurrency "$concurrency" -duration "${seconds}s" > "output/sustained-cache-$name.json"
  after=$(counters)
  printf '{"name":"%s","countersBefore":%s,"countersAfter":%s}\n' "$name" "$before" "$after" > "output/sustained-cache-$name-metrics.json"
}

# Misma clave que cmd/fleet/main.go: fleet:v1:<base>:vehicle:{id}:data. La marca de generación se conserva.
cache_key="fleet:v1:${FLEET_CACHE_DATABASE:-fleet}:vehicle:{auto-001}:data"
redis_cli() {
  docker compose exec -T -e CACHE_KEY="$cache_key" redis sh -c \
    "redis-cli --no-auth-warning -a \"\${REDIS_PASSWORD:?}\" $1 \"\$CACHE_KEY\""
}
# Borra la ficha de la caché y la vuelve a cargar con una lectura; falla si la clave no es la real,
# para que un cambio de nombre no convierta la fase fría en una medición caliente sin aviso.
reset_entry() {
  redis_cli DEL >/dev/null
  "$bin" -path "$route" -role client -concurrency 1 -duration 1s >/dev/null
  if [[ $(redis_cli EXISTS | tr -d '\r') != 1 ]]; then
    echo "La entrada $cache_key no existe tras leer la ficha" >&2
    exit 1
  fi
}

reset_entry
redis_cli DEL >/dev/null
phase cold

# El TTL de la ficha es 60 s y los aciertos no lo renuevan: con 30 s por fase, la entrada creada en la
# fase fría expiraría durante la caliente. Se renueva justo antes para que la fase mida solo aciertos.
reset_entry
phase warm

docker compose stop --timeout 5 redis >/dev/null
phase fallback
docker compose start redis >/dev/null
for _ in $(seq 1 90); do
  if [[ $(docker inspect --format '{{.State.Health.Status}}' "$(docker compose ps -q redis)" 2>/dev/null || true) == healthy ]]; then
    redis_restored=true
    break
  fi
  sleep 0.5
done
[[ $redis_restored == true ]] || { echo "Redis no recuperó estado saludable" >&2; exit 1; }

# Mantiene tráfico de búsqueda mientras smoke mide alta, actualización y baja.
# cmd/load admite hasta 30 s por ejecución: se encadenan ventanas contiguas hasta que el recorrido termine,
# de modo que un runner lento no deje el recorrido sin carga. La última ventana en curso se completa.
rm -f output/sustained-search*.json tmp/sustained-search-stop
search_loop() {
  local window=0
  while [[ ! -e tmp/sustained-search-stop ]]; do
    window=$((window + 1))
    "$bin" -path '/v1/vehicles/search?page=1&pageSize=20&sort=price_asc' -role client       -concurrency "$concurrency" -duration 30s > "output/sustained-search-$window.json" || return 1
  done
}
search_loop &
search_pid=$!
sleep 1
"$smoke" -history | tee output/sustained-smoke.log
touch tmp/sustained-search-stop
wait "$search_pid" || { echo "La carga de búsqueda falló durante el recorrido de frescura" >&2; exit 1; }
search_pid=

node - "$seconds" "$concurrency" <<'NODE'
const fs = require("fs");
const [seconds, concurrency] = process.argv.slice(2).map(Number);
const names = ["cold", "warm", "fallback"];
const phases = names.map((name) => {
  const load = JSON.parse(fs.readFileSync(`output/sustained-cache-${name}.json`, "utf8"));
  const metrics = JSON.parse(fs.readFileSync(`output/sustained-cache-${name}-metrics.json`, "utf8"));
  const delta = Object.fromEntries(Object.keys(metrics.countersAfter).map((key) => [key, metrics.countersAfter[key] - metrics.countersBefore[key]]));
  return {name, cacheCounterDelta: delta, stage: load.stages[0], revision: load.revision, workingTreeDirty: load.workingTreeDirty};
});
const searchStages = fs.readdirSync("output").filter((name) => /^sustained-search-\d+\.json$/u.test(name))
  .sort((x, y) => parseInt(x.match(/\d+/u)[0], 10) - parseInt(y.match(/\d+/u)[0], 10))
  .flatMap((name) => JSON.parse(fs.readFileSync(`output/${name}`, "utf8")).stages);
const search = {stages: searchStages};
if (searchStages.length < 1) throw new Error("no hay ventanas de carga de búsqueda");
const smoke = fs.readFileSync("output/sustained-smoke.log", "utf8");
const freshness = [...smoke.matchAll(/PASS índice transición=(\S+) versión=(\d+) cotaDesdeEscrituraMs=(\d+)/gu)]
  .map((match) => ({transition: match[1], version: Number(match[2]), upperBoundFromWriteMs: Number(match[3])}));
const validStage = (stage) => stage.transportErrors === 0 && stage.invalidJSON === 0 && Object.keys(stage.httpStatuses).every((status) => status === "200");
console.log("diagnóstico", JSON.stringify({phases: phases.map((p) => ({name: p.name, delta: p.cacheCounterDelta, statuses: p.stage.httpStatuses, transportErrors: p.stage.transportErrors, invalidJSON: p.stage.invalidJSON})), searchStatuses: search.stages.map((s) => s.httpStatuses), freshness}));
for (const phase of phases) if (!validStage(phase.stage)) throw new Error(`errores durante fase ${phase.name}`);
if (phases[0].cacheCounterDelta.miss < 1 || phases[0].cacheCounterDelta.sourceDetail < 1) throw new Error("la fase fría no leyó la fuente");
if (phases[1].cacheCounterDelta.hit < 1 || phases[1].cacheCounterDelta.sourceDetail !== 0) throw new Error("la fase caliente no quedó servida por caché");
if (phases[2].cacheCounterDelta.sourceDetail < 1 || phases[2].cacheCounterDelta.error + phases[2].cacheCounterDelta.bypass < 1) throw new Error("el fallback no ejercitó Redis caído y MongoDB");
if (!search.stages.every(validStage)) throw new Error("la carga de búsqueda registró errores");
if (freshness.length !== 3 || freshness.some((sample) => sample.upperBoundFromWriteMs >= 5000)) throw new Error("frescura incompleta o fuera de 5 segundos");
const result = {measuredAt: new Date().toISOString(), method: "closed-loop GET", requestedPhaseSeconds: seconds, concurrency,
  cachePhases: phases, searchLoad: search.stages, searchFreshness: freshness, redisRestored: true};
fs.writeFileSync("output/sustained-summary.json", JSON.stringify(result, null, 2));
for (const phase of phases) console.log(`${phase.name.padEnd(8)} rps=${phase.stage.successfulRequestsPerSecond.toFixed(1)} p95=${phase.stage.p95Ms}ms source=${phase.cacheCounterDelta.sourceDetail}`);
for (const sample of freshness) console.log(`Solr ${sample.transition}: < ${sample.upperBoundFromWriteMs} ms desde escritura`);
NODE

trap - EXIT
echo "PASS: resultados en output/sustained-summary.json"
