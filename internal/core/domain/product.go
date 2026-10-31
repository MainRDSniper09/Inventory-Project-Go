package domain

import (
	"regexp"
	"strings"
	"time"
)

const (
	maxSKULen         = 32
	maxProductNameLen = 120
	maxDescriptionLen = 500
)

var skuPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]*$`)

// Product es la entidad principal del catálogo.
//
// Nota de diseño: la CANTIDAD en inventario NO vive aquí. El stock es un
// concepto distinto (cambia con cada venta/entrada) y se modela en Stock y
// StockMovement. Esto respeta el Principio de Responsabilidad Única (SRP).
type Product struct {
	ID          int64
	SKU         string
	Name        string
	Description string
	CategoryID  *int64
	Price       Money
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProductDetails son los campos editables de un producto.
type ProductDetails struct {
	Name        string
	Description string
	CategoryID  *int64
	Price       Money
}

// NewProduct crea un producto activo y válido.
func NewProduct(sku string, d ProductDetails, now time.Time) (*Product, error) {
	sku = strings.ToUpper(strings.TrimSpace(sku))
	if err := validateSKU(sku); err != nil {
		return nil, err
	}
	p := &Product{
		SKU:       sku,
		Active:    true,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
	if err := p.apply(d); err != nil {
		return nil, err
	}
	return p, nil
}

// Update modifica los datos editables. El SKU es inmutable: es la identidad
// de negocio del producto y cambiarlo rompería la trazabilidad de reportes.
func (p *Product) Update(d ProductDetails, now time.Time) error {
	if !p.Active {
		return NewValidationError("product", "archived products cannot be modified")
	}
	if err := p.apply(d); err != nil {
		return err
	}
	p.UpdatedAt = now.UTC()
	return nil
}

func (p *Product) apply(d ProductDetails) error {
	name := strings.TrimSpace(d.Name)
	if name == "" {
		return NewValidationError("name", "is required")
	}
	if len([]rune(name)) > maxProductNameLen {
		return NewValidationError("name", "must be at most 120 characters")
	}
	desc := strings.TrimSpace(d.Description)
	if len([]rune(desc)) > maxDescriptionLen {
		return NewValidationError("description", "must be at most 500 characters")
	}
	if d.Price < 0 {
		return NewValidationError("price", "must be greater than or equal to 0")
	}
	if d.CategoryID != nil && *d.CategoryID <= 0 {
		return NewValidationError("category_id", "must be a positive id")
	}
	p.Name = name
	p.Description = desc
	p.CategoryID = d.CategoryID
	p.Price = d.Price
	return nil
}

func validateSKU(sku string) error {
	if sku == "" {
		return NewValidationError("sku", "is required")
	}
	if len(sku) > maxSKULen {
		return NewValidationError("sku", "must be at most 32 characters")
	}
	if !skuPattern.MatchString(sku) {
		return NewValidationError("sku", "only letters, digits, '-' and '_' are allowed")
	}
	return nil
}

// ProductWithStock es un modelo de lectura (read model) que combina el
// producto con su categoría y existencia actual. Se usa para listados.
type ProductWithStock struct {
	Product
	CategoryName string
	Quantity     int
}
