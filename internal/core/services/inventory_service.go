package services

import (
	"context"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// InventoryService implementa ports.InventoryService.
type InventoryService struct {
	products ports.ProductRepository
	stock    ports.StockRepository
	tx       ports.TxManager
	clock    ports.Clock
}

var _ ports.InventoryService = (*InventoryService)(nil)

// NewInventoryService construye el servicio.
func NewInventoryService(p ports.ProductRepository, s ports.StockRepository, tx ports.TxManager, clock ports.Clock) *InventoryService {
	return &InventoryService{products: p, stock: s, tx: tx, clock: clock}
}

// RecordMovement registra una entrada, salida o ajuste manual.
//
// Defensa en profundidad: validamos el stock aquí (mensaje de error claro) y
// la BD lo vuelve a garantizar con un CHECK (quantity >= 0) por si dos
// peticiones concurrentes compiten por las mismas unidades.
func (s *InventoryService) RecordMovement(ctx context.Context, cmd ports.RecordMovementCmd) (*domain.StockMovement, error) {
	m, err := domain.NewManualMovement(cmd.ProductID, cmd.Type, cmd.Quantity, cmd.Reason, s.clock.Now())
	if err != nil {
		return nil, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.products.GetByID(ctx, cmd.ProductID)
		if err != nil {
			return err
		}
		if !p.Active {
			return domain.NewValidationError("product_id", "product is archived")
		}
		if m.Delta < 0 {
			st, err := s.stock.Get(ctx, cmd.ProductID)
			if err != nil {
				return err
			}
			if !st.CanFulfill(-m.Delta) {
				return &domain.StockError{ProductID: cmd.ProductID, Requested: -m.Delta, Available: st.Quantity}
			}
		}
		return s.stock.RecordMovement(ctx, m)
	})
	if err != nil {
		return nil, fmt.Errorf("record movement: %w", err)
	}
	return m, nil
}

// Movements devuelve el historial (kardex) de un producto.
func (s *InventoryService) Movements(ctx context.Context, productID int64, limit int) ([]domain.StockMovement, error) {
	if _, err := s.products.GetByID(ctx, productID); err != nil {
		return nil, err
	}
	return s.stock.ListMovements(ctx, productID, clampLimit(limit))
}
