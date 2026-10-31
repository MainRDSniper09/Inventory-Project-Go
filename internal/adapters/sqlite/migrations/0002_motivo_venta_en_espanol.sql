-- =============================================================================
-- 0002_motivo_venta_en_espanol.sql
-- =============================================================================
-- El trigger de 0001 escribía el motivo del movimiento como 'sale #N'.
-- Lo pasamos a español ('venta #N') para que el kardex sea consistente.
--
-- Ejemplo didáctico: 0001 ya está publicada (otras BDs la tienen aplicada y
-- registrada en schema_migrations), por eso NO se edita. Toda corrección de
-- esquema va en una migración nueva. SQLite no tiene CREATE OR REPLACE TRIGGER,
-- así que se elimina y se vuelve a crear dentro de la misma transacción.
-- Los movimientos históricos conservan su texto original (el kardex es inmutable).
-- =============================================================================

DROP TRIGGER trg_sale_items_ai_register;

CREATE TRIGGER trg_sale_items_ai_register
AFTER INSERT ON sale_items
BEGIN
    UPDATE sales
       SET total_cents = total_cents + NEW.quantity * NEW.unit_price_cents
     WHERE id = NEW.sale_id;

    INSERT INTO stock_movements (product_id, type, delta, reason, sale_item_id, created_at)
    VALUES (NEW.product_id, 'SALE', -NEW.quantity, 'venta #' || NEW.sale_id, NEW.id,
            (SELECT created_at FROM sales WHERE id = NEW.sale_id));
END;
