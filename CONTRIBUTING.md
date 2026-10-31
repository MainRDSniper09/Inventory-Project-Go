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

Tipos: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`, `chore`. GoReleaser usa estos prefijos para el changelog.

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
