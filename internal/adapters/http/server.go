package http

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

//go:embed web/static
var staticFS embed.FS

// Services agrupa los puertos de entrada que el adaptador HTTP consume.
// Depende de INTERFACES, no de implementaciones concretas (DIP).
type Services struct {
	Categories ports.CategoryService
	Products   ports.ProductService
	Inventory  ports.InventoryService
	Sales      ports.SaleService
	Reports    ports.ReportService
}

// Handler expone la API REST y la interfaz web.
type Handler struct {
	svc   Services
	log   *slog.Logger
	clock ports.Clock
}

// NewHandler construye el http.Handler raíz con su cadena de middlewares.
func NewHandler(svc Services, clock ports.Clock, log *slog.Logger) http.Handler {
	h := &Handler{svc: svc, log: log, clock: clock}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.health)

	mux.HandleFunc("GET /api/v1/categories", h.listCategories)
	mux.HandleFunc("POST /api/v1/categories", h.createCategory)

	mux.HandleFunc("GET /api/v1/products", h.listProducts)
	mux.HandleFunc("POST /api/v1/products", h.createProduct)
	mux.HandleFunc("GET /api/v1/products/{id}", h.getProduct)
	mux.HandleFunc("PUT /api/v1/products/{id}", h.updateProduct)
	mux.HandleFunc("DELETE /api/v1/products/{id}", h.archiveProduct)
	mux.HandleFunc("GET /api/v1/products/{id}/movements", h.listMovements)
	mux.HandleFunc("POST /api/v1/products/{id}/movements", h.recordMovement)

	mux.HandleFunc("GET /api/v1/sales", h.listSales)
	mux.HandleFunc("POST /api/v1/sales", h.registerSale)
	mux.HandleFunc("GET /api/v1/sales/{id}", h.getSale)

	mux.HandleFunc("GET /api/v1/dashboard", h.dashboard)
	mux.HandleFunc("GET /api/v1/reports/sales", h.salesReport)

	static, _ := fs.Sub(staticFS, "web/static")
	mux.Handle("GET /", http.FileServerFS(static))

	return withRecover(log, withLogging(log, withSecurityHeaders(mux)))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// pathID extrae y valida el parámetro {id} de la ruta.
func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.NewValidationError("id", "must be a positive integer")
	}
	return id, nil
}

func queryInt(r *http.Request, key string, def int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, domain.NewValidationError(key, "must be an integer")
	}
	return n, nil
}
