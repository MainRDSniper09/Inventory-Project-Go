package sqlite

import (
	"context"
	"database/sql"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// ReportRepository implementa ports.ReportRepository usando las vistas
// v_product_stock y v_sales_lines.
type ReportRepository struct{ db *sql.DB }

var _ ports.ReportRepository = (*ReportRepository)(nil)

// NewReportRepository construye el repositorio.
func NewReportRepository(db *sql.DB) *ReportRepository { return &ReportRepository{db: db} }

// Summary calcula los KPIs del dashboard.
func (r *ReportRepository) Summary(ctx context.Context, rg domain.DateRange, threshold int) (domain.DashboardSummary, error) {
	s := domain.DashboardSummary{Range: rg, LowStockThreshold: threshold}
	q := conn(ctx, r.db)

	var invValue int64
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(quantity), 0),
		       COALESCE(SUM(quantity * price_cents), 0),
		       COALESCE(SUM(CASE WHEN quantity <= ? THEN 1 ELSE 0 END), 0)
		  FROM v_product_stock WHERE active = 1`, threshold).
		Scan(&s.ActiveProducts, &s.UnitsInStock, &invValue, &s.LowStockCount); err != nil {
		return s, mapError(err)
	}
	s.InventoryValue = domain.Money(invValue)

	var revenue int64
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT sale_id),
		       COALESCE(SUM(quantity), 0),
		       COALESCE(SUM(subtotal_cents), 0)
		  FROM v_sales_lines
		 WHERE sold_at >= ? AND sold_at < ?`, formatTime(rg.From), formatTime(rg.To)).
		Scan(&s.SalesCount, &s.UnitsSold, &revenue); err != nil {
		return s, mapError(err)
	}
	s.Revenue = domain.Money(revenue)
	return s, nil
}

// TopProducts devuelve los productos más vendidos (por unidades).
func (r *ReportRepository) TopProducts(ctx context.Context, rg domain.DateRange, limit int) ([]domain.ProductSales, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT product_id, sku, product_name, SUM(quantity) AS units, SUM(subtotal_cents) AS revenue
		  FROM v_sales_lines
		 WHERE sold_at >= ? AND sold_at < ?
		 GROUP BY product_id, sku, product_name
		 ORDER BY units DESC, revenue DESC, product_id
		 LIMIT ?`, formatTime(rg.From), formatTime(rg.To), limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]domain.ProductSales, 0)
	for rows.Next() {
		var (
			ps  domain.ProductSales
			rev int64
		)
		if err := rows.Scan(&ps.ProductID, &ps.SKU, &ps.Name, &ps.UnitsSold, &rev); err != nil {
			return nil, mapError(err)
		}
		ps.Revenue = domain.Money(rev)
		out = append(out, ps)
	}
	return out, rows.Err()
}

// DailySales agrupa por día UTC (los primeros 10 caracteres de la fecha ISO).
func (r *ReportRepository) DailySales(ctx context.Context, rg domain.DateRange) ([]domain.DailySales, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT substr(sold_at, 1, 10) AS day,
		       COUNT(DISTINCT sale_id),
		       SUM(quantity),
		       SUM(subtotal_cents)
		  FROM v_sales_lines
		 WHERE sold_at >= ? AND sold_at < ?
		 GROUP BY day
		 ORDER BY day`, formatTime(rg.From), formatTime(rg.To))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]domain.DailySales, 0)
	for rows.Next() {
		var (
			d   domain.DailySales
			rev int64
		)
		if err := rows.Scan(&d.Day, &d.SalesCount, &d.UnitsSold, &rev); err != nil {
			return nil, mapError(err)
		}
		d.Revenue = domain.Money(rev)
		out = append(out, d)
	}
	return out, rows.Err()
}

// LowStock lista productos activos con existencia <= threshold.
func (r *ReportRepository) LowStock(ctx context.Context, threshold, limit int) ([]domain.ProductWithStock, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT `+productStockColumns+`
		  FROM v_product_stock
		 WHERE active = 1 AND quantity <= ?
		 ORDER BY quantity, name COLLATE NOCASE
		 LIMIT ?`, threshold, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return collectProductStock(rows)
}

// SalesLines devuelve el detalle plano para exportar.
func (r *ReportRepository) SalesLines(ctx context.Context, rg domain.DateRange) ([]domain.SaleReportLine, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx, `
		SELECT sale_id, sold_at, sku, product_name, quantity, unit_price_cents, subtotal_cents
		  FROM v_sales_lines
		 WHERE sold_at >= ? AND sold_at < ?
		 ORDER BY sold_at, sale_id, sku`, formatTime(rg.From), formatTime(rg.To))
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	out := make([]domain.SaleReportLine, 0)
	for rows.Next() {
		var (
			l              domain.SaleReportLine
			soldAt         string
			unit, subtotal int64
		)
		if err := rows.Scan(&l.SaleID, &soldAt, &l.SKU, &l.Product, &l.Quantity, &unit, &subtotal); err != nil {
			return nil, mapError(err)
		}
		if l.SoldAt, err = parseTime(soldAt); err != nil {
			return nil, err
		}
		l.UnitPrice = domain.Money(unit)
		l.Subtotal = domain.Money(subtotal)
		out = append(out, l)
	}
	return out, rows.Err()
}
