#!/usr/bin/env bash
# Ejecutar desde fuera de la VM para comprobar la entrada pública.
set -euo pipefail

base=${1:-}
if [[ ! $base =~ ^https://([A-Za-z0-9][A-Za-z0-9.-]+)$ ]]; then
  echo 'Uso: bash scripts/check-public.sh https://nombre-publico' >&2
  exit 2
fi
host=${BASH_REMATCH[1]}
if [[ $host == localhost || $host == 127.* || $host == *.local ]]; then
  echo 'Usar el nombre público desde una máquina externa a la VM' >&2
  exit 2
fi
command -v curl >/dev/null || { echo 'Falta curl' >&2; exit 2; }

status() {
  curl --silent --show-error --max-time 15 --output /dev/null \
    --write-out '%{http_code}' "$1"
}
expect_status() {
  local got
  got=$(status "$1")
  if [[ $got != "$2" ]]; then
    echo "FALLO: $1 devolvió $got; se esperaba $2" >&2
    exit 1
  fi
  echo "OK: $1 → $got"
}

expect_status "$base/" 200
expect_status "$base/operador/personal" 200
expect_status "$base/v1/employees" 401
expect_status "$base/internal/ready" 404

redirect=$(curl --silent --show-error --max-time 15 --output /dev/null \
  --write-out '%{redirect_url}' "http://$host/")
if [[ $redirect != "$base/" ]]; then
  echo "FALLO: HTTP redirigió a '$redirect' en vez de $base/" >&2
  exit 1
fi
echo 'OK: HTTP redirige a HTTPS'

if [[ -n ${AUTH_GYM_TOKEN:-} ]]; then
  command -v go >/dev/null || { echo 'Falta Go para verificar el contrato autenticado' >&2; exit 2; }
  go run ./cmd/checkshared -url "$base"
  echo 'OK: contrato autenticado y permisos'
else
  echo 'OMITIDO: contrato autenticado (definir AUTH_GYM_TOKEN en el entorno)'
fi
