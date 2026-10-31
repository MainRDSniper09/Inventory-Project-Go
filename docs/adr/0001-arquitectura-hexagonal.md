# ADR-0001: Arquitectura Hexagonal (Ports & Adapters)

- **Estado:** Aceptado
- **Fecha:** 2026-10-31

## Contexto

El proyecto tiene dos objetivos: ser un inventario funcional y servir como material de aprendizaje de Go con una arquitectura que escale. Se espera que el sistema crezca (usuarios, proveedores, multi-almacén) y que eventualmente cambie de motor de BD o agregue nuevas interfaces (CLI, gRPC).

## Decisión

Adoptamos Arquitectura Hexagonal con esta estructura:

- `internal/core/domain` — entidades y reglas puras, sin dependencias.
- `internal/core/ports` — interfaces de entrada (casos de uso) y salida (repositorios, tx, reloj).
- `internal/core/services` — implementación de casos de uso.
- `internal/adapters/{http,sqlite}` — tecnología concreta.
- `cmd/inventory` — composition root con inyección de dependencias manual.

Se descarta usar un framework de DI (wire, fx): para este tamaño, la inyección manual en `main.go` es más explícita y didáctica.

## Consecuencias

- ✅ Los casos de uso se prueban con fakes en memoria, sin BD.
- ✅ Cambiar de tecnología afecta sólo a un adaptador y a `main.go`.
- ✅ Las dependencias son explícitas y verificables por el compilador.
- ⚠️ Más archivos y algo de "boilerplate" (DTOs, mapeos) que en un CRUD directo.
- ⚠️ Requiere disciplina: nada en `core/` puede importar `adapters/`.
