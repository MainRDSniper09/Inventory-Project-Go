# ADR-0004: Distribución como binario único y como imagen Docker

- **Estado:** Aceptado
- **Fecha:** 2026-10-31

## Contexto

Se pidió desplegar con Docker "o un instalable, lo que quede mejor". Go compila rápido y genera binarios estáticos, lo que abre ambas opciones.

## Decisión

Ofrecemos **ambas**, generadas automáticamente al crear un tag `vX.Y.Z`:

1. **Binario único** por plataforma (GoReleaser: linux/windows/darwin × amd64/arm64). La UI web, las migraciones y el driver SQLite van embebidos (`//go:embed`). Es la opción **recomendada para usuarios finales** de un negocio pequeño: descargar, ejecutar, abrir el navegador.
2. **Imagen Docker** multi-arch en `ghcr.io`, basada en `distroless/static:nonroot`. Es la opción **recomendada para servidores** y para quien ya opera contenedores.

Instaladores nativos (MSI, .deb, Homebrew) quedan en el roadmap (v1.0.0).

## Consecuencias

- ✅ Cero dependencias en tiempo de ejecución.
- ✅ Imagen pequeña, sin shell, no-root, con `HEALTHCHECK` vía `inventory -healthcheck`.
- ⚠️ La BD SQLite debe estar en un volumen persistente (`/data`) en Docker.
