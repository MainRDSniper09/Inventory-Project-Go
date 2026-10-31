package domain

import "fmt"

// Money representa un valor monetario en CENTAVOS.
//
// Nunca usamos float64 para dinero: 0.1 + 0.2 != 0.3 en punto flotante.
// Guardar enteros evita errores de redondeo acumulados en reportes.
type Money int64

// NewMoney valida que el monto no sea negativo.
func NewMoney(cents int64) (Money, error) {
	if cents < 0 {
		return 0, NewValidationError("price", "must be greater than or equal to 0")
	}
	return Money(cents), nil
}

// Cents devuelve el valor crudo en centavos.
func (m Money) Cents() int64 { return int64(m) }

// Multiply calcula m * qty (por ejemplo, precio unitario * cantidad).
func (m Money) Multiply(qty int) Money { return m * Money(qty) }

// String formatea el monto como "1234.56".
func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}
