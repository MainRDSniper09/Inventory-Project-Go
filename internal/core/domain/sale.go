package domain

import (
	"strings"
	"time"
)

const maxSaleItems = 100

// SaleItem es una línea de venta. UnitPrice es una FOTO del precio al momento
// de vender: si el precio del producto cambia mañana, los reportes de hoy no
// deben alterarse.
type SaleItem struct {
	ID        int64
	SaleID    int64
	ProductID int64
	Quantity  int
	UnitPrice Money
}

// Subtotal = cantidad * precio unitario.
func (i SaleItem) Subtotal() Money { return i.UnitPrice.Multiply(i.Quantity) }

// Sale es el agregado raíz de una venta: controla sus líneas y su total.
type Sale struct {
	ID        int64
	Notes     string
	Items     []SaleItem
	CreatedAt time.Time
}

// NewSale crea una venta vacía.
func NewSale(notes string, now time.Time) (*Sale, error) {
	notes = strings.TrimSpace(notes)
	if len([]rune(notes)) > maxReasonLen {
		return nil, NewValidationError("notes", "must be at most 200 characters")
	}
	return &Sale{Notes: notes, CreatedAt: now.UTC()}, nil
}

// AddItem agrega una línea; si el producto ya existe en la venta se suman
// las cantidades (una sola línea por producto).
func (s *Sale) AddItem(productID int64, qty int, unitPrice Money) error {
	if productID <= 0 {
		return NewValidationError("product_id", "must be a positive id")
	}
	if qty <= 0 {
		return NewValidationError("quantity", "must be greater than 0")
	}
	if unitPrice < 0 {
		return NewValidationError("unit_price", "must be greater than or equal to 0")
	}
	for i := range s.Items {
		if s.Items[i].ProductID == productID {
			s.Items[i].Quantity += qty
			return nil
		}
	}
	if len(s.Items) >= maxSaleItems {
		return NewValidationError("items", "a sale can have at most 100 different products")
	}
	s.Items = append(s.Items, SaleItem{ProductID: productID, Quantity: qty, UnitPrice: unitPrice})
	return nil
}

// Total suma los subtotales de todas las líneas.
func (s *Sale) Total() Money {
	var t Money
	for _, it := range s.Items {
		t += it.Subtotal()
	}
	return t
}

// UnitsSold suma las cantidades vendidas.
func (s *Sale) UnitsSold() int {
	n := 0
	for _, it := range s.Items {
		n += it.Quantity
	}
	return n
}

// Validate verifica las invariantes del agregado antes de persistir.
func (s *Sale) Validate() error {
	if len(s.Items) == 0 {
		return NewValidationError("items", "a sale needs at least one item")
	}
	return nil
}
