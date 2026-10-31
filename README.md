# Inventario Go

[![CI](https://github.com/MainRDSniper09/Inventory-Project-Go/actions/workflows/ci.yml/badge.svg)](https://github.com/MainRDSniper09/Inventory-Project-Go/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Sistema de **inventario y ventas** open source escrito en Go, con **Arquitectura Hexagonal**, principios **SOLID** y **SQLite**. Se distribuye como **un único binario** (con la UI web embebida) o como **imagen Docker**.

Nació como proyecto de aprendizaje: el código está comentado en español explicando el *por qué* de cada decisión.

---

## Contenido

- [Funcionalidades](#funcionalidades)
- [Capturas](#capturas)
- [Inicio rápido](#inicio-rápido)
- [Arquitectura](#arquitectura)
- [Base de datos](#base-de-datos)
- [API](#api)
- [Configuración](#configuración)
- [Pruebas](#pruebas)
- [Despliegue](#despliegue)
- [Roadmap](#roadmap)
- [Contribuir](#contribuir)

## Funcionalidades

**MVP v0.1.0**

- 📦 **Productos**: crear, listar, buscar, editar y archivar (borrado lógico). SKU único e inmutable.
- 🏷️ **Categorías** para agrupar productos.
- 📊 **Inventario con kardex**: entradas, salidas y ajustes. Cada cambio de stock queda registrado en un libro **inmutable**; el stock nunca puede ser negativo.
- 🧾 **Ventas** atómicas: el precio se toma del catálogo, el stock se descuenta automáticamente y, si algo falla, nada se guarda.
- 📈 **Dashboard**: ventas, unidades vendidas, ingresos, valor del inventario, ventas por día, productos más vendidos y alertas de stock bajo.
- 📄 **Reportes** de ventas por rango de fechas en **JSON** o **CSV** (protegido contra *CSV injection*).
- 🖥️ **UI web** embebida en el binario, con tema claro/oscuro.

## Capturas

> Capturas **reales**, generadas automáticamente con [Playwright](https://playwright.dev) recorriendo la UI: crear una categoría y un producto, registrar movimientos, vender y exportar. El mismo script sirve como prueba de humo de la interfaz: falla si hay errores de JavaScript o si un flujo no produce el resultado esperado. Para regenerarlas: `bash scripts/screenshots/run.sh` (sólo requiere Docker).

### Dashboard

Indicadores del periodo, ingresos por día, productos más vendidos y alertas de stock bajo.

![Dashboard con KPIs, ingresos por día, más vendidos y stock bajo](docs/images/dashboard.png)

<table>
  <tr>
    <th width="70%">Tema oscuro (automático según el sistema)</th>
    <th>Móvil</th>
  </tr>
  <tr>
    <td><img src="docs/images/dashboard-dark.png" alt="Dashboard en tema oscuro"></td>
    <td><img src="docs/images/dashboard-mobile.png" alt="Dashboard en un teléfono"></td>
  </tr>
</table>

### Productos

Catálogo con búsqueda por SKU o nombre, existencia en tiempo real y acciones de stock, edición y archivado.

![Listado de productos](docs/images/products.png)

<table>
  <tr>
    <th>Alta de producto con stock inicial</th>
    <th>Movimiento de inventario y kardex</th>
  </tr>
  <tr>
    <td><img src="docs/images/product-dialog.png" alt="Formulario de nuevo producto"></td>
    <td><img src="docs/images/inventory-movement.png" alt="Ajuste de inventario con historial del kardex"></td>
  </tr>
</table>

### Ventas

Venta de varias líneas con total estimado. El precio lo pone el servidor desde el catálogo y el stock se descuenta en la misma transacción.

![Registro de venta y últimas ventas](docs/images/sales.png)

### Reportes

Detalle de ventas por rango de fechas, con descarga en CSV.

![Reporte de ventas](docs/images/reports.png)

## Inicio rápido

### Opción A — Docker (recomendado para servidor)

```bash
git clone https://github.com/MainRDSniper09/Inventory-Project-Go.git
cd Inventory-Project-Go
INVENTORY_SEED=true docker compose up -d --build   # con datos de demostración
```

Abre **http://localhost:8080**.

### Opción B — Binario (recomendado para usuario final)

Descarga el archivo para tu sistema desde [Releases](https://github.com/MainRDSniper09/Inventory-Project-Go/releases), descomprímelo y ejecuta:

```bash
./inventory            # Linux / macOS
inventory.exe          # Windows
```

### Opción C — Desde el código (desarrollo)

Requiere **Go 1.27+**. No necesitas gcc ni instalar SQLite.

```bash
go run ./cmd/inventory                       # BD vacía
INVENTORY_SEED=true go run ./cmd/inventory   # con datos demo
```

En PowerShell: `$env:INVENTORY_SEED="true"; go run ./cmd/inventory`

## Arquitectura

**Hexagonal (Ports & Adapters)**: el negocio está en el centro y no depende de ninguna tecnología. Las dependencias siempre apuntan hacia adentro.

```mermaid
flowchart LR
    subgraph P["Adaptadores primarios"]
        HTTP["HTTP<br/>API REST + UI"]
    end
    subgraph C["Núcleo"]
        IN["Puertos de entrada<br/>(casos de uso)"]
        SVC["Servicios"]
        DOM["Dominio"]
        OUT["Puertos de salida<br/>(repositorios, tx, reloj)"]
        IN --> SVC --> DOM
        SVC --> OUT
    end
    subgraph S["Adaptadores secundarios"]
        SQL["SQLite<br/>repos + triggers"]
    end
    HTTP --> IN
    SQL -. implementa .-> OUT
```

```text
cmd/inventory/          Composition root (inyección de dependencias)
internal/core/domain    Entidades y reglas de negocio — sin dependencias
internal/core/ports     Interfaces (contratos) de entrada y salida
internal/core/services  Casos de uso
internal/adapters/http  API REST, middlewares, UI embebida
internal/adapters/sqlite Repositorios, migraciones, triggers y vistas
internal/platform       Configuración y reloj
```

📘 **Documentación completa** con diagrama de clases, flujo de petición, diagrama de secuencia de una venta, SOLID aplicado y estrategia de pruebas: **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**

### Secuencia: registrar una venta (resumen)

```mermaid
sequenceDiagram
    actor U as Usuario
    participant H as HTTP
    participant S as SaleService
    participant DB as SQLite
    U->>H: POST /api/v1/sales
    H->>S: Register(cmd)
    S->>DB: BEGIN IMMEDIATE
    S->>DB: lee productos (precio vigente) y stock
    alt stock insuficiente
        S->>DB: ROLLBACK
        H-->>U: 409 insufficient_stock
    else ok
        S->>DB: INSERT sales + sale_items
        Note over DB: triggers: total de la venta,<br/>movimiento SALE, descuento de stock
        S->>DB: COMMIT
        H-->>U: 201 Created
    end
```

## Base de datos

```mermaid
erDiagram
    categories ||--o{ products : clasifica
    products ||--|| inventory : "existencia"
    products ||--o{ stock_movements : kardex
    products ||--o{ sale_items : "se vende en"
    sales ||--|{ sale_items : contiene
    sale_items ||--o| stock_movements : genera
```

SQLite no tiene *stored procedures*; su equivalente idiomático —que usamos— son **triggers** (mantienen stock y totales), **vistas** (consultas reutilizables) y **constraints** (`CHECK quantity >= 0`, FKs, columnas generadas).

📘 Detalle de tablas, triggers, vistas, PRAGMAs y migraciones: **[docs/DATABASE.md](docs/DATABASE.md)**

## API

| Método | Ruta | Descripción |
|---|---|---|
| `GET/POST` | `/api/v1/categories` | Listar / crear categorías |
| `GET/POST` | `/api/v1/products` | Listar-buscar / crear productos |
| `GET/PUT/DELETE` | `/api/v1/products/{id}` | Ver / editar / archivar |
| `GET/POST` | `/api/v1/products/{id}/movements` | Kardex / registrar movimiento |
| `GET/POST` | `/api/v1/sales` | Listar / registrar ventas |
| `GET` | `/api/v1/sales/{id}` | Detalle de venta |
| `GET` | `/api/v1/dashboard` | KPIs del tablero |
| `GET` | `/api/v1/reports/sales?format=csv` | Reporte de ventas |
| `GET` | `/healthz` | Health check |

```bash
curl -s -X POST localhost:8080/api/v1/products -H 'Content-Type: application/json' \
  -d '{"sku":"BEB-COLA-600","name":"Refresco cola","price_cents":1800,"initial_stock":24}'
```

📘 Referencia completa con errores y ejemplos: **[docs/API.md](docs/API.md)**

## Configuración

Variables de entorno ([12-Factor](https://12factor.net/config)):

| Variable | Default | Descripción |
|---|---|---|
| `INVENTORY_ADDR` | `:8080` | Dirección de escucha |
| `INVENTORY_DB_PATH` | `inventory.db` | Ruta del archivo SQLite (`/data/inventory.db` en Docker) |
| `INVENTORY_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `INVENTORY_LOG_FORMAT` | `text` | `text` o `json` (`json` en Docker) |
| `INVENTORY_SEED` | `false` | Carga datos demo si la BD está vacía |
| `INVENTORY_SHUTDOWN_TIMEOUT` | `10s` | Tiempo para apagado ordenado |

Flags: `inventory -version`, `inventory -healthcheck`.

## Pruebas

```bash
go test ./...          # todas las pruebas
go test -cover ./...   # con cobertura
make cover             # reporte HTML
```

| Nivel | Archivo | Qué valida |
|---|---|---|
| Unitarias de dominio | `core/domain/domain_test.go` | Reglas e invariantes de las entidades |
| Unitarias de casos de uso | `core/services/services_test.go` | Orquestación con *fakes* en memoria (sin BD) |
| Integración | `adapters/sqlite/sqlite_test.go` | Triggers, constraints, rollback, reportes contra SQLite real |
| Extremo a extremo | `adapters/http/http_test.go` | HTTP → servicios → SQLite, códigos de error, cabeceras |
| UI en navegador real | `scripts/screenshots/capture.mjs` | Flujos de la interfaz con Playwright, sin errores de JS (y genera las capturas) |

Cobertura actual: dominio ~97 %, servicios ~91 %, HTTP ~88 %, SQLite ~76 %.

## Despliegue

| Forma | Comando | Ideal para |
|---|---|---|
| Docker Compose | `docker compose up -d --build` | Servidores, VPS |
| Imagen publicada | `docker run -p 8080:8080 -v inv:/data ghcr.io/mainrdsniper09/inventory-go` | Servidores |
| Binario | Descargar de *Releases* | PC del negocio, sin instalar nada |
| Compilar | `make build` → `bin/inventory` | Desarrollo |

La imagen es **distroless** (sin shell), corre como **usuario no-root**, con **sistema de archivos de sólo lectura** en Compose y `HEALTHCHECK` integrado.

### Publicar una versión: cerrar el milestone

Cada **milestone** es una versión (`v0.2.0 Seguridad y usuarios`). **Al cerrarlo en GitHub se publica la release automáticamente**:

```mermaid
flowchart LR
  A["Cerrar milestone<br/>v0.2.0 Seguridad y usuarios"] --> B{¿Issues abiertos?}
  B -- sí --> R["Se reabre el milestone<br/>y se listan los pendientes"]
  B -- no --> C["Tag v0.2.0 en main"]
  C --> D["GoReleaser<br/>binarios Linux/Windows/macOS<br/>amd64 + arm64"]
  C --> E["Imagen Docker multi-arch<br/>ghcr.io · :0.2.0 :0.2 :latest"]
  D --> F["Release 'v0.2.0 — Seguridad y usuarios'<br/>notas: issues por tipo + PRs"]
```

También funciona `git tag vX.Y.Z && git push --tags`, y desde *Actions → Milestone release → Run workflow* se puede reintentar una release fallida. Detalle en [CONTRIBUTING](CONTRIBUTING.md#publicar-una-versión).

📘 Decisiones: [ADR-0004](docs/adr/0004-distribucion-binario-y-docker.md) · [ADR-0005](docs/adr/0005-release-por-milestone.md)

## Roadmap

`v0.1.0 MVP` ✅ → `v0.2.0 Seguridad y usuarios` → `v0.3.0 Compras y proveedores` → `v0.4.0 Operación (multi-almacén, códigos de barras)` → `v0.5.0 Analítica` → `v1.0.0 Producción`

📘 Detalle: **[docs/ROADMAP.md](docs/ROADMAP.md)** · 📋 **[Tablero del proyecto](https://github.com/users/MainRDSniper09/projects/3)** · Seguimiento en [Issues](https://github.com/MainRDSniper09/Inventory-Project-Go/issues) y [Milestones](https://github.com/MainRDSniper09/Inventory-Project-Go/milestones).

## Decisiones de arquitectura (ADR)

| ADR | Decisión |
|---|---|
| [0001](docs/adr/0001-arquitectura-hexagonal.md) | Arquitectura hexagonal con DI manual |
| [0002](docs/adr/0002-sqlite-go-puro.md) | SQLite con driver Go puro (sin CGO) |
| [0003](docs/adr/0003-triggers-en-lugar-de-stored-procedures.md) | Triggers y vistas como equivalente a stored procedures |
| [0004](docs/adr/0004-distribucion-binario-y-docker.md) | Distribución como binario único y Docker |
| [0005](docs/adr/0005-release-por-milestone.md) | Una release por milestone, publicada al cerrarlo |

## Contribuir

Lee **[CONTRIBUTING.md](CONTRIBUTING.md)**. Issues etiquetados `good first issue` son ideales para empezar. Para vulnerabilidades, ver [SECURITY.md](SECURITY.md).

> ⚠️ El MVP **no tiene autenticación**: úsalo en local o en una red de confianza (planificado para v0.2.0).

## Licencia

[MIT](LICENSE) © 2026 Juan Barreto
