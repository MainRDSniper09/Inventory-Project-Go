package domain

import (
	"strings"
	"time"
)

const maxCategoryNameLen = 60

// Category agrupa productos (ej. "Bebidas", "Electrónica").
type Category struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

// NewCategory construye una categoría válida o devuelve un error de validación.
func NewCategory(name, description string, now time.Time) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, NewValidationError("name", "is required")
	}
	if len([]rune(name)) > maxCategoryNameLen {
		return nil, NewValidationError("name", "must be at most 60 characters")
	}
	return &Category{
		Name:        name,
		Description: strings.TrimSpace(description),
		CreatedAt:   now.UTC(),
	}, nil
}
