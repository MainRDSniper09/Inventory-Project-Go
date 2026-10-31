package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// StockRepository implementa ports.StockRepository.
type StockRepository struct{ db *sql.DB }

var _ ports.StockRepository = (*StockRepository)(nil)

// NewStockRepository construye el repositorio.
func NewStockRepository(db *sql.DB) *StockRepository { return &StockRepository{db: db} }

// Get devuelve la existencia actual de un producto.
func (r *StockRepository) Get(ctx context.Context, productID int64) (domain.Stock, error) {
	var (
		s       domain.Stock
		updated string
	)
	err := conn(ctx, r.db).QueryRowContext(ctx,
		`SELECT product_id, quantity, updated_at FROM inventory WHERE product_id = ?`, productID).
		Scan(&s.ProductID, &s.Quantity, &updated)
	if err != nil {
		return s, mapError(err)
	}
	if s.UpdatedAt, err = parseTime(updated); err != nil {
		return s, err
	}
	return s, nil
}

// RecordMovement inserta un asiento en el kardex. El trigger
// trg_stock_movements_ai_apply actualiza la tabla inventory.
func (r *StockRepository) RecordMovement(ctx context.Context, m *domain.StockMovement) error {
	res, err := conn(ctx, r.db).ExecContext(ctx, `
		INSERT INTO stock_movements (product_id, type, delta, reason, sale_item_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.ProductID, string(m.Type), m.Delta, m.Reason, nullableID(m.SaleItemID), formatTime(m.CreatedAt))
	if err != nil {
		return mapError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("movement last insert id: %w", err)
	}
	m.ID = id
	return nil
}

// ListMovements devuelve los movimientos más recientes primero.
func (r *StockRepository) ListMovements(ctx context.Context, productID int64, limit int) ([]domain.StockMovement, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT id, product_id, type, delta, reason, sale_item_id, created_at
		  FROM stock_movements
		 WHERE product_id = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`, productID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]domain.StockMovement, 0)
	for rows.Next() {
		var (
			m       domain.StockMovement
			typ     string
			saleIt  sql.NullInt64
			created string
		)
		if err := rows.Scan(&m.ID, &m.ProductID, &typ, &m.Delta, &m.Reason, &saleIt, &created); err != nil {
			return nil, mapError(err)
		}
		m.Type = domain.MovementType(typ)
		m.SaleItemID = idPtr(saleIt)
		if m.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
