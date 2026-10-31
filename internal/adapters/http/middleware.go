package http

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// statusRecorder captura el código de respuesta para el log de acceso.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withLogging registra método, ruta, estado y duración de cada petición.
func withLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.InfoContext(r.Context(), "http request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

// withRecover evita que un panic tumbe el servidor completo.
func withRecover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.ErrorContext(r.Context(), "panic recovered", "panic", p, "stack", string(debug.Stack()))
				writeJSON(w, http.StatusInternalServerError,
					errorBody{Error: apiError{Code: "internal", Message: "unexpected error"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withSecurityHeaders añade cabeceras defensivas básicas.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
}
