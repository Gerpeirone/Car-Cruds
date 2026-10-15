#!/usr/bin/env bash
# Despliega Car Cruds en una VM Ubuntu 24.04 con HTTPS. Ejecutar desde la
# raíz del repositorio clonado. Es repetible: conserva .env y los datos.
set -euo pipefail

if [[ ! -f compose.yaml || ! -f compose.deploy.yaml ]]; then
  echo "Ejecutar desde la raíz del repositorio" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Instalando Docker..."
  sudo apt-get update
  sudo apt-get install -y docker.io docker-compose-v2 curl
  sudo usermod -aG docker "$USER"
fi

DOCKER=(docker)
if ! docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
fi

if [[ ! -f .env ]]; then
  echo "Generando .env con credenciales aleatorias..."
  "${DOCKER[@]}" run --rm -u "$(id -u):$(id -g)" -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
    -v "$PWD":/src -w /src golang:1.26.1-alpine go run ./cmd/devsetup
fi

if ! grep -q '^PUBLIC_HOST=.\+' .env; then
  ip=$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]')
  host="${ip//./-}.sslip.io"
  echo "PUBLIC_HOST=$host" >> .env
  echo "PUBLIC_HOST no estaba definido; se usa $host"
fi

"${DOCKER[@]}" compose -f compose.yaml -f compose.deploy.yaml up -d --build --wait --wait-timeout 600

host=$(grep '^PUBLIC_HOST=' .env | tail -1 | cut -d= -f2-)
echo
echo "Car Cruds publicado en https://$host"
echo "Token del gimnasio: grep AUTH_GYM_TOKEN .env (compartir solo por canal privado)"
