// Package clock provee la implementación real del puerto ports.Clock.
package clock

import "time"

// System devuelve la hora real en UTC.
type System struct{}

// Now implementa ports.Clock.
func (System) Now() time.Time { return time.Now().UTC() }
