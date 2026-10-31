package services

import (
	"context"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// CategoryService implementa ports.CategoryService.
type CategoryService struct {
	repo  ports.CategoryRepository
	clock ports.Clock
}

var _ ports.CategoryService = (*CategoryService)(nil)

// NewCategoryService construye el servicio.
func NewCategoryService(repo ports.CategoryRepository, clock ports.Clock) *CategoryService {
	return &CategoryService{repo: repo, clock: clock}
}

// Create valida y persiste una categoría.
func (s *CategoryService) Create(ctx context.Context, name, description string) (*domain.Category, error) {
	c, err := domain.NewCategory(name, description, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	return c, nil
}

// List devuelve todas las categorías.
func (s *CategoryService) List(ctx context.Context) ([]domain.Category, error) {
	return s.repo.List(ctx)
}
