# ADR-0002: SQLite con driver en Go puro (modernc.org/sqlite)

- **Estado:** Aceptado
- **Fecha:** 2026-10-31

## Contexto

Se requiere SQLite como base de datos. Existen dos drivers principales:

| Driver | CGO | Pros | Contras |
|---|---|---|---|
| `github.com/mattn/go-sqlite3` | Sí | El más usado, ligeramente más rápido | Requiere gcc; compilación cruzada complicada; no permite `CGO_ENABLED=0` |
| `modernc.org/sqlite` | No | Go puro, binario estático, compila en cualquier SO | Algo más lento en cargas muy pesadas |

## Decisión

Usamos `modernc.org/sqlite`.

## Consecuencias

- ✅ `go build` funciona en Windows/Linux/macOS sin instalar gcc.
- ✅ Binarios estáticos (`CGO_ENABLED=0`) → imagen Docker `distroless/static` de pocos MB.
- ✅ GoReleaser compila para 6 plataformas desde un solo runner.
- ⚠️ `go test -race` requiere CGO; en CI Linux se puede habilitar aparte si se necesita.
- ⚠️ SQLite admite un único escritor: adecuado para un negocio pequeño/mediano. Para alta concurrencia de escritura, el roadmap contempla un adaptador PostgreSQL (la arquitectura hexagonal lo hace posible sin tocar el núcleo).
