#!/usr/bin/env bash
# Regenera las capturas de docs/images/ usando sólo Docker:
#   1. construye la imagen de la app y la levanta con datos demo;
#   2. ejecuta Playwright (Chromium sin interfaz) en otro contenedor de la
#      misma red, que recorre la UI y guarda las capturas.
#
# Uso: bash scripts/screenshots/run.sh
set -euo pipefail

# `pwd -W` da la ruta nativa en Git Bash (C:/...); en Linux/macOS usa `pwd`.
ROOT="$(cd "$(dirname "$0")/../.." && (pwd -W 2>/dev/null || pwd))"
PW_VERSION="1.52.0"
NET="inventory-screenshots"
APP="inventory-screenshots-app"
export MSYS_NO_PATHCONV=1 # Git Bash en Windows: no reescribir rutas de contenedor

cleanup() {
  docker rm -f "$APP" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

echo "==> Construyendo imagen"
docker build -q -t inventory-go:screenshots "$ROOT" >/dev/null

echo "==> Levantando la app con datos demo"
docker network create "$NET" >/dev/null
docker run -d --name "$APP" --network "$NET" -e INVENTORY_SEED=true inventory-go:screenshots >/dev/null

mkdir -p "$ROOT/docs/images"
echo "==> Ejecutando Playwright $PW_VERSION"
docker run --rm --network "$NET" --ipc=host \
  -v "$ROOT:/work" \
  "mcr.microsoft.com/playwright:v${PW_VERSION}-noble" \
  bash -c "cd /tmp && npm i --silent --no-audit --no-fund playwright@${PW_VERSION} >/dev/null \
    && cp /work/scripts/screenshots/capture.mjs . \
    && for i in \$(seq 1 30); do node -e \"fetch('http://$APP:8080/healthz').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\" && break; sleep 1; done \
    && node capture.mjs http://$APP:8080 /work/docs/images"

echo "==> Capturas en docs/images/"
