// Package ports define los CONTRATOS del hexágono.
//
//   - Puertos de salida (driven / secondary): lo que el núcleo NECESITA del
//     mundo exterior (persistencia, reloj, transacciones). Los implementan los
//     adaptadores secundarios (ej. internal/adapters/sqlite).
//   - Puertos de entrada (driving / primary): lo que el núcleo OFRECE (casos
//     de uso). Los consumen los adaptadores primarios (ej. internal/adapters/http).
//
// El núcleo sólo conoce estas interfaces: así se cumple el Principio de
// Inversión de Dependencias (la "D" de SOLID).
package ports

import (
	"context"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
)

// ProductFilter son los criterios de búsqueda de productos.
type ProductFilter struct {
	Query           string // busca en SKU y nombre
	CategoryID      *int64
	IncludeArchived bool
	Limit           int
	Offset          int
}

// ProductRepository persiste el catálogo de productos.
// Interfaces pequeñas y específicas = Principio de Segregación de Interfaces (ISP).
type ProductRepository interface {
	Create(ctx context.Context, p *domain.Product) error
	Update(ctx context.Context, p *domain.Product) error
	GetByID(ctx context.Context, id int64) (*domain.Product, error)
	GetWithStock(ctx context.Context, id int64) (*domain.ProductWithStock, error)
	List(ctx context.Context, f ProductFilter) ([]domain.ProductWithStock, error)
	Archive(ctx context.Context, id int64, at time.Time) error
}

// CategoryRepository persiste categorías.
type CategoryRepository interface {
	Create(ctx context.Context, c *domain.Category) error
	GetByID(ctx context.Context, id int64) (*domain.Category, error)
	List(ctx context.Context) ([]domain.Category, error)
}

// StockRepository lee existencias y registra movimientos del libro de inventario.
type StockRepository interface {
	Get(ctx context.Context, productID int64) (domain.Stock, error)
	RecordMovement(ctx context.Context, m *domain.StockMovement) error
	ListMovements(ctx context.Context, productID int64, limit int) ([]domain.StockMovement, error)
}

// SaleRepository persiste ventas (agregado Sale + SaleItems).
type SaleRepository interface {
	Create(ctx context.Context, s *domain.Sale) error
	GetByID(ctx context.Context, id int64) (*domain.Sale, error)
	List(ctx context.Context, r domain.DateRange, limit int) ([]domain.Sale, error)
}

// ReportRepository es el lado de LECTURA (CQRS ligero): consultas agregadas
// optimizadas que no necesitan reconstruir entidades completas.
type ReportRepository interface {
	Summary(ctx context.Context, r domain.DateRange, lowStockThreshold int) (domain.DashboardSummary, error)
	TopProducts(ctx context.Context, r domain.DateRange, limit int) ([]domain.ProductSales, error)
	DailySales(ctx context.Context, r domain.DateRange) ([]domain.DailySales, error)
	LowStock(ctx context.Context, threshold, limit int) ([]domain.ProductWithStock, error)
	SalesLines(ctx context.Context, r domain.DateRange) ([]domain.SaleReportLine, error)
}

// TxManager ejecuta fn dentro de una transacción (Unit of Work).
// Los repositorios detectan la transacción a través del context.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Clock abstrae el tiempo para que las pruebas sean deterministas.
type Clock interface {
	Now() time.Time
}
