#!/usr/bin/env bash
# Diagnóstico de solo lectura del Hospital por la entrada pública:
# React -> gateway -> Empleados -> hospital. Ejecuta únicamente GET de médicos
# y facturación con el token administrativo; no crea turnos ni cargos y no
# imprime credenciales. Uso, en la VM y desde la raíz del repositorio:
#   bash scripts/check-hospital.sh [https://<PUBLIC_HOST>] [AAAA-MM]
# Sin URL usa PUBLIC_HOST de .env; sin período, el mes UTC actual.
set -euo pipefail

env_file=${ENV_FILE:-.env}
value() {
  [[ -f $env_file ]] || return 0
  sed -n "s/^$1=//p" "$env_file" | tail -n 1
}

base=${1:-}
if [[ -z $base ]]; then
  host=$(value PUBLIC_HOST)
  [[ -n $host ]] || { echo "Indicar la URL o definir PUBLIC_HOST en $env_file" >&2; exit 2; }
  base="https://$host"
fi
base=${base%/}
period=${2:-$(date -u +%Y-%m)}
[[ $period =~ ^[0-9]{4}-(0[1-9]|1[0-2])$ ]] || { echo "Período inválido: usar AAAA-MM" >&2; exit 2; }
token=${AUTH_ADMIN_TOKEN:-$(value AUTH_ADMIN_TOKEN)}
[[ -n $token ]] || { echo "Falta AUTH_ADMIN_TOKEN en el entorno o en $env_file" >&2; exit 2; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failed=0

check() {
  local name=$1 path=$2 status code
  : > "$tmp/body"
  # The header goes through stdin (printf is a builtin): the token never
  # appears in the process list or in the output.
  status=$(printf 'Authorization: Bearer %s\n' "$token" |
    curl -sS --max-time 15 -o "$tmp/body" -w '%{http_code}' -H @- "$base$path" 2>/dev/null) || status=000
  code=$(grep -o '"code":"[A-Z_]*"' "$tmp/body" | head -n 1 | cut -d '"' -f 4 || true)
  case "$status:$code" in
    200:*) echo "OK    $name: 200" ; return ;;
    404:NOT_FOUND)
      echo "FALLA $name: 404 NOT_FOUND. El contenedor de Empleados no tiene esta ruta: su imagen es anterior a 65226bf (10/11/2026)."
      echo "      Reconstruir solo Empleados: docker compose <archivos> build employees && docker compose <archivos> up -d --no-deps --wait employees" ;;
    401:*) echo "FALLA $name: 401. El token administrativo no coincide con el .env con que se levantó el gateway." ;;
    403:*) echo "FALLA $name: 403. La credencial usada no tiene rol admin." ;;
    503:NOT_CONFIGURED) echo "FALLA $name: 503 NOT_CONFIGURED. Empleados no recibió CLINIC_BASE_URL; revisar .env y recrear Empleados." ;;
    503:CLINIC_UNAVAILABLE) echo "FALLA $name: 503 CLINIC_UNAVAILABLE. Empleados llamó al hospital y falló: clave rechazada, red, timeout o respuesta incompatible. Ver: docker compose <archivos> logs --since 10m employees" ;;
    400:*) echo "FALLA $name: 400 ${code:-sin código}. Revisar el período enviado." ;;
    000:*) echo "FALLA $name: sin respuesta de $base." ;;
    *) echo "FALLA $name: $status ${code:-sin código}." ;;
  esac
  failed=1
}

echo "Hospital vía $base (solo lectura, período $period)"
check "médicos" "/v1/employees/clinic/doctors"
check "facturación" "/v1/employees/clinic/usage?period=$period"
exit "$failed"
