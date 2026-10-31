package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// CategoryRepository implementa ports.CategoryRepository.
type CategoryRepository struct{ db *sql.DB }

var _ ports.CategoryRepository = (*CategoryRepository)(nil)

// NewCategoryRepository construye el repositorio.
func NewCategoryRepository(db *sql.DB) *CategoryRepository { return &CategoryRepository{db: db} }

// Create inserta la categoría (nombre único, sin distinguir mayúsculas).
func (r *CategoryRepository) Create(ctx context.Context, c *domain.Category) error {
	res, err := conn(ctx, r.db).ExecContext(ctx,
		`INSERT INTO categories (name, description, created_at) VALUES (?, ?, ?)`,
		c.Name, c.Description, formatTime(c.CreatedAt))
	if err != nil {
		return mapError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("category last insert id: %w", err)
	}
	c.ID = id
	return nil
}

// GetByID busca una categoría.
func (r *CategoryRepository) GetByID(ctx context.Context, id int64) (*domain.Category, error) {
	row := conn(ctx, r.db).QueryRowContext(ctx,
		`SELECT id, name, description, created_at FROM categories WHERE id = ?`, id)
	c, err := scanCategory(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// List devuelve todas las categorías ordenadas por nombre.
func (r *CategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := conn(ctx, r.db).QueryContext(ctx,
		`SELECT id, name, description, created_at FROM categories ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	out := make([]domain.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanCategory(s rowScanner) (domain.Category, error) {
	var (
		c       domain.Category
		created string
	)
	if err := s.Scan(&c.ID, &c.Name, &c.Description, &created); err != nil {
		return c, mapError(err)
	}
	t, err := parseTime(created)
	if err != nil {
		return c, err
	}
	c.CreatedAt = t
	return c, nil
}
