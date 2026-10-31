# API REST v1

Base: `http://localhost:8080/api/v1` · Formato: JSON · Dinero: **centavos** (`price_cents: 1850` = $18.50).

## Errores

Todas las respuestas de error tienen la misma forma:

```json
{ "error": { "code": "invalid_input", "message": "is required", "field": "name" } }
```

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `invalid_input` | Validación fallida, JSON mal formado o campo desconocido |
| 404 | `not_found` | El recurso no existe |
| 409 | `conflict` | SKU o categoría duplicados |
| 409 | `insufficient_stock` | La operación dejaría stock negativo |
| 500 | `internal` | Error inesperado (detalle sólo en logs del servidor) |

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/healthz` | Estado del servicio |
| `GET` | `/categories` | Lista categorías |
| `POST` | `/categories` | Crea categoría |
| `GET` | `/products?q=&category_id=&include_archived=&limit=&offset=` | Lista/busca productos |
| `POST` | `/products` | Crea producto (opcional `initial_stock`) |
| `GET` | `/products/{id}` | Obtiene producto con existencia |
| `PUT` | `/products/{id}` | Actualiza nombre, descripción, categoría, precio |
| `DELETE` | `/products/{id}` | Archiva (borrado lógico) |
| `GET` | `/products/{id}/movements?limit=` | Kardex del producto |
| `POST` | `/products/{id}/movements` | Entrada / salida / ajuste |
| `GET` | `/sales?from=&to=&limit=` | Lista ventas |
| `POST` | `/sales` | Registra venta |
| `GET` | `/sales/{id}` | Detalle de venta |
| `GET` | `/dashboard?from=&to=&low_stock=&top=` | KPIs del tablero |
| `GET` | `/reports/sales?from=&to=&format=json\|csv` | Reporte de ventas |

Fechas `from`/`to`: `YYYY-MM-DD`, **ambas inclusivas**, en UTC. Por defecto: últimos 30 días.

## Ejemplos

```bash
# Crear categoría
curl -s -X POST localhost:8080/api/v1/categories \
  -H 'Content-Type: application/json' -d '{"name":"Bebidas"}'

# Crear producto con 24 unidades iniciales
curl -s -X POST localhost:8080/api/v1/products \
  -H 'Content-Type: application/json' \
  -d '{"sku":"BEB-COLA-600","name":"Refresco cola 600ml","category_id":1,"price_cents":1800,"initial_stock":24}'

# Entrada de mercancía
curl -s -X POST localhost:8080/api/v1/products/1/movements \
  -H 'Content-Type: application/json' -d '{"type":"IN","quantity":12,"reason":"compra proveedor"}'

# Ajuste por conteo físico (-2), el motivo es obligatorio
curl -s -X POST localhost:8080/api/v1/products/1/movements \
  -H 'Content-Type: application/json' -d '{"type":"ADJUST","quantity":-2,"reason":"conteo mensual"}'

# Registrar venta (el precio lo toma el servidor, no el cliente)
curl -s -X POST localhost:8080/api/v1/sales \
  -H 'Content-Type: application/json' \
  -d '{"notes":"mostrador","items":[{"product_id":1,"quantity":3}]}'

# Dashboard de octubre
curl -s 'localhost:8080/api/v1/dashboard?from=2026-10-01&to=2026-10-31'

# Descargar reporte CSV
curl -s -OJ 'localhost:8080/api/v1/reports/sales?from=2026-10-01&to=2026-10-31&format=csv'
```

### Respuesta de `GET /dashboard`

```json
{
  "from": "2026-10-02T00:00:00Z",
  "to": "2026-11-01T00:00:00Z",
  "summary": {
    "active_products": 5, "units_in_stock": 140,
    "inventory_value_cents": 242500, "inventory_value": "2425.00",
    "sales_count": 3, "units_sold": 14,
    "revenue_cents": 23700, "revenue": "237.00",
    "low_stock_threshold": 5, "low_stock_count": 1
  },
  "top_products": [{ "product_id": 2, "sku": "BEB-AGUA-1L", "name": "Agua natural 1L", "units_sold": 6, "revenue_cents": 7200 }],
  "daily": [{ "day": "2026-10-31", "sales_count": 3, "units_sold": 14, "revenue_cents": 23700 }],
  "low_stock": [{ "id": 4, "sku": "SNK-GALL-CHOC", "quantity": 4, "...": "..." }]
}
```
