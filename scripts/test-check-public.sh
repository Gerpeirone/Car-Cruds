#!/usr/bin/env bash
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat > "$tmp/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
args="$*"
if [[ $args == *'http://demo.example/'* ]]; then
  printf 'https://demo.example/'
elif [[ ${MOCK_TLS_FAILURE:-} == true ]]; then
  exit 60
elif [[ $args == *'/v1/employees'* ]]; then
  printf '%s' "${MOCK_API_STATUS:-401}"
elif [[ $args == *'/internal/ready'* ]]; then
  printf '404'
else
  printf '200'
fi
MOCK
chmod +x "$tmp/curl"
export PATH="$tmp:$PATH"
unset AUTH_GYM_TOKEN

bash "$repo/scripts/check-public.sh" https://demo.example > "$tmp/result"
grep -q 'OMITIDO: contrato autenticado' "$tmp/result"
if MOCK_API_STATUS=200 bash "$repo/scripts/check-public.sh" https://demo.example > "$tmp/result" 2>&1; then
  echo 'Se aceptó una API pública sin autenticación' >&2
  exit 1
fi
if MOCK_TLS_FAILURE=true bash "$repo/scripts/check-public.sh" https://demo.example > "$tmp/result" 2>&1; then
  echo 'Se aceptó un fallo TLS' >&2
  exit 1
fi
echo 'PASS: entrada válida, API expuesta y fallo TLS'
