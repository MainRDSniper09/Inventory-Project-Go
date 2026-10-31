package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/http"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/sqlite"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/services"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

var now = time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)

// newServer arma el hexágono completo con una BD real temporal.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	clk := fixedClock{now}
	tx := sqlite.NewTxManager(db)
	pr, cr, sr := sqlite.NewProductRepository(db), sqlite.NewCategoryRepository(db), sqlite.NewStockRepository(db)
	svc := httpadapter.Services{
		Categories: services.NewCategoryService(cr, clk),
		Products:   services.NewProductService(pr, cr, sr, tx, clk),
		Inventory:  services.NewInventoryService(pr, sr, tx, clk),
		Sales:      services.NewSaleService(pr, sr, sqlite.NewSaleRepository(db), tx, clk),
		Reports:    services.NewReportService(sqlite.NewReportRepository(db)),
	}
	srv := httptest.NewServer(httpadapter.NewHandler(svc, clk, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		if s, ok := body.(string); ok {
			r = strings.NewReader(s)
		} else {
			b, _ := json.Marshal(body)
			r = bytes.NewReader(b)
		}
	}
	req, _ := http.NewRequest(method, srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, out
}

func decode[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return v
}

type product struct {
	ID       int64  `json:"id"`
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Price    string `json:"price"`
	Category string `json:"category_name"`
}

type apiErr struct {
	Error struct{ Code, Message, Field string } `json:"error"`
}

func TestAPI_FullFlow(t *testing.T) {
	srv := newServer(t)

	// health + UI estática
	if code, _ := do(t, srv, "GET", "/healthz", nil); code != 200 {
		t.Fatalf("healthz: %d", code)
	}
	if code, body := do(t, srv, "GET", "/", nil); code != 200 || !strings.Contains(string(body), "Inventario") {
		t.Fatalf("index: %d", code)
	}

	// categoría
	code, body := do(t, srv, "POST", "/api/v1/categories", map[string]any{"name": "Drinks"})
	if code != 201 {
		t.Fatalf("create category: %d %s", code, body)
	}
	cat := decode[struct{ ID int64 }](t, body)
	if code, _ := do(t, srv, "POST", "/api/v1/categories", map[string]any{"name": "drinks"}); code != 409 {
		t.Fatalf("duplicate category (case-insensitive) should be 409, got %d", code)
	}

	// crear producto con stock inicial
	code, body = do(t, srv, "POST", "/api/v1/products", map[string]any{
		"sku": "cola-1", "name": "Cola", "price_cents": 1850, "initial_stock": 10, "category_id": cat.ID,
	})
	if code != 201 {
		t.Fatalf("create product: %d %s", code, body)
	}
	p := decode[product](t, body)
	if p.SKU != "COLA-1" || p.Quantity != 10 || p.Price != "18.50" || p.Category != "Drinks" {
		t.Fatalf("unexpected product: %+v", p)
	}

	// listar / buscar
	_, body = do(t, srv, "GET", "/api/v1/products?q=cola", nil)
	if list := decode[[]product](t, body); len(list) != 1 {
		t.Fatalf("expected 1 product, got %d", len(list))
	}

	// actualizar
	code, body = do(t, srv, "PUT", "/api/v1/products/1", map[string]any{"name": "Cola Zero", "price_cents": 2000})
	if code != 200 || decode[product](t, body).Name != "Cola Zero" {
		t.Fatalf("update: %d %s", code, body)
	}

	// movimiento manual
	code, body = do(t, srv, "POST", "/api/v1/products/1/movements", map[string]any{"type": "in", "quantity": 5})
	if code != 201 {
		t.Fatalf("movement: %d %s", code, body)
	}
	code, body = do(t, srv, "POST", "/api/v1/products/1/movements", map[string]any{"type": "OUT", "quantity": 100})
	if code != 409 || decode[apiErr](t, body).Error.Code != "insufficient_stock" {
		t.Fatalf("expected insufficient_stock, got %d %s", code, body)
	}

	// venta
	code, body = do(t, srv, "POST", "/api/v1/sales", map[string]any{
		"notes": "web", "items": []map[string]any{{"product_id": 1, "quantity": 4}},
	})
	if code != 201 {
		t.Fatalf("sale: %d %s", code, body)
	}
	sale := decode[struct {
		ID         int64 `json:"id"`
		TotalCents int64 `json:"total_cents"`
	}](t, body)
	if sale.TotalCents != 8000 {
		t.Fatalf("expected total 8000, got %d", sale.TotalCents)
	}
	if code, _ := do(t, srv, "GET", "/api/v1/sales/1", nil); code != 200 {
		t.Fatalf("get sale: %d", code)
	}
	_, body = do(t, srv, "GET", "/api/v1/sales", nil)
	if s := decode[[]any](t, body); len(s) != 1 {
		t.Fatalf("list sales: %s", body)
	}

	_, body = do(t, srv, "GET", "/api/v1/products/1", nil)
	if got := decode[product](t, body).Quantity; got != 11 {
		t.Fatalf("expected stock 10+5-4=11, got %d", got)
	}

	// kardex: IN inicial, IN manual, SALE
	_, body = do(t, srv, "GET", "/api/v1/products/1/movements", nil)
	if m := decode[[]map[string]any](t, body); len(m) != 3 || m[0]["type"] != "SALE" {
		t.Fatalf("unexpected movements: %s", body)
	}

	// dashboard
	code, body = do(t, srv, "GET", "/api/v1/dashboard?from=2026-10-01&to=2026-10-31", nil)
	if code != 200 {
		t.Fatalf("dashboard: %d %s", code, body)
	}
	dash := decode[struct {
		Summary struct {
			SalesCount   int   `json:"sales_count"`
			UnitsSold    int   `json:"units_sold"`
			RevenueCents int64 `json:"revenue_cents"`
		} `json:"summary"`
		TopProducts []struct{ SKU string } `json:"top_products"`
	}](t, body)
	if dash.Summary.SalesCount != 1 || dash.Summary.UnitsSold != 4 || dash.Summary.RevenueCents != 8000 || len(dash.TopProducts) != 1 {
		t.Fatalf("unexpected dashboard: %s", body)
	}

	// reportes
	_, body = do(t, srv, "GET", "/api/v1/reports/sales?from=2026-10-31&to=2026-10-31", nil)
	if lines := decode[[]any](t, body); len(lines) != 1 {
		t.Fatalf("report json: %s", body)
	}
	res, err := srv.Client().Get(srv.URL + "/api/v1/reports/sales?format=csv&from=2026-10-31&to=2026-10-31")
	if err != nil {
		t.Fatal(err)
	}
	csvBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") ||
		!strings.Contains(res.Header.Get("Content-Disposition"), "sales_20261031_20261031.csv") ||
		!strings.Contains(string(csvBody), "COLA-1") || !strings.Contains(string(csvBody), "80.00") {
		t.Fatalf("unexpected csv: %v %s", res.Header, csvBody)
	}

	// archivar
	if code, _ := do(t, srv, "DELETE", "/api/v1/products/1", nil); code != 204 {
		t.Fatalf("archive: %d", code)
	}
	if code, _ := do(t, srv, "DELETE", "/api/v1/products/1", nil); code != 404 {
		t.Fatalf("archive twice: %d", code)
	}
	code, body = do(t, srv, "POST", "/api/v1/sales", map[string]any{"items": []map[string]any{{"product_id": 1, "quantity": 1}}})
	if code != 400 {
		t.Fatalf("selling archived product should be 400, got %d %s", code, body)
	}
}

func TestAPI_InputErrors(t *testing.T) {
	srv := newServer(t)
	cases := []struct {
		method, path string
		body         any
		want         int
		field        string
	}{
		{"POST", "/api/v1/products", `{"sku":`, 400, "body"},
		{"POST", "/api/v1/products", `{"sku":"A","name":"x","unknown":1}`, 400, "body"},
		{"POST", "/api/v1/products", `{"sku":"A","name":"x"}{"x":1}`, 400, "body"},
		{"POST", "/api/v1/products", map[string]any{"sku": "A B", "name": "x"}, 400, "sku"},
		{"GET", "/api/v1/products/abc", nil, 400, "id"},
		{"GET", "/api/v1/products/0", nil, 400, "id"},
		{"GET", "/api/v1/products/99", nil, 404, ""},
		{"GET", "/api/v1/products?limit=x", nil, 400, "limit"},
		{"GET", "/api/v1/products?category_id=x", nil, 400, "category_id"},
		{"GET", "/api/v1/dashboard?from=31-10-2026", nil, 400, "from"},
		{"GET", "/api/v1/dashboard?from=2026-10-10&to=2026-10-01", nil, 400, "range"},
		{"GET", "/api/v1/reports/sales?format=xml", nil, 400, "format"},
		{"POST", "/api/v1/sales", map[string]any{"items": []any{}}, 400, "items"},
		{"POST", "/api/v1/products/1/movements", map[string]any{"type": "SALE", "quantity": 1}, 400, "type"},
	}
	for _, c := range cases {
		code, body := do(t, srv, c.method, c.path, c.body)
		if code != c.want {
			t.Errorf("%s %s: expected %d, got %d (%s)", c.method, c.path, c.want, code, body)
			continue
		}
		if c.field != "" {
			if got := decode[apiErr](t, body).Error.Field; got != c.field {
				t.Errorf("%s %s: expected field %q, got %q", c.method, c.path, c.field, got)
			}
		}
	}
	if code, _ := do(t, srv, "PATCH", "/api/v1/products/1", nil); code != http.StatusMethodNotAllowed {
		t.Errorf("PATCH should be 405, got %d", code)
	}
}

// ---- errores internos: el cliente NO debe ver detalles

type brokenReports struct{ ports.ReportService }

func (brokenReports) Dashboard(context.Context, ports.DashboardQuery) (*domain.Dashboard, error) {
	return nil, errors.New("secret connection string leaked")
}

type panicCategories struct{ ports.CategoryService }

func (panicCategories) List(context.Context) ([]domain.Category, error) { panic("boom") }

func TestAPI_InternalErrorsAreHidden(t *testing.T) {
	h := httpadapter.NewHandler(httpadapter.Services{Reports: brokenReports{}, Categories: panicCategories{}},
		fixedClock{now}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, path := range []string{"/api/v1/dashboard", "/api/v1/categories"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 500 || strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), "internal") {
			t.Fatalf("%s: unexpected: %d %s", path, rec.Code, rec.Body)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers missing")
	}
}
