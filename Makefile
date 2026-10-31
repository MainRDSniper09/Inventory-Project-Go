# Atajos de desarrollo. En Windows: usar Git Bash o WSL, o ejecutar los
# comandos de cada objetivo directamente.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: help run seed build test cover lint fmt docker-build docker-up docker-down clean

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

run: ## Ejecuta la app en modo desarrollo
	go run ./cmd/inventory

seed: ## Ejecuta la app con datos de demostración
	INVENTORY_SEED=true go run ./cmd/inventory

build: ## Compila el binario estático en ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/inventory ./cmd/inventory

test: ## Ejecuta todas las pruebas
	go test ./...

cover: ## Genera reporte de cobertura (coverage.html)
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1

lint: ## Analiza el código con golangci-lint
	golangci-lint run ./...

fmt: ## Formatea el código
	gofmt -w .

docker-build: ## Construye la imagen Docker
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t inventory-go:$(VERSION) .

docker-up: ## Levanta la app con Docker Compose
	docker compose up -d --build

docker-down: ## Detiene la app de Docker Compose
	docker compose down

clean: ## Borra artefactos generados
	rm -rf bin dist coverage.out coverage.html
