-- =============================================================================
-- 0001_init.sql  ·  Esquema inicial del inventario (MVP v0.1.0)
-- =============================================================================
-- Convenciones:
--   * Dinero en CENTAVOS (INTEGER), nunca REAL.
--   * Fechas en TEXT ISO-8601 UTC: 'YYYY-MM-DDTHH:MM:SS.sssZ' (ordenables).
--   * SQLite NO tiene Stored Procedures. Su equivalente idiomático es:
--       - TRIGGERS  -> lógica transaccional que corre dentro de la BD.
--       - VIEWS     -> consultas reutilizables (read models).
--       - CHECK/FK  -> invariantes que la BD garantiza siempre.
-- =============================================================================

-- ---------------------------------------------------------------- categorías
CREATE TABLE categories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL
);

-- ---------------------------------------------------------------- productos
CREATE TABLE products (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    sku         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    category_id INTEGER REFERENCES categories (id) ON DELETE SET NULL,
    price_cents INTEGER NOT NULL CHECK (price_cents >= 0),
    active      INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);
CREATE INDEX idx_products_category ON products (category_id);
CREATE INDEX idx_products_name     ON products (name);

-- ---------------------------------------------------------------- inventario
-- Proyección 1:1 con products. Su cantidad la mantiene un trigger a partir
-- del libro stock_movements; la aplicación NUNCA la escribe directamente.
CREATE TABLE inventory (
    product_id INTEGER PRIMARY KEY REFERENCES products (id),
    quantity   INTEGER NOT NULL DEFAULT 0
               CONSTRAINT chk_inventory_non_negative CHECK (quantity >= 0),
    updated_at TEXT    NOT NULL
);

-- ---------------------------------------------------------------- ventas
CREATE TABLE sales (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    notes       TEXT    NOT NULL DEFAULT '',
    total_cents INTEGER NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    created_at  TEXT    NOT NULL
);
CREATE INDEX idx_sales_created_at ON sales (created_at);

CREATE TABLE sale_items (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    sale_id          INTEGER NOT NULL REFERENCES sales (id),
    product_id       INTEGER NOT NULL REFERENCES products (id),
    quantity         INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents INTEGER NOT NULL CHECK (unit_price_cents >= 0),
    subtotal_cents   INTEGER GENERATED ALWAYS AS (quantity * unit_price_cents) STORED,
    UNIQUE (sale_id, product_id)
);
CREATE INDEX idx_sale_items_product ON sale_items (product_id);

-- ---------------------------------------------------------------- kardex
-- Libro contable de inventario (append-only). Cada fila es un hecho inmutable.
CREATE TABLE stock_movements (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id   INTEGER NOT NULL REFERENCES products (id),
    type         TEXT    NOT NULL CHECK (type IN ('IN', 'OUT', 'ADJUST', 'SALE')),
    delta        INTEGER NOT NULL CHECK (delta <> 0),
    reason       TEXT    NOT NULL DEFAULT '',
    sale_item_id INTEGER REFERENCES sale_items (id),
    created_at   TEXT    NOT NULL,
    CONSTRAINT chk_movement_sign CHECK (
        (type = 'IN' AND delta > 0)
        OR (type IN ('OUT', 'SALE') AND delta < 0)
        OR type = 'ADJUST'
    )
);
CREATE INDEX idx_movements_product ON stock_movements (product_id, created_at);

-- =============================================================================
-- TRIGGERS  (el equivalente a Stored Procedures en SQLite)
-- =============================================================================

-- sp_create_inventory: todo producto nace con una fila de inventario en 0.
CREATE TRIGGER trg_products_ai_create_inventory
AFTER INSERT ON products
BEGIN
    INSERT INTO inventory (product_id, quantity, updated_at)
    VALUES (NEW.id, 0, NEW.created_at);
END;

-- sp_apply_movement: cada asiento del kardex actualiza la existencia.
-- Si el resultado fuese negativo, chk_inventory_non_negative aborta TODO.
CREATE TRIGGER trg_stock_movements_ai_apply
AFTER INSERT ON stock_movements
BEGIN
    UPDATE inventory
       SET quantity   = quantity + NEW.delta,
           updated_at = NEW.created_at
     WHERE product_id = NEW.product_id;
END;

-- sp_register_sale_item: al vender, acumula el total de la venta y genera el
-- movimiento SALE (que a su vez dispara trg_stock_movements_ai_apply).
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

-- El kardex es inmutable: para corregir se registra un ADJUST, no se edita.
CREATE TRIGGER trg_stock_movements_bu_immutable
BEFORE UPDATE ON stock_movements
BEGIN
    SELECT RAISE(ABORT, 'stock_movements is append-only');
END;

CREATE TRIGGER trg_stock_movements_bd_immutable
BEFORE DELETE ON stock_movements
BEGIN
    SELECT RAISE(ABORT, 'stock_movements is append-only');
END;

-- Las líneas de venta tampoco se modifican: alterarían el kardex.
CREATE TRIGGER trg_sale_items_bu_immutable
BEFORE UPDATE ON sale_items
BEGIN
    SELECT RAISE(ABORT, 'sale_items are immutable');
END;

-- =============================================================================
-- VISTAS  (consultas reutilizables / read models)
-- =============================================================================

CREATE VIEW v_product_stock AS
SELECT p.id,
       p.sku,
       p.name,
       p.description,
       p.category_id,
       COALESCE(c.name, '')     AS category_name,
       p.price_cents,
       p.active,
       p.created_at,
       p.updated_at,
       COALESCE(i.quantity, 0)  AS quantity
  FROM products p
  LEFT JOIN categories c ON c.id = p.category_id
  LEFT JOIN inventory  i ON i.product_id = p.id;

CREATE VIEW v_sales_lines AS
SELECT s.id              AS sale_id,
       s.created_at      AS sold_at,
       p.id              AS product_id,
       p.sku             AS sku,
       p.name            AS product_name,
       si.quantity       AS quantity,
       si.unit_price_cents,
       si.subtotal_cents
  FROM sale_items si
  JOIN sales    s ON s.id = si.sale_id
  JOIN products p ON p.id = si.product_id;
