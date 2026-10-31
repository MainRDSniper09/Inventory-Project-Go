# ADR-0003: Triggers y vistas como equivalente a Stored Procedures

- **Estado:** Aceptado
- **Fecha:** 2026-10-31

## Contexto

Se pidió que la base de datos tuviera "SPs" (stored procedures). **SQLite no implementa `CREATE PROCEDURE`**: es una BD embebida y espera que la lógica procedural viva en la aplicación anfitriona.

Aun así, queremos que la BD garantice ciertas invariantes aunque alguien la manipule directamente (scripts, herramientas de admin, bugs).

## Decisión

Usamos los mecanismos nativos de SQLite que cumplen el mismo rol:

1. **Triggers** para lógica transaccional de *integridad de datos*:
   - mantener la proyección `inventory` a partir del kardex `stock_movements`;
   - acumular `sales.total_cents`;
   - generar el movimiento `SALE` al vender;
   - hacer inmutables el kardex y las líneas de venta.
2. **Vistas** (`v_product_stock`, `v_sales_lines`) como consultas reutilizables para lectura.
3. **CHECK / FK / UNIQUE / columnas generadas** para invariantes declarativas.

Las **reglas de negocio** (producto archivado no se vende, motivo obligatorio en ajustes, precio tomado del catálogo) permanecen en el dominio Go.

## Consecuencias

- ✅ El stock no puede quedar negativo ni el kardex alterarse, ni siquiera con SQL manual.
- ✅ Las pruebas de integración verifican los triggers contra SQLite real.
- ⚠️ La lógica en triggers es menos visible; se documenta en `docs/DATABASE.md` y en comentarios del SQL.
- ⚠️ Un futuro adaptador PostgreSQL debe replicar estas invariantes (allí sí podrían ser funciones/procedimientos PL/pgSQL).
