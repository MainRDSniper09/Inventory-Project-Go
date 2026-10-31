package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// ProductRepository implementa ports.ProductRepository.
type ProductRepository struct{ db *sql.DB }

var _ ports.ProductRepository = (*ProductRepository)(nil)

// NewProductRepository construye el repositorio.
func NewProductRepository(db *sql.DB) *ProductRepository { return &ProductRepository{db: db} }

const productStockColumns = `id, sku, name, description, category_id, category_name,
	price_cents, active, created_at, updated_at, quantity`

// Create inserta el producto. El trigger trg_products_ai_create_inventory
// crea automáticamente su fila de inventario.
func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	res, err := conn(ctx, r.db).ExecContext(ctx, `
		INSERT INTO products (sku, name, description, category_id, price_cents, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.SKU, p.Name, p.Description, nullableID(p.CategoryID), p.Price.Cents(), boolToInt(p.Active),
		formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return mapError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("product last insert id: %w", err)
	}
	p.ID = id
	return nil
}

// Update guarda los campos editables.
func (r *ProductRepository) Update(ctx context.Context, p *domain.Product) error {
	res, err := conn(ctx, r.db).ExecContext(ctx, `
		UPDATE products
		   SET name = ?, description = ?, category_id = ?, price_cents = ?, updated_at = ?
		 WHERE id = ?`,
		p.Name, p.Description, nullableID(p.CategoryID), p.Price.Cents(), formatTime(p.UpdatedAt), p.ID)
	if err != nil {
		return mapError(err)
	}
	return expectAffected(res)
}

// GetByID devuelve la entidad Product.
func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	row := conn(ctx, r.db).QueryRowContext(ctx, `
		SELECT id, sku, name, description, category_id, price_cents, active, created_at, updated_at
		  FROM products WHERE id = ?`, id)
	var (
		p                domain.Product
		catID            sql.NullInt64
		price            int64
		active           int
		created, updated string
	)
	if err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &catID, &price, &active, &created, &updated); err != nil {
		return nil, mapError(err)
	}
	p.CategoryID = idPtr(catID)
	p.Price = domain.Money(price)
	p.Active = active == 1
	var err error
	if p.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if p.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetWithStock lee de la vista v_product_stock.
func (r *ProductRepository) GetWithStock(ctx context.Context, id int64) (*domain.ProductWithStock, error) {
	row := conn(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+productStockColumns+` FROM v_product_stock WHERE id = ?`, id)
	p, err := scanProductStock(row)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List busca productos con filtros y paginación. Los parámetros siempre van
// como placeholders (?) para prevenir inyección SQL.
func (r *ProductRepository) List(ctx context.Context, f ports.ProductFilter) ([]domain.ProductWithStock, error) {
	var (
		where []string
		args  []any
	)
	if !f.IncludeArchived {
		where = append(where, "active = 1")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "(sku LIKE ? ESCAPE '\\' OR name LIKE ? ESCAPE '\\')")
		like := "%" + escapeLike(q) + "%"
		args = append(args, like, like)
	}
	if f.CategoryID != nil {
		where = append(where, "category_id = ?")
		args = append(args, *f.CategoryID)
	}
	query := `SELECT ` + productStockColumns + ` FROM v_product_stock`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY name COLLATE NOCASE, id LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := conn(ctx, r.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return collectProductStock(rows)
}

// Archive desactiva el producto (borrado lógico).
func (r *ProductRepository) Archive(ctx context.Context, id int64, at time.Time) error {
	res, err := conn(ctx, r.db).ExecContext(ctx,
		`UPDATE products SET active = 0, updated_at = ? WHERE id = ? AND active = 1`, formatTime(at), id)
	if err != nil {
		return mapError(err)
	}
	return expectAffected(res)
}

// ---------------------------------------------------------------- helpers

type rowScanner interface{ Scan(dest ...any) error }

func scanProductStock(s rowScanner) (domain.ProductWithStock, error) {
	var (
		p                domain.ProductWithStock
		catID            sql.NullInt64
		price            int64
		active           int
		created, updated string
	)
	if err := s.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &catID, &p.CategoryName,
		&price, &active, &created, &updated, &p.Quantity); err != nil {
		return p, mapError(err)
	}
	p.CategoryID = idPtr(catID)
	p.Price = domain.Money(price)
	p.Active = active == 1
	var err error
	if p.CreatedAt, err = parseTime(created); err != nil {
		return p, err
	}
	if p.UpdatedAt, err = parseTime(updated); err != nil {
		return p, err
	}
	return p, nil
}

func collectProductStock(rows *sql.Rows) ([]domain.ProductWithStock, error) {
	out := make([]domain.ProductWithStock, 0)
	for rows.Next() {
		p, err := scanProductStock(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func expectAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func nullableID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

func idPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
