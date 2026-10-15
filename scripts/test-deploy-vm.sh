#!/usr/bin/env bash
# Ensaya el flujo del script con comandos falsos: no instala ni despliega nada.
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
mkdir -p "$repo/tmp"
root=$(mktemp -d "$repo/tmp/deploy-vm-test.XXXXXX")
mkdir -p "$root/bin"

cat > "$root/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_STATE/docker-calls"
if [[ "${1:-}" == info ]]; then
  [[ ! -f "$MOCK_STATE/engine-down" ]]
elif [[ "${1:-} ${2:-}" == 'compose version' ]]; then
  [[ ! -f "$MOCK_STATE/missing-plugin" || -f "$MOCK_STATE/plugin-installed" ]]
elif [[ "${1:-}" == run ]]; then
  printf 'APP_PORT=8080\nAUTH_GYM_TOKEN=fictitious-test-token\n' > .env
elif [[ " $* " == *' config '* ]]; then
  [[ ! -f "$MOCK_STATE/invalid-config" ]]
fi
MOCK
cat > "$root/bin/sudo" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == docker ]]; then
  shift
  exec docker "$@"
fi
if [[ "${1:-} ${2:-}" == 'apt-get install' ]]; then
  touch "$MOCK_STATE/plugin-installed"
fi
MOCK
cat > "$root/bin/curl" <<'MOCK'
#!/usr/bin/env bash
printf '203.0.113.10\n'
MOCK
chmod +x "$root/bin/"*
export PATH="$root/bin:$PATH"

scenario() {
  export MOCK_STATE="$root/$1"
  mkdir -p "$MOCK_STATE"
  cd "$MOCK_STATE"
  touch compose.yaml compose.deploy.yaml
}
deploy() { bash "$repo/scripts/deploy-vm.sh" > result.txt 2>&1; }
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

scenario existing
printf 'AUTH_GYM_TOKEN=fictitious-test-token\nPUBLIC_HOST=existing.example.invalid\n' > .env
cp .env expected.env
deploy
cmp -s .env expected.env || fail 'se modificó una configuración existente'
grep -q 'up -d --build --wait' docker-calls || fail 'no se inició Compose'

scenario empty-host
printf 'AUTH_GYM_TOKEN=fictitious-test-token\nPUBLIC_HOST=\n' > .env
deploy
[[ $(grep -c '^PUBLIC_HOST=' .env) == 1 ]] || fail 'PUBLIC_HOST duplicado'
grep -q '^PUBLIC_HOST=203-0-113-10.sslip.io$' .env || fail 'host automático incorrecto'

scenario missing-plugin
touch missing-plugin
deploy
[[ -f plugin-installed ]] || fail 'no se instaló el plugin faltante'

scenario new-env
deploy
grep -q '^AUTH_GYM_TOKEN=fictitious-test-token$' .env || fail 'devsetup no se ejecutó'
if [[ "$OSTYPE" == linux* ]]; then
  [[ $(stat -c '%a' .env) == 600 ]] || fail 'permisos incorrectos de .env'
fi

scenario engine-down
touch engine-down
if deploy; then fail 'se aceptó un motor inaccesible'; fi
! grep -q 'up -d' docker-calls || fail 'se intentó desplegar sin motor'

scenario invalid-config
printf 'PUBLIC_HOST=existing.example.invalid\n' > .env
touch invalid-config
if deploy; then fail 'se aceptó una configuración inválida'; fi
! grep -q 'up -d' docker-calls || fail 'se intentó desplegar una configuración inválida'

printf 'PASS: seis escenarios del script con comandos falsos; sin instalaciones ni acceso externo\n'
