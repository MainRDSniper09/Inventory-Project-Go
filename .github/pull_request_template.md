## Resumen

<!-- ¿Qué cambia y por qué? -->

Closes #

## Capas afectadas

- [ ] `core/domain`
- [ ] `core/ports`
- [ ] `core/services`
- [ ] `adapters/sqlite` (¿nueva migración?)
- [ ] `adapters/http` (¿cambia la API? actualizar `docs/API.md`)
- [ ] Documentación / diagramas

## Checklist

- [ ] `gofmt -l .` no lista archivos
- [ ] `go vet ./...` sin advertencias
- [ ] `go test ./...` pasa
- [ ] Agregué/actualicé pruebas
- [ ] `core/` no importa adaptadores
