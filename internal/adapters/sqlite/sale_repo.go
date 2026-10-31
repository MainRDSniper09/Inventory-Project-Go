package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// SaleRepository implementa ports.SaleRepository.
type SaleRepository struct {
	db *sql.DB
	tx *TxManager
}

var _ ports.SaleRepository = (*SaleRepository)(nil)

// NewSaleRepository construye el repositorio.
func NewSaleRepository(db *sql.DB) *SaleRepository {
	return &SaleRepository{db: db, tx: NewTxManager(db)}
}

// Create persiste la cabecera y sus líneas. Si no hay transacción activa en
// ctx abre una propia: una venta a medias nunca debe quedar guardada.
//
// Al insertar cada línea, trg_sale_items_ai_register:
//   - acumula sales.total_cents
//   - inserta un movimiento SALE en el kardex (que descuenta inventory)
func (r *SaleRepository) Create(ctx context.Context, s *domain.Sale) error {
	return r.tx.WithinTx(ctx, func(ctx context.Context) error {
		q := conn(ctx, r.db)
		res, err := q.ExecContext(ctx,
			`INSERT INTO sales (notes, created_at) VALUES (?, ?)`, s.Notes, formatTime(s.CreatedAt))
		if err != nil {
			return mapError(err)
		}
		if s.ID, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("sale last insert id: %w", err)
		}
		for i := range s.Items {
			it := &s.Items[i]
			it.SaleID = s.ID
			res, err := q.ExecContext(ctx, `
				INSERT INTO sale_items (sale_id, product_id, quantity, unit_price_cents)
				VALUES (?, ?, ?, ?)`, it.SaleID, it.ProductID, it.Quantity, it.UnitPrice.Cents())
			if err != nil {
				return mapError(err)
			}
			if it.ID, err = res.LastInsertId(); err != nil {
				return fmt.Errorf("sale item last insert id: %w", err)
			}
		}
		return nil
	})
}

// GetByID carga la venta con sus líneas.
func (r *SaleRepository) GetByID(ctx context.Context, id int64) (*domain.Sale, error) {
	q := conn(ctx, r.db)
	var (
		s       domain.Sale
		created string
	)
	if err := q.QueryRowContext(ctx,
		`SELECT id, notes, created_at FROM sales WHERE id = ?`, id).Scan(&s.ID, &s.Notes, &created); err != nil {
		return nil, mapError(err)
	}
	var err error
	if s.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	items, err := r.itemsFor(ctx, []int64{s.ID})
	if err != nil {
		return nil, err
	}
	s.Items = items[s.ID]
	return &s, nil
}

// List devuelve las ventas del rango, más recientes primero.
func (r *SaleRepository) List(ctx context.Context, rg domain.DateRange, limit int) ([]domain.Sale, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT id, notes, created_at FROM sales
		 WHERE created_at >= ? AND created_at < ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`, formatTime(rg.From), formatTime(rg.To), limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	sales := make([]domain.Sale, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var (
			s       domain.Sale
			created string
		)
		if err := rows.Scan(&s.ID, &s.Notes, &created); err != nil {
			return nil, mapError(err)
		}
		if s.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		sales = append(sales, s)
		ids = append(ids, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return sales, nil
	}
	// Una sola consulta para todas las líneas: evita el problema N+1.
	items, err := r.itemsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range sales {
		sales[i].Items = items[sales[i].ID]
	}
	return sales, nil
}

func (r *SaleRepository) itemsFor(ctx context.Context, saleIDs []int64) (map[int64][]domain.SaleItem, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(saleIDs)), ",")
	args := make([]any, len(saleIDs))
	for i, id := range saleIDs {
		args[i] = id
	}
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT id, sale_id, product_id, quantity, unit_price_cents
		  FROM sale_items WHERE sale_id IN (`+placeholders+`) ORDER BY id`, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make(map[int64][]domain.SaleItem, len(saleIDs))
	for rows.Next() {
		var (
			it    domain.SaleItem
			price int64
		)
		if err := rows.Scan(&it.ID, &it.SaleID, &it.ProductID, &it.Quantity, &price); err != nil {
			return nil, mapError(err)
		}
		it.UnitPrice = domain.Money(price)
		out[it.SaleID] = append(out[it.SaleID], it)
	}
	return out, rows.Err()
}
