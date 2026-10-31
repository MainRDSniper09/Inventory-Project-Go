# Guía de contribución

¡Gracias por tu interés! Este proyecto es open source y también material de aprendizaje, así que las preguntas son tan bienvenidas como el código.

## Flujo de trabajo

1. **Busca o abre un Issue** antes de programar algo grande. Comenta que lo tomarás.
2. Haz *fork* y crea una rama desde `main`:
   `feat/<issue>-descripcion-corta`, `fix/<issue>-...`, `docs/...`
3. Programa siguiendo las reglas de abajo e incluye pruebas.
4. Verifica localmente:
   ```bash
   gofmt -l .        # no debe listar archivos
   go vet ./...
   go test ./...
   ```
5. Abre un Pull Request usando la plantilla y enlaza el Issue (`Closes #123`).

## Commits — Conventional Commits

```
feat(sales): permitir cancelar una venta
fix(sqlite): mapear error de FK a ErrInvalidInput
docs: actualizar diagrama ER
test(services): cubrir stock insuficiente al fusionar líneas
```

Tipos: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`.

## Publicar una versión

Las versiones siguen [SemVer](https://semver.org/lang/es/) y **cada milestone es una versión**. Su título debe empezar con ella: `v0.2.0 Seguridad y usuarios`.

1. Asigna cada issue al milestone de la versión donde se entregará.
2. Pon etiquetas `type:*` a issues y PRs: con ellas se agrupan las notas de la release.
3. Cuando todos los issues estén cerrados, **cierra el milestone** (*Issues → Milestones → Close*).
4. El workflow [`milestone-release.yml`](.github/workflows/milestone-release.yml):
   - valida que el título empiece con `vX.Y.Z` (si no, lo ignora);
   - si quedan issues abiertos, **reabre el milestone** y falla listándolos en el resumen del run;
   - crea el tag sobre el último commit de `main`;
   - llama a [`release.yml`](.github/workflows/release.yml), que publica los binarios (GoReleaser) y la imagen `ghcr.io/<owner>/inventory-go` con las etiquetas `X.Y.Z`, `X.Y` y `latest`.
5. Las notas incluyen la descripción del milestone, sus issues agrupados por tipo, cómo instalar esa versión y los PRs incluidos ([`release-notes.sh`](.github/scripts/release-notes.sh)).

| Situación | Qué hacer |
|---|---|
| La release falló a mitad de camino | *Actions → Milestone release → Run workflow* con el número del milestone. Es idempotente: reutiliza el tag. |
| Necesito un hotfix sin milestone | `git tag v0.1.1 && git push origin v0.1.1` también publica (notas sólo con los PRs). |
| Cerré un milestone por error | Si aún había issues abiertos, se reabre solo. Si estaba completo, la release ya salió: publica una versión nueva en lugar de borrarla. |

Vista previa local de las notas: `REPO=owner/repo bash .github/scripts/release-notes.sh v0.2.0 "v0.2.0 Seguridad y usuarios"`.

## Reglas de arquitectura

- `internal/core/**` **no** puede importar `internal/adapters/**`, `database/sql` ni `net/http`.
- Nueva tecnología = nuevo adaptador que implementa un puerto existente.
- Nueva regla de negocio = en `domain/` (si es de una entidad) o `services/` (si orquesta varias).
- Cambios de esquema = **nueva** migración `NNNN_descripcion.sql`; nunca editar una publicada.
- Dinero siempre en centavos (`domain.Money`); fechas siempre en UTC.
- Toda consulta SQL usa placeholders `?`; nunca concatenes valores del usuario.

## Pruebas

| Cambiaste… | Agrega pruebas en… |
|---|---|
| Entidad del dominio | `internal/core/domain/*_test.go` |
| Caso de uso | `internal/core/services/services_test.go` (con fakes) |
| SQL / migración / trigger | `internal/adapters/sqlite/sqlite_test.go` |
| Endpoint | `internal/adapters/http/http_test.go` |
| UI (HTML/CSS/JS) | `scripts/screenshots/capture.mjs` — luego ejecuta `bash scripts/screenshots/run.sh` y sube las capturas actualizadas de `docs/images/` |

## Etiquetas de Issues

`type:feature`, `type:bug`, `type:docs`, `type:chore` · `area:domain`, `area:api`, `area:db`, `area:ui`, `area:devops` · `good first issue`, `help wanted` · `priority:high|medium|low`.
