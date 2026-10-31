// Package domain contiene el núcleo del negocio: entidades, value objects y
// reglas invariantes. NO depende de ningún otro paquete del proyecto ni de
// librerías de infraestructura (BD, HTTP, etc.). Es el centro del hexágono.
package domain

import (
	"errors"
	"fmt"
)

// Errores centinela del dominio. Los adaptadores los traducen a su propio
// lenguaje (HTTP status, códigos de salida, etc.) usando errors.Is.
var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource already exists")
	ErrInvalidInput      = errors.New("invalid input")
	ErrInsufficientStock = errors.New("insufficient stock")
)

// ValidationError describe un campo inválido. Envuelve ErrInvalidInput para
// que errors.Is(err, ErrInvalidInput) funcione en cualquier capa.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

// NewValidationError es un atajo para construir errores de validación.
func NewValidationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// StockError da contexto cuando una operación dejaría el inventario negativo.
type StockError struct {
	ProductID int64
	Requested int
	Available int
}

func (e *StockError) Error() string {
	return fmt.Sprintf("insufficient stock for product %d: requested %d, available %d",
		e.ProductID, e.Requested, e.Available)
}

func (e *StockError) Unwrap() error { return ErrInsufficientStock }
