// Package http es un ADAPTADOR PRIMARIO: traduce peticiones HTTP a llamadas
// a los casos de uso (puertos de entrada) y sus resultados a JSON/CSV/HTML.
//
// Sólo usa la librería estándar (net/http con el router de Go 1.22+): menos
// dependencias = menos superficie de ataque y más estabilidad a largo plazo.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// writeError traduce errores del DOMINIO a códigos HTTP. Es el único lugar
// donde vive esta correspondencia (SRP).
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var (
		status = http.StatusInternalServerError
		body   = apiError{Code: "internal", Message: "unexpected error"}
		ve     *domain.ValidationError
	)
	switch {
	case errors.As(err, &ve):
		status, body = http.StatusBadRequest, apiError{Code: "invalid_input", Message: ve.Message, Field: ve.Field}
	case errors.Is(err, domain.ErrInvalidInput):
		status, body = http.StatusBadRequest, apiError{Code: "invalid_input", Message: err.Error()}
	case errors.Is(err, domain.ErrNotFound):
		status, body = http.StatusNotFound, apiError{Code: "not_found", Message: "resource not found"}
	case errors.Is(err, domain.ErrConflict):
		status, body = http.StatusConflict, apiError{Code: "conflict", Message: "resource already exists"}
	case errors.Is(err, domain.ErrInsufficientStock):
		status, body = http.StatusConflict, apiError{Code: "insufficient_stock", Message: err.Error()}
	default:
		// No filtramos detalles internos al cliente; sí al log.
		log.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	}
	writeJSON(w, status, errorBody{Error: body})
}

// decodeJSON lee el cuerpo con límite de tamaño y rechaza campos desconocidos
// (así detectamos errores de tipeo del cliente en vez de ignorarlos).
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.NewValidationError("body", fmt.Sprintf("invalid JSON: %v", err))
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewValidationError("body", "must contain a single JSON object")
	}
	return nil
}
