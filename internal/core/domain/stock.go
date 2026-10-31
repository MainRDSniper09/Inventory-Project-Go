package domain

import (
	"strings"
	"time"
)

// MovementType clasifica un movimiento de inventario.
type MovementType string

const (
	MovementIn     MovementType = "IN"     // entrada de mercancía (compra, devolución)
	MovementOut    MovementType = "OUT"    // salida manual (merma, daño)
	MovementAdjust MovementType = "ADJUST" // ajuste por conteo físico (+/-)
	MovementSale   MovementType = "SALE"   // generado automáticamente por una venta
)

const maxReasonLen = 200

// Stock es la existencia actual de un producto. En la BD se mantiene como una
// proyección del libro de movimientos (ver migraciones / triggers).
type Stock struct {
	ProductID int64
	Quantity  int
	UpdatedAt time.Time
}

// CanFulfill indica si hay suficiente existencia para sacar qty unidades.
func (s Stock) CanFulfill(qty int) bool { return qty > 0 && s.Quantity >= qty }

// StockMovement es un asiento inmutable del libro de inventario (ledger).
// Delta es con signo: positivo suma existencias, negativo resta.
type StockMovement struct {
	ID         int64
	ProductID  int64
	Type       MovementType
	Delta      int
	Reason     string
	SaleItemID *int64
	CreatedAt  time.Time
}

// NewManualMovement crea un movimiento hecho por un usuario.
//
//   - IN:     quantity > 0, Delta = +quantity
//   - OUT:    quantity > 0, Delta = -quantity
//   - ADJUST: quantity != 0, Delta = quantity (puede ser negativo)
//
// Los movimientos SALE no se crean a mano: los genera la venta.
func NewManualMovement(productID int64, t MovementType, quantity int, reason string, now time.Time) (*StockMovement, error) {
	if productID <= 0 {
		return nil, NewValidationError("product_id", "must be a positive id")
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > maxReasonLen {
		return nil, NewValidationError("reason", "must be at most 200 characters")
	}

	var delta int
	switch t {
	case MovementIn:
		if quantity <= 0 {
			return nil, NewValidationError("quantity", "must be greater than 0")
		}
		delta = quantity
	case MovementOut:
		if quantity <= 0 {
			return nil, NewValidationError("quantity", "must be greater than 0")
		}
		delta = -quantity
	case MovementAdjust:
		if quantity == 0 {
			return nil, NewValidationError("quantity", "must be different from 0")
		}
		if reason == "" {
			return nil, NewValidationError("reason", "is required for adjustments")
		}
		delta = quantity
	case MovementSale:
		return nil, NewValidationError("type", "SALE movements are created by sales only")
	default:
		return nil, NewValidationError("type", "must be one of IN, OUT, ADJUST")
	}

	return &StockMovement{
		ProductID: productID,
		Type:      t,
		Delta:     delta,
		Reason:    reason,
		CreatedAt: now.UTC(),
	}, nil
}
