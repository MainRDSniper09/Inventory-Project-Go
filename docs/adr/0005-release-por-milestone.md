# ADR-0005: Una release por milestone, publicada al cerrarlo

- **Estado:** Aceptado
- **Fecha:** 2026-10-31

## Contexto

El roadmap ya está organizado en milestones (`v0.1.0 MVP`, `v0.2.0 Seguridad y usuarios`…), cada uno con sus issues. Publicar una versión requería crear el tag a mano, y las notas de GoReleaser se armaban con los mensajes de commit, sin relación con el milestone planificado.

Queremos que **planificar** (milestone), **ejecutar** (issues y PRs) y **entregar** (release) queden enlazados, sin pasos manuales que se puedan olvidar.

## Decisión

- **Convención:** cada milestone es una versión SemVer y su título empieza con ella (`vX.Y.Z <nombre>`).
- **Disparador:** el evento `milestone: closed` ejecuta `milestone-release.yml`, que:
  1. ignora milestones cuyo título no empiece con `vX.Y.Z`;
  2. si hay issues abiertos, **reabre el milestone** y falla, para no publicar una versión incompleta;
  3. es idempotente: no hace nada si la release existe y reutiliza el tag si ya existía;
  4. crea el tag sobre `main` mediante la API.
- **Publicación:** `release.yml` se vuelve un *workflow reutilizable* (`workflow_call`) y lo llama directamente el workflow del milestone.
  - Motivo: GitHub **no dispara** workflows por eventos creados con el `GITHUB_TOKEN` (protección contra bucles). Un tag creado por un workflow nunca activaría `on: push: tags`.
  - El push manual de tags sigue funcionando para hotfixes.
- **Notas:** `release-notes.sh` combina la descripción del milestone, sus issues agrupados por etiqueta `type:*`, instrucciones de instalación y los PRs incluidos (API `generate-notes` con las categorías de `.github/release.yml`). El changelog propio de GoReleaser se desactiva.

## Alternativas descartadas

| Alternativa | Por qué no |
|---|---|
| Tag manual + changelog por commits | Depende de recordar el paso; las notas no reflejan lo planificado. |
| Crear el tag con un PAT para que dispare `on: push: tags` | Requiere guardar y rotar un secreto personal; el workflow reutilizable logra lo mismo sin secretos extra. |
| release-please / semantic-release | Calculan la versión a partir de los commits; aquí la versión ya está decidida en el roadmap (milestone). |

## Consecuencias

- ✅ Cerrar un milestone es el único paso para publicar; la trazabilidad roadmap → issues → PRs → release queda en las notas.
- ✅ No se publican versiones con trabajo pendiente.
- ✅ Las releases fallidas se reintentan sin efectos duplicados.
- ⚠️ El tag apunta al último commit de `main` al momento de cerrar el milestone: hay que cerrarlo después de fusionar todos sus PRs.
- ⚠️ Las imágenes de `ghcr.io` se crean como privadas en la primera publicación; hay que hacer pública la imagen una vez (*Packages → inventory-go → Package settings → Change visibility*).
