#!/usr/bin/env bash
# Escenarios de scripts/check-hospital.sh con un curl simulado: sin red ni credenciales reales.
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat > "$tmp/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
out='' url='' header=$(cat)
while (($#)); do
  case $1 in
    -o) out=$2; shift ;;
    -X|--request|-d|--data*|-F|--form) echo "método o cuerpo no permitido: $1" >&2; exit 99 ;;
    http*) url=$1 ;;
  esac
  shift
done
[[ $header == "Authorization: Bearer fixture-admin-token-not-real" ]] || { echo 'encabezado inesperado' >&2; exit 98; }
if [[ $url == */clinic/doctors ]]; then status=${MOCK_DOCTORS:-200}; else status=${MOCK_USAGE:-200}; fi
case $status in
  200) body='{"doctors":[]}' ;;
  404) body='{"code":"NOT_FOUND","message":"Ruta inexistente"}' ;;
  503) body="{\"code\":\"${MOCK_CODE:-CLINIC_UNAVAILABLE}\",\"message\":\"x\"}" ;;
  *) body='{}' ;;
esac
printf '%s' "$body" > "$out"
printf '%s' "$status"
MOCK
chmod +x "$tmp/curl"
export PATH="$tmp:$PATH"
printf 'PUBLIC_HOST=demo.example\nAUTH_ADMIN_TOKEN=fixture-admin-token-not-real\n' > "$tmp/env"
export ENV_FILE="$tmp/env"
unset AUTH_ADMIN_TOKEN
run() { bash "$repo/scripts/check-hospital.sh" "$@" > "$tmp/result" 2>&1; }

run "" 2026-11
grep -q 'https://demo.example' "$tmp/result"
[[ $(grep -c '^OK' "$tmp/result") == 2 ]]

if MOCK_DOCTORS=404 MOCK_USAGE=404 run "" 2026-11; then echo 'Se aceptó un Empleados sin rutas' >&2; exit 1; fi
[[ $(grep -c 'anterior a 65226bf' "$tmp/result") == 2 ]]

if MOCK_USAGE=503 MOCK_CODE=NOT_CONFIGURED run https://demo.example 2026-11; then echo 'Se aceptó un hospital sin configurar' >&2; exit 1; fi
grep -q 'NOT_CONFIGURED' "$tmp/result"

if run https://demo.example 2026-13; then echo 'Se aceptó un período inválido' >&2; exit 1; fi

if grep -rq 'fixture-admin-token-not-real' "$tmp/result"; then echo 'El token apareció en la salida' >&2; exit 1; fi
echo 'PASS: rutas presentes, imagen anterior, hospital sin configurar y período inválido'
