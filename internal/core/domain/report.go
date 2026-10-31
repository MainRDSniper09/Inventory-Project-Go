package domain

import "time"

// DateRange es un intervalo semiabierto [From, To).
type DateRange struct {
	From time.Time
	To   time.Time
}

// NewDateRange valida que From < To.
func NewDateRange(from, to time.Time) (DateRange, error) {
	if !from.Before(to) {
		return DateRange{}, NewValidationError("range", "'from' must be before 'to'")
	}
	return DateRange{From: from.UTC(), To: to.UTC()}, nil
}

// DashboardSummary agrupa los KPIs principales del tablero.
type DashboardSummary struct {
	Range             DateRange
	ActiveProducts    int
	UnitsInStock      int
	InventoryValue    Money
	SalesCount        int
	UnitsSold         int
	Revenue           Money
	LowStockThreshold int
	LowStockCount     int
}

// ProductSales es el acumulado de ventas de un producto en un periodo.
type ProductSales struct {
	ProductID int64
	SKU       string
	Name      string
	UnitsSold int
	Revenue   Money
}

// DailySales es el acumulado de ventas de un día (UTC).
type DailySales struct {
	Day        string // YYYY-MM-DD
	SalesCount int
	UnitsSold  int
	Revenue    Money
}

// SaleReportLine es una fila plana para exportar (CSV / JSON).
type SaleReportLine struct {
	SaleID    int64
	SoldAt    time.Time
	SKU       string
	Product   string
	Quantity  int
	UnitPrice Money
	Subtotal  Money
}

// Dashboard es la respuesta completa del caso de uso de tablero.
type Dashboard struct {
	Summary     DashboardSummary
	TopProducts []ProductSales
	Daily       []DailySales
	LowStock    []ProductWithStock
}
