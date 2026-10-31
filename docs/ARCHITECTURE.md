# Arquitectura

> Documento vivo. Si cambias la arquitectura, actualiza este archivo y registra la decisión en [`docs/adr/`](adr/).

## Índice

1. [Arquitectura Hexagonal en 5 minutos](#1-arquitectura-hexagonal-en-5-minutos)
2. [Mapa del código](#2-mapa-del-código)
3. [Diagrama de componentes (hexágono)](#3-diagrama-de-componentes-hexágono)
4. [Diagrama de clases](#4-diagrama-de-clases)
5. [Flujo de una petición](#5-flujo-de-una-petición)
6. [Secuencia: registrar una venta](#6-secuencia-registrar-una-venta)
7. [Principios SOLID aplicados](#7-principios-solid-aplicados)
8. [Estrategia de pruebas](#8-estrategia-de-pruebas)
9. [Despliegue](#9-despliegue)

---

## 1. Arquitectura Hexagonal en 5 minutos

La **Arquitectura Hexagonal** (o *Ports & Adapters*, Alistair Cockburn, 2005) separa el software en tres anillos:

| Anillo | Qué contiene | En este proyecto | ¿De qué depende? |
|---|---|---|---|
| **Dominio** | Entidades, value objects, reglas de negocio puras | `internal/core/domain` | De **nada** |
| **Aplicación** | Casos de uso y **puertos** (interfaces) | `internal/core/services`, `internal/core/ports` | Sólo del dominio |
| **Adaptadores** | Tecnología concreta: HTTP, SQL, CLI… | `internal/adapters/*` | Del núcleo (implementan/usan sus puertos) |

**La regla de oro: las dependencias siempre apuntan hacia adentro.** El dominio nunca importa `database/sql` ni `net/http`.

Hay dos tipos de puertos:

- **Puertos de entrada (driving / primarios)** — lo que la aplicación *ofrece*: `ProductService`, `SaleService`… Los **usa** un adaptador primario (el servidor HTTP).
- **Puertos de salida (driven / secundarios)** — lo que la aplicación *necesita*: `ProductRepository`, `TxManager`, `Clock`… Los **implementa** un adaptador secundario (SQLite).

¿Qué ganamos?

- **Testabilidad**: los servicios se prueban con *fakes* en memoria, sin BD (ver `services/fakes_test.go`).
- **Reemplazabilidad**: cambiar SQLite → PostgreSQL = escribir `internal/adapters/postgres` y cambiar 1 línea en `main.go`. El núcleo no se toca.
- **Claridad**: la lógica de negocio no está mezclada con detalles de JSON ni de SQL.

## 2. Mapa del código

```text
cmd/inventory/            ← Composition root: arma el grafo de dependencias (main.go, seed.go)
internal/
├── core/                 ← EL HEXÁGONO (sin dependencias de infraestructura)
│   ├── domain/           ← Entidades y reglas: Product, Sale, StockMovement, Money…
│   ├── ports/            ← Interfaces: inbound.go (casos de uso), outbound.go (repos, tx, reloj)
│   └── services/         ← Implementación de los casos de uso
├── adapters/
│   ├── http/             ← Adaptador PRIMARIO: API REST + UI web embebida
│   └── sqlite/           ← Adaptador SECUNDARIO: repositorios + migraciones SQL
└── platform/             ← Utilidades transversales: config (env vars), clock
```

## 3. Diagrama de componentes (hexágono)

```mermaid
flowchart LR
    subgraph Primarios["Adaptadores primarios (driving)"]
        UI["Navegador<br/>UI web embebida"]
        CLI["curl / Postman<br/>clientes REST"]
        HTTP["adapters/http<br/>Handler + DTOs + middleware"]
    end

    subgraph Core["Núcleo (hexágono)"]
        direction TB
        IN["ports/inbound.go<br/>ProductService · SaleService<br/>InventoryService · ReportService<br/>CategoryService"]
        SVC["services/<br/>implementación de casos de uso"]
        DOM["domain/<br/>Product · Sale · StockMovement<br/>Money · DateRange · errores"]
        OUT["ports/outbound.go<br/>*Repository · TxManager · Clock"]
        IN --> SVC
        SVC --> DOM
        SVC --> OUT
    end

    subgraph Secundarios["Adaptadores secundarios (driven)"]
        SQL["adapters/sqlite<br/>repositorios + TxManager"]
        CLK["platform/clock"]
        DB[("SQLite<br/>tablas · triggers · vistas")]
    end

    UI --> HTTP
    CLI --> HTTP
    HTTP -- "usa" --> IN
    SQL -. "implementa" .-> OUT
    CLK -. "implementa" .-> OUT
    SQL --> DB
```

## 4. Diagrama de clases

En Go no hay clases, pero `struct` + métodos + interfaces cumplen ese papel. Las flechas punteadas `..|>` indican "implementa".

```mermaid
classDiagram
    direction TB

    class Money {
        <<value object>>
        int64 cents
        +Cents() int64
        +Multiply(qty int) Money
        +String() string
    }

    class Category {
        +int64 ID
        +string Name
        +string Description
        +time CreatedAt
    }

    class Product {
        <<entity>>
        +int64 ID
        +string SKU
        +string Name
        +string Description
        +int64* CategoryID
        +Money Price
        +bool Active
        +Update(ProductDetails, time) error
    }

    class ProductWithStock {
        <<read model>>
        +Product
        +string CategoryName
        +int Quantity
    }

    class Stock {
        +int64 ProductID
        +int Quantity
        +CanFulfill(qty int) bool
    }

    class StockMovement {
        <<entity · inmutable>>
        +int64 ID
        +int64 ProductID
        +MovementType Type
        +int Delta
        +string Reason
        +int64* SaleItemID
    }

    class MovementType {
        <<enumeration>>
        IN
        OUT
        ADJUST
        SALE
    }

    class Sale {
        <<aggregate root>>
        +int64 ID
        +string Notes
        +[]SaleItem Items
        +AddItem(productID, qty, price) error
        +Total() Money
        +UnitsSold() int
        +Validate() error
    }

    class SaleItem {
        +int64 ProductID
        +int Quantity
        +Money UnitPrice
        +Subtotal() Money
    }

    Product --> Money
    Product "0..*" --> "0..1" Category
    ProductWithStock --|> Product
    StockMovement --> MovementType
    Sale "1" *-- "1..100" SaleItem
    SaleItem --> Money
    Stock "1" -- "1" Product
    StockMovement "0..*" --> "1" Product

    class ProductRepository {
        <<interface · puerto salida>>
        +Create(ctx, *Product) error
        +Update(ctx, *Product) error
        +GetByID(ctx, id) *Product
        +GetWithStock(ctx, id) *ProductWithStock
        +List(ctx, ProductFilter) []ProductWithStock
        +Archive(ctx, id, time) error
    }
    class StockRepository {
        <<interface · puerto salida>>
        +Get(ctx, productID) Stock
        +RecordMovement(ctx, *StockMovement) error
        +ListMovements(ctx, productID, limit)
    }
    class SaleRepository {
        <<interface · puerto salida>>
        +Create(ctx, *Sale) error
        +GetByID(ctx, id) *Sale
        +List(ctx, DateRange, limit) []Sale
    }
    class TxManager {
        <<interface · puerto salida>>
        +WithinTx(ctx, fn) error
    }

    class SaleServicePort {
        <<interface · puerto entrada>>
        +Register(ctx, RegisterSaleCmd) *Sale
        +Get(ctx, id) *Sale
        +List(ctx, DateRange, limit) []Sale
    }

    class SaleService {
        <<caso de uso>>
        -ProductRepository products
        -StockRepository stock
        -SaleRepository sales
        -TxManager tx
        -Clock clock
    }

    SaleService ..|> SaleServicePort
    SaleService --> ProductRepository
    SaleService --> StockRepository
    SaleService --> SaleRepository
    SaleService --> TxManager
    SaleService ..> Sale : crea

    class SQLiteSaleRepository {
        <<adapter>>
        -sql.DB db
    }
    class SQLiteTxManager {
        <<adapter>>
        -sql.DB db
    }
    class HTTPHandler {
        <<adapter primario>>
        -Services svc
        +registerSale(w, r)
    }

    SQLiteSaleRepository ..|> SaleRepository
    SQLiteTxManager ..|> TxManager
    HTTPHandler --> SaleServicePort
```

> El diagrama muestra en detalle la rebanada **Ventas**; `ProductService`, `InventoryService`, `CategoryService` y `ReportService` siguen exactamente el mismo patrón.

## 5. Flujo de una petición

```mermaid
flowchart TD
    A([Cliente HTTP]) --> B[withRecover<br/>captura panics]
    B --> C[withLogging<br/>método · ruta · status · ms]
    C --> D[withSecurityHeaders<br/>CSP · nosniff · DENY]
    D --> E{ServeMux<br/>GET/POST /api/v1/...}
    E -->|ruta estática| S[UI embebida<br/>index.html · app.js]
    E -->|ruta API| F[Handler<br/>parsea JSON → Cmd]
    F -->|JSON inválido| X[writeError 400]
    F --> G[Servicio<br/>caso de uso]
    G --> H[Entidad de dominio<br/>valida invariantes]
    H -->|ValidationError| X
    G --> I[TxManager.WithinTx]
    I --> J[Repositorio SQLite]
    J --> K[(SQLite<br/>triggers + CHECK)]
    K -->|constraint| L[mapError<br/>→ error de dominio]
    L --> X
    J --> M[Resultado]
    M --> N[DTO → JSON 2xx]
    X --> O([Respuesta])
    N --> O
```

**Traducción de errores** — cada capa habla su propio idioma:

| Origen | Error | Traducido a (dominio) | HTTP |
|---|---|---|---|
| Dominio | `ValidationError{Field}` | — | `400 invalid_input` |
| SQLite | `UNIQUE constraint` | `ErrConflict` | `409 conflict` |
| SQLite | `FOREIGN KEY constraint` | `ErrInvalidInput` | `400 invalid_input` |
| SQLite | `CHECK chk_inventory_non_negative` | `ErrInsufficientStock` | `409 insufficient_stock` |
| SQLite | `sql.ErrNoRows` | `ErrNotFound` | `404 not_found` |
| Cualquier otro | — | — | `500 internal` (detalle sólo en logs) |

## 6. Secuencia: registrar una venta

El caso de uso más interesante, porque combina validación en dominio, transacción y triggers.

```mermaid
sequenceDiagram
    autonumber
    actor U as Usuario
    participant H as HTTP Handler
    participant S as SaleService
    participant D as domain.Sale
    participant T as TxManager
    participant PR as ProductRepository
    participant SR as StockRepository
    participant SaR as SaleRepository
    participant DB as SQLite

    U->>H: POST /api/v1/sales {items:[{product_id, quantity}]}
    H->>H: decodeJSON (límite 1 MiB, sin campos desconocidos)
    H->>S: Register(ctx, RegisterSaleCmd)
    S->>D: NewSale(notes, clock.Now())
    S->>T: WithinTx(ctx, fn)
    T->>DB: BEGIN IMMEDIATE (lock de escritura)
    loop por cada línea
        S->>PR: GetByID(productID)
        PR-->>S: Product (precio vigente)
        S->>D: AddItem(id, qty, product.Price)
        Note right of D: fusiona líneas repetidas
    end
    S->>D: Validate()
    loop por cada item consolidado
        S->>SR: Get(productID)
        SR-->>S: Stock
        alt stock insuficiente
            S-->>T: StockError
            T->>DB: ROLLBACK
            H-->>U: 409 insufficient_stock
        end
    end
    S->>SaR: Create(sale)
    SaR->>DB: INSERT INTO sales
    loop por cada item
        SaR->>DB: INSERT INTO sale_items
        Note over DB: trg_sale_items_ai_register<br/>1) sales.total_cents += subtotal<br/>2) INSERT stock_movements (SALE, -qty)
        Note over DB: trg_stock_movements_ai_apply<br/>inventory.quantity += delta<br/>(CHECK quantity >= 0)
    end
    T->>DB: COMMIT
    S-->>H: *Sale
    H-->>U: 201 Created + Location: /api/v1/sales/{id}
```

**Defensa en profundidad**: el stock se valida dos veces. El servicio lo verifica para dar un mensaje de error claro; la BD lo garantiza con un `CHECK` como última línea de defensa. Además, `_txlock=immediate` hace que la transacción tome el lock de escritura desde el `BEGIN`, evitando que dos ventas concurrentes lean el mismo stock.

## 7. Principios SOLID aplicados

| Principio | Dónde se ve |
|---|---|
| **S** — Responsabilidad única | `Product` no conoce su stock (eso es `Stock`/`StockMovement`). `writeError` es el único lugar que mapea errores→HTTP. `mapError` es el único que mapea SQLite→dominio. |
| **O** — Abierto/cerrado | Nuevos adaptadores (PostgreSQL, gRPC, CLI) se agregan sin modificar el núcleo. Nuevas migraciones se agregan como archivos nuevos, nunca editando las anteriores. |
| **L** — Sustitución de Liskov | Los fakes de prueba y los repositorios SQLite son intercambiables: ambos cumplen el mismo contrato y los servicios no notan la diferencia. |
| **I** — Segregación de interfaces | En vez de un `Repository` gigante hay 5 interfaces pequeñas. `InventoryService` sólo recibe `ProductRepository` y `StockRepository`; no puede tocar ventas. |
| **D** — Inversión de dependencias | Los servicios dependen de `ports.*` (abstracciones). `main.go` es el único que conoce `sqlite.*`. Las aserciones `var _ ports.X = (*Y)(nil)` verifican el contrato al compilar. |

## 8. Estrategia de pruebas

```mermaid
flowchart BT
    U["Unitarias de dominio<br/>domain_test.go · sin I/O · ~0 ms"]
    S["Unitarias de casos de uso<br/>services_test.go · fakes en memoria"]
    I["Integración del adaptador<br/>sqlite_test.go · BD real temporal<br/>valida triggers, CHECK, rollback"]
    E["Extremo a extremo<br/>http_test.go · HTTP → servicios → SQLite"]
    U --> S --> I --> E
```

- Cada prueba de integración crea su propia BD en `t.TempDir()` → aisladas y paralelizables.
- `Clock` inyectado → fechas deterministas.
- Las pruebas de HTTP verifican que los errores internos **no** filtran detalles al cliente.

```bash
go test ./...            # todo
go test -cover ./...     # con cobertura
make cover               # reporte HTML
```

## 9. Despliegue

```mermaid
flowchart LR
    subgraph Dev["Desarrollador"]
        SRC[Código fuente]
    end
    subgraph GH["GitHub"]
        CI["Actions CI<br/>vet · test (Linux/Win/macOS)<br/>lint · docker smoke test"]
        MS["Milestone cerrado<br/>vX.Y.Z ..."]
        REL["Actions Release<br/>(reutilizable)"]
        GR["GoReleaser<br/>binarios linux/win/mac<br/>amd64 + arm64"]
        GHCR["ghcr.io<br/>imagen multi-arch"]
    end
    subgraph Run["Ejecución"]
        BIN["Binario único<br/>./inventory"]
        DOCK["Contenedor distroless<br/>usuario nonroot · volumen /data"]
    end
    SRC -- push/PR --> CI
    MS -- "valida + crea tag" --> REL
    SRC -. "git tag (hotfix)" .-> REL
    REL --> GR --> BIN
    REL --> GHCR --> DOCK
```

Dos formas de distribución, ambas generadas desde el mismo código:

1. **Binario único** (~12 MB): UI, migraciones y driver SQLite embebidos. Sin dependencias: descargar y ejecutar.
2. **Imagen Docker** distroless (sin shell), usuario no-root, `HEALTHCHECK` integrado y volumen para la BD.
