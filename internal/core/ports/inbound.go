package ports

import (
	"context"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
)

// ---------- Comandos (DTOs de entrada de los casos de uso) ----------

// CreateProductCmd son los datos para crear un producto.
type CreateProductCmd struct {
	SKU          string
	Name         string
	Description  string
	CategoryID   *int64
	PriceCents   int64
	InitialStock int
}

// UpdateProductCmd son los datos editables de un producto.
type UpdateProductCmd struct {
	Name        string
	Description string
	CategoryID  *int64
	PriceCents  int64
}

// RecordMovementCmd registra una entrada/salida/ajuste manual.
type RecordMovementCmd struct {
	ProductID int64
	Type      domain.MovementType
	Quantity  int
	Reason    string
}

// SaleLineCmd es una línea solicitada en una venta.
type SaleLineCmd struct {
	ProductID int64
	Quantity  int
}

// RegisterSaleCmd registra una venta completa.
type RegisterSaleCmd struct {
	Notes string
	Items []SaleLineCmd
}

// DashboardQuery parametriza el tablero.
type DashboardQuery struct {
	Range             domain.DateRange
	LowStockThreshold int
	TopLimit          int
}

// ---------- Casos de uso (puertos de entrada) ----------

// CategoryService gestiona categorías.
type CategoryService interface {
	Create(ctx context.Context, name, description string) (*domain.Category, error)
	List(ctx context.Context) ([]domain.Category, error)
}

// ProductService es el CRUD del catálogo.
type ProductService interface {
	Create(ctx context.Context, cmd CreateProductCmd) (*domain.ProductWithStock, error)
	Get(ctx context.Context, id int64) (*domain.ProductWithStock, error)
	List(ctx context.Context, f ProductFilter) ([]domain.ProductWithStock, error)
	Update(ctx context.Context, id int64, cmd UpdateProductCmd) (*domain.ProductWithStock, error)
	Archive(ctx context.Context, id int64) error
}

// InventoryService gestiona existencias.
type InventoryService interface {
	RecordMovement(ctx context.Context, cmd RecordMovementCmd) (*domain.StockMovement, error)
	Movements(ctx context.Context, productID int64, limit int) ([]domain.StockMovement, error)
}

// SaleService registra y consulta ventas.
type SaleService interface {
	Register(ctx context.Context, cmd RegisterSaleCmd) (*domain.Sale, error)
	Get(ctx context.Context, id int64) (*domain.Sale, error)
	List(ctx context.Context, r domain.DateRange, limit int) ([]domain.Sale, error)
}

// ReportService alimenta el dashboard y los reportes exportables.
type ReportService interface {
	Dashboard(ctx context.Context, q DashboardQuery) (*domain.Dashboard, error)
	SalesReport(ctx context.Context, r domain.DateRange) ([]domain.SaleReportLine, error)
}
