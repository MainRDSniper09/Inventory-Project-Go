#!/usr/bin/env bash
# Genera las notas (Markdown) de una release.
#
#   REPO=owner/repo bash .github/scripts/release-notes.sh <tag> [titulo-del-milestone]
#
# Contenido:
#   1. Resumen del milestone y sus issues cerrados agrupados por tipo (si aplica).
#   2. Cómo instalar esta versión (binario / Docker).
#   3. PRs incluidos desde el tag anterior (generados por GitHub con las
#      categorías de .github/release.yml).
#
# Sólo requiere `gh` y `git` (usa el jq integrado de gh: --jq).
set -euo pipefail

TAG="${1:?uso: release-notes.sh <tag> [milestone]}"
export MILESTONE="${2:-}"
REPO="${REPO:?define REPO=owner/repo}"
OWNER="${REPO%%/*}"
OWNER_LC=$(tr '[:upper:]' '[:lower:]' <<<"$OWNER")
VERSION="${TAG#v}"
PREV=$(git describe --tags --abbrev=0 "${TAG}^" 2>/dev/null || true)

if [[ -n "$MILESTONE" ]]; then
  desc=$(gh api "repos/$REPO/milestones?state=all&per_page=100" \
    --jq '.[] | select(.title == env.MILESTONE) | .description')
  echo "## 🎯 Milestone: $MILESTONE"
  echo
  [[ -n "$desc" ]] && { echo "$desc"; echo; }
  echo "### Issues resueltos"
  gh issue list -R "$REPO" --milestone "$MILESTONE" --state closed --limit 500 \
    --json number,title,labels --jq '
      ["type:feature", "type:bug", "type:test", "type:docs", "type:chore", "other"] as $order
      | {"type:feature": "✨ Funcionalidades", "type:bug": "🐛 Correcciones", "type:test": "🧪 Pruebas",
         "type:docs": "📚 Documentación", "type:chore": "🔧 Mantenimiento e infraestructura",
         "other": "📌 Otros"} as $heading
      | map(. + {t: ([.labels[].name | select(startswith("type:"))][0] // "other")})
      | map(.t as $t | .t = (if ($order | index($t)) == null then "other" else $t end))
      | group_by(.t)
      | sort_by(.[0].t as $t | $order | index($t))
      | .[]
      | "\n#### \($heading[.[0].t])\n" + (sort_by(.number) | map("- \(.title) (#\(.number))") | join("\n"))'
  echo
fi

cat <<EOF

## 📦 Instalación

**Binario** (sin dependencias): descarga el archivo de tu sistema en *Assets*, descomprímelo y ejecuta \`inventory\` (o \`inventory.exe\`). Abre http://localhost:8080.

**Docker:**
\`\`\`bash
docker run -d -p 8080:8080 -v inventory-data:/data ghcr.io/${OWNER_LC}/inventory-go:${VERSION}
\`\`\`

Verifica la descarga con \`checksums.txt\`.

EOF

echo "## 🔀 Cambios incluidos"
args=(-f tag_name="$TAG")
[[ -n "$PREV" ]] && args+=(-f previous_tag_name="$PREV")
gh api "repos/$REPO/releases/generate-notes" "${args[@]}" --jq .body | sed "1{/^## What.s Changed/d}"
