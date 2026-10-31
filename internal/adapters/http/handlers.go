package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// Cada handler sigue el mismo patrón de 3 pasos:
//   1. Parsear/validar la entrada HTTP -> comando del caso de uso.
//   2. Invocar el puerto de entrada.
//   3. Mapear el resultado (o error) a la respuesta HTTP.

// ------------------------------------------------------------ categorías

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.Categories.List(r.Context())
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	out := make([]categoryResponse, len(cats))
	for i, c := range cats {
		out[i] = toCategory(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	c, err := h.svc.Categories.Create(r.Context(), req.Name, req.Description)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCategory(*c))
}

// ------------------------------------------------------------ productos

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := queryInt(r, "limit", 0)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	offset, err := queryInt(r, "offset", 0)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	f := ports.ProductFilter{
		Query:           q.Get("q"),
		IncludeArchived: q.Get("include_archived") == "true",
		Limit:           limit,
		Offset:          offset,
	}
	if v := q.Get("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, r, h.log, domain.NewValidationError("category_id", "must be an integer"))
			return
		}
		f.CategoryID = &id
	}
	ps, err := h.svc.Products.List(r.Context(), f)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toProducts(ps))
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	p, err := h.svc.Products.Create(r.Context(), ports.CreateProductCmd{
		SKU: req.SKU, Name: req.Name, Description: req.Description,
		CategoryID: req.CategoryID, PriceCents: req.PriceCents, InitialStock: req.InitialStock,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/products/%d", p.ID))
	writeJSON(w, http.StatusCreated, toProduct(*p))
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	p, err := h.svc.Products.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toProduct(*p))
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req updateProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	p, err := h.svc.Products.Update(r.Context(), id, ports.UpdateProductCmd{
		Name: req.Name, Description: req.Description, CategoryID: req.CategoryID, PriceCents: req.PriceCents,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toProduct(*p))
}

func (h *Handler) archiveProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.svc.Products.Archive(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ------------------------------------------------------------ inventario

func (h *Handler) recordMovement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req movementRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	m, err := h.svc.Inventory.RecordMovement(r.Context(), ports.RecordMovementCmd{
		ProductID: id, Type: domain.MovementType(strings.ToUpper(req.Type)), Quantity: req.Quantity, Reason: req.Reason,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toMovement(*m))
}

func (h *Handler) listMovements(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	limit, err := queryInt(r, "limit", 0)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	ms, err := h.svc.Inventory.Movements(r.Context(), id, limit)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	out := make([]movementResponse, len(ms))
	for i, m := range ms {
		out[i] = toMovement(m)
	}
	writeJSON(w, http.StatusOK, out)
}

// ------------------------------------------------------------ ventas

func (h *Handler) registerSale(w http.ResponseWriter, r *http.Request) {
	var req saleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	cmd := ports.RegisterSaleCmd{Notes: req.Notes, Items: make([]ports.SaleLineCmd, len(req.Items))}
	for i, it := range req.Items {
		cmd.Items[i] = ports.SaleLineCmd{ProductID: it.ProductID, Quantity: it.Quantity}
	}
	s, err := h.svc.Sales.Register(r.Context(), cmd)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/sales/%d", s.ID))
	writeJSON(w, http.StatusCreated, toSale(*s))
}

func (h *Handler) getSale(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	s, err := h.svc.Sales.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toSale(*s))
}

func (h *Handler) listSales(w http.ResponseWriter, r *http.Request) {
	rg, err := h.dateRange(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	limit, err := queryInt(r, "limit", 0)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	sales, err := h.svc.Sales.List(r.Context(), rg, limit)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	out := make([]saleResponse, len(sales))
	for i, s := range sales {
		out[i] = toSale(s)
	}
	writeJSON(w, http.StatusOK, out)
}

// ------------------------------------------------------------ dashboard / reportes

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	rg, err := h.dateRange(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	low, err := queryInt(r, "low_stock", 0)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	top, err := queryInt(r, "top", 0)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	d, err := h.svc.Reports.Dashboard(r.Context(), ports.DashboardQuery{Range: rg, LowStockThreshold: low, TopLimit: top})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toDashboard(*d))
}

func (h *Handler) salesReport(w http.ResponseWriter, r *http.Request) {
	rg, err := h.dateRange(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		writeError(w, r, h.log, domain.NewValidationError("format", "must be json or csv"))
		return
	}
	lines, err := h.svc.Reports.SalesReport(r.Context(), rg)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}

	if format == "json" {
		out := make([]reportLineResponse, len(lines))
		for i, l := range lines {
			out[i] = reportLineResponse{l.SaleID, l.SoldAt, l.SKU, l.Product, l.Quantity, l.UnitPrice.Cents(), l.Subtotal.Cents()}
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	filename := fmt.Sprintf("sales_%s_%s.csv", rg.From.Format("20060102"), rg.To.AddDate(0, 0, -1).Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"sale_id", "sold_at", "sku", "product", "quantity", "unit_price", "subtotal"})
	for _, l := range lines {
		_ = cw.Write([]string{
			strconv.FormatInt(l.SaleID, 10), l.SoldAt.Format(time.RFC3339), l.SKU, sanitizeCSV(l.Product),
			strconv.Itoa(l.Quantity), l.UnitPrice.String(), l.Subtotal.String(),
		})
	}
	cw.Flush()
}

// sanitizeCSV previene "CSV injection": Excel ejecuta celdas que empiezan
// con = + - @ como fórmulas.
func sanitizeCSV(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// dateRange lee ?from=YYYY-MM-DD&to=YYYY-MM-DD (ambos inclusivos, UTC).
// Por defecto: últimos 30 días incluyendo hoy.
func (h *Handler) dateRange(r *http.Request) (domain.DateRange, error) {
	const layout = "2006-01-02"
	q := r.URL.Query()
	today := h.clock.Now().UTC().Truncate(24 * time.Hour)
	to := today.AddDate(0, 0, 1)
	from := today.AddDate(0, 0, -29)

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(layout, v)
		if err != nil {
			return domain.DateRange{}, domain.NewValidationError("from", "must have format YYYY-MM-DD")
		}
		from = t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(layout, v)
		if err != nil {
			return domain.DateRange{}, domain.NewValidationError("to", "must have format YYYY-MM-DD")
		}
		to = t.AddDate(0, 0, 1) // 'to' inclusivo -> límite exclusivo del día siguiente
	}
	return domain.NewDateRange(from, to)
}
