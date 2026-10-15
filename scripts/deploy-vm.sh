#!/usr/bin/env bash
# Despliega Car Cruds en una VM Ubuntu 24.04 con HTTPS. Ejecutar desde la
# raíz del repositorio clonado. Es repetible: conserva .env y los datos.
set -euo pipefail

if [[ ! -f compose.yaml || ! -f compose.deploy.yaml ]]; then
  echo "Ejecutar desde la raíz del repositorio" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
  echo "Instalando dependencias..."
  sudo apt-get update
  sudo apt-get install -y docker.io docker-compose-v2 curl
fi

DOCKER=(docker)
if ! docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
fi
"${DOCKER[@]}" info >/dev/null
if ! "${DOCKER[@]}" compose version >/dev/null 2>&1; then
  sudo apt-get update
  sudo apt-get install -y docker-compose-v2
fi

if [[ ! -f .env ]]; then
  echo "Generando .env con credenciales aleatorias..."
  "${DOCKER[@]}" run --rm -u "$(id -u):$(id -g)" -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
    -v "$PWD":/src -w /src golang:1.26.1-alpine go run ./cmd/devsetup
fi
chmod 600 .env

if ! grep -q '^PUBLIC_HOST=.\+' .env; then
  ip=$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]')
  host="${ip//./-}.sslip.io"
  # Eliminar la asignación vacía evita claves duplicadas si se copió .env.example.
  sed -i '/^PUBLIC_HOST=/d' .env
  printf '\nPUBLIC_HOST=%s\n' "$host" >> .env
  echo "PUBLIC_HOST no estaba definido; se usa $host"
fi

"${DOCKER[@]}" compose -f compose.yaml -f compose.deploy.yaml config --quiet
"${DOCKER[@]}" compose -f compose.yaml -f compose.deploy.yaml up -d --build --wait --wait-timeout 600

host=$(grep '^PUBLIC_HOST=' .env | tail -1 | cut -d= -f2-)
echo
echo "Contenedores iniciados para https://$host; verificar DNS, certificado y contrato desde fuera de la VM:"
echo "go run ./cmd/checkshared -url https://$host -env .env"
echo "Token del gimnasio: grep AUTH_GYM_TOKEN .env (compartir solo por canal privado)"
