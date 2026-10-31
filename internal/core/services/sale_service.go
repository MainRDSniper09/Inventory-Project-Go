package services

import (
	"context"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// SaleService implementa ports.SaleService.
type SaleService struct {
	products ports.ProductRepository
	stock    ports.StockRepository
	sales    ports.SaleRepository
	tx       ports.TxManager
	clock    ports.Clock
}

var _ ports.SaleService = (*SaleService)(nil)

// NewSaleService construye el servicio.
func NewSaleService(p ports.ProductRepository, st ports.StockRepository, sa ports.SaleRepository,
	tx ports.TxManager, clock ports.Clock) *SaleService {
	return &SaleService{products: p, stock: st, sales: sa, tx: tx, clock: clock}
}

// Register crea una venta de forma ATÓMICA:
//  1. Toma el precio vigente de cada producto (el cliente no decide el precio).
//  2. Verifica existencias suficientes.
//  3. Persiste la venta; la BD descuenta el stock vía triggers.
//
// Si cualquier paso falla, la transacción se revierte completa.
func (s *SaleService) Register(ctx context.Context, cmd ports.RegisterSaleCmd) (*domain.Sale, error) {
	sale, err := domain.NewSale(cmd.Notes, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if len(cmd.Items) == 0 {
		return nil, domain.NewValidationError("items", "a sale needs at least one item")
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		for _, line := range cmd.Items {
			p, err := s.products.GetByID(ctx, line.ProductID)
			if err != nil {
				return fmt.Errorf("product %d: %w", line.ProductID, err)
			}
			if !p.Active {
				return domain.NewValidationError("product_id", fmt.Sprintf("product %d is archived", p.ID))
			}
			if err := sale.AddItem(p.ID, line.Quantity, p.Price); err != nil {
				return err
			}
		}
		if err := sale.Validate(); err != nil {
			return err
		}
		// Verificamos stock DESPUÉS de consolidar líneas repetidas.
		for _, it := range sale.Items {
			st, err := s.stock.Get(ctx, it.ProductID)
			if err != nil {
				return err
			}
			if !st.CanFulfill(it.Quantity) {
				return &domain.StockError{ProductID: it.ProductID, Requested: it.Quantity, Available: st.Quantity}
			}
		}
		return s.sales.Create(ctx, sale)
	})
	if err != nil {
		return nil, fmt.Errorf("register sale: %w", err)
	}
	return sale, nil
}

// Get devuelve una venta con sus líneas.
func (s *SaleService) Get(ctx context.Context, id int64) (*domain.Sale, error) {
	return s.sales.GetByID(ctx, id)
}

// List devuelve ventas de un periodo.
func (s *SaleService) List(ctx context.Context, r domain.DateRange, limit int) ([]domain.Sale, error) {
	return s.sales.List(ctx, r, clampLimit(limit))
}
