// Package services implementa los casos de uso (puertos de entrada).
// Orquesta entidades del dominio y puertos de salida; no sabe nada de HTTP
// ni de SQL.
package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

const (
	defaultListLimit = 50
	maxListLimit     = 500
)

// ProductService implementa ports.ProductService.
type ProductService struct {
	products   ports.ProductRepository
	categories ports.CategoryRepository
	stock      ports.StockRepository
	tx         ports.TxManager
	clock      ports.Clock
}

// Verificación en compilación de que cumplimos el contrato.
var _ ports.ProductService = (*ProductService)(nil)

// NewProductService recibe sus dependencias por constructor (Inyección de
// Dependencias): el servicio no decide QUÉ implementación usar.
func NewProductService(p ports.ProductRepository, c ports.CategoryRepository, s ports.StockRepository,
	tx ports.TxManager, clock ports.Clock) *ProductService {
	return &ProductService{products: p, categories: c, stock: s, tx: tx, clock: clock}
}

// Create registra el producto y, opcionalmente, su existencia inicial.
// Ambas operaciones ocurren en UNA transacción: o se guardan las dos o ninguna.
func (s *ProductService) Create(ctx context.Context, cmd ports.CreateProductCmd) (*domain.ProductWithStock, error) {
	if cmd.InitialStock < 0 {
		return nil, domain.NewValidationError("initial_stock", "must be greater than or equal to 0")
	}
	price, err := domain.NewMoney(cmd.PriceCents)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	p, err := domain.NewProduct(cmd.SKU, domain.ProductDetails{
		Name: cmd.Name, Description: cmd.Description, CategoryID: cmd.CategoryID, Price: price,
	}, now)
	if err != nil {
		return nil, err
	}

	var out *domain.ProductWithStock
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.ensureCategory(ctx, p.CategoryID); err != nil {
			return err
		}
		if err := s.products.Create(ctx, p); err != nil {
			return err
		}
		if cmd.InitialStock > 0 {
			m, err := domain.NewManualMovement(p.ID, domain.MovementIn, cmd.InitialStock, "stock inicial", now)
			if err != nil {
				return err
			}
			if err := s.stock.RecordMovement(ctx, m); err != nil {
				return err
			}
		}
		out, err = s.products.GetWithStock(ctx, p.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return out, nil
}

// Get devuelve un producto con su existencia.
func (s *ProductService) Get(ctx context.Context, id int64) (*domain.ProductWithStock, error) {
	return s.products.GetWithStock(ctx, id)
}

// List aplica límites seguros de paginación y delega al repositorio.
func (s *ProductService) List(ctx context.Context, f ports.ProductFilter) ([]domain.ProductWithStock, error) {
	f.Limit = clampLimit(f.Limit)
	if f.Offset < 0 {
		f.Offset = 0
	}
	return s.products.List(ctx, f)
}

// Update modifica datos editables (no el stock: eso es InventoryService).
func (s *ProductService) Update(ctx context.Context, id int64, cmd ports.UpdateProductCmd) (*domain.ProductWithStock, error) {
	price, err := domain.NewMoney(cmd.PriceCents)
	if err != nil {
		return nil, err
	}
	var out *domain.ProductWithStock
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.products.GetByID(ctx, id)
		if err != nil {
			return err
		}
		details := domain.ProductDetails{
			Name: cmd.Name, Description: cmd.Description, CategoryID: cmd.CategoryID, Price: price,
		}
		if err := p.Update(details, s.clock.Now()); err != nil {
			return err
		}
		if err := s.ensureCategory(ctx, p.CategoryID); err != nil {
			return err
		}
		if err := s.products.Update(ctx, p); err != nil {
			return err
		}
		out, err = s.products.GetWithStock(ctx, id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("update product %d: %w", id, err)
	}
	return out, nil
}

// Archive hace un borrado LÓGICO: el producto deja de listarse y venderse,
// pero su historial de ventas y movimientos se conserva para los reportes.
func (s *ProductService) Archive(ctx context.Context, id int64) error {
	if err := s.products.Archive(ctx, id, s.clock.Now()); err != nil {
		return fmt.Errorf("archive product %d: %w", id, err)
	}
	return nil
}

func (s *ProductService) ensureCategory(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	if _, err := s.categories.GetByID(ctx, *id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewValidationError("category_id", "category does not exist")
		}
		return err
	}
	return nil
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultListLimit
	case limit > maxListLimit:
		return maxListLimit
	default:
		return limit
	}
}
