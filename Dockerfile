# syntax=docker/dockerfile:1

# ---------- Etapa 1: compilación ----------
FROM golang:1.27-alpine AS build
WORKDIR /src

# Primero sólo los manifiestos: Docker cachea esta capa mientras no cambien
# las dependencias, acelerando las reconstrucciones.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=none
# CGO_ENABLED=0 -> binario 100% estático (modernc.org/sqlite es Go puro).
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /out/inventory ./cmd/inventory \
 && mkdir -p /out/data

# ---------- Etapa 2: runtime mínimo ----------
# distroless/static: sin shell ni gestor de paquetes (~2 MB). Menor superficie
# de ataque y usuario no-root por defecto.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/inventory /usr/local/bin/inventory
COPY --from=build --chown=nonroot:nonroot /out/data /data

ENV INVENTORY_ADDR=:8080 \
    INVENTORY_DB_PATH=/data/inventory.db \
    INVENTORY_LOG_FORMAT=json

EXPOSE 8080
VOLUME ["/data"]
USER nonroot:nonroot

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/inventory", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/inventory"]
