package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/services"
)

var now = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

type env struct {
	store     *memStore
	tx        *fakeTx
	products  *services.ProductService
	cats      *services.CategoryService
	inventory *services.InventoryService
	sales     *services.SaleService
}

func newEnv() env {
	st := newStore()
	tx := &fakeTx{}
	clock := fixedClock{now}
	p, c, s, sa := fakeProducts{st}, fakeCategories{st}, fakeStock{st}, fakeSales{st}
	return env{
		store:     st,
		tx:        tx,
		products:  services.NewProductService(p, c, s, tx, clock),
		cats:      services.NewCategoryService(c, clock),
		inventory: services.NewInventoryService(p, s, tx, clock),
		sales:     services.NewSaleService(p, s, sa, tx, clock),
	}
}

func (e env) createProduct(t *testing.T, sku string, price int64, stock int) *domain.ProductWithStock {
	t.Helper()
	p, err := e.products.Create(context.Background(), ports.CreateProductCmd{
		SKU: sku, Name: "P " + sku, PriceCents: price, InitialStock: stock,
	})
	if err != nil {
		t.Fatalf("create %s: %v", sku, err)
	}
	return p
}

// ------------------------------------------------------------ ProductService

func TestProductService_CreateWithInitialStock(t *testing.T) {
	e := newEnv()
	p := e.createProduct(t, "lap-1", 99900, 7)
	if p.SKU != "LAP-1" || p.Quantity != 7 || p.Price != 99900 {
		t.Fatalf("unexpected: %+v", p)
	}
	if len(e.store.movements) != 1 || e.store.movements[0].Reason != "stock inicial" {
		t.Fatalf("expected initial stock movement, got %+v", e.store.movements)
	}
	if e.tx.calls != 1 {
		t.Fatalf("create must run in a transaction, calls=%d", e.tx.calls)
	}
}

func TestProductService_CreateValidations(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	missing := int64(42)
	cases := []ports.CreateProductCmd{
		{SKU: "A", Name: "x", PriceCents: -1},
		{SKU: "A", Name: "x", InitialStock: -1},
		{SKU: "", Name: "x"},
		{SKU: "A", Name: "x", CategoryID: &missing},
	}
	for i, cmd := range cases {
		if _, err := e.products.Create(ctx, cmd); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("case %d: expected ErrInvalidInput, got %v", i, err)
		}
	}
	e.createProduct(t, "DUP", 1, 0)
	if _, err := e.products.Create(ctx, ports.CreateProductCmd{SKU: "dup", Name: "x"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestProductService_CreatePropagatesRepositoryErrors(t *testing.T) {
	e := newEnv()
	boom := errors.New("disk full")
	e.store.failOn["stock.RecordMovement"] = boom
	_, err := e.products.Create(context.Background(), ports.CreateProductCmd{SKU: "A", Name: "x", InitialStock: 1})
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped repo error, got %v", err)
	}
}

func TestProductService_WithCategory(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	cat, err := e.cats.Create(ctx, "Tech", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.products.Create(ctx, ports.CreateProductCmd{SKU: "A", Name: "x", CategoryID: &cat.ID})
	if err != nil || p.CategoryID == nil || *p.CategoryID != cat.ID {
		t.Fatalf("unexpected: %+v %v", p, err)
	}
	cats, _ := e.cats.List(ctx)
	if len(cats) != 1 {
		t.Fatalf("expected 1 category, got %d", len(cats))
	}
	if _, err := e.cats.Create(ctx, "", ""); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatal("empty category must fail")
	}
}

func TestProductService_UpdateGetListArchive(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	p := e.createProduct(t, "A", 100, 0)

	upd, err := e.products.Update(ctx, p.ID, ports.UpdateProductCmd{Name: "New", PriceCents: 200})
	if err != nil || upd.Name != "New" || upd.Price != 200 {
		t.Fatalf("unexpected update: %+v %v", upd, err)
	}
	if _, err := e.products.Update(ctx, 999, ports.UpdateProductCmd{Name: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := e.products.Update(ctx, p.ID, ports.UpdateProductCmd{Name: "x", PriceCents: -5}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid, got %v", err)
	}

	got, err := e.products.Get(ctx, p.ID)
	if err != nil || got.ID != p.ID {
		t.Fatalf("get failed: %v", err)
	}

	if _, err := e.products.List(ctx, ports.ProductFilter{Limit: 100000, Offset: -3}); err != nil {
		t.Fatal(err)
	}
	if lastLimit != 500 {
		t.Fatalf("limit must be clamped to 500, got %d", lastLimit)
	}
	_, _ = e.products.List(ctx, ports.ProductFilter{})
	if lastLimit != 50 {
		t.Fatalf("default limit must be 50, got %d", lastLimit)
	}

	if err := e.products.Archive(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.products.Archive(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second archive must be not found, got %v", err)
	}
	if _, err := e.products.Update(ctx, p.ID, ports.UpdateProductCmd{Name: "x"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("archived update must be invalid, got %v", err)
	}
}

// ---------------------------------------------------------- InventoryService

func TestInventoryService_RecordMovement(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	p := e.createProduct(t, "A", 100, 5)

	if _, err := e.inventory.RecordMovement(ctx, ports.RecordMovementCmd{ProductID: p.ID, Type: domain.MovementIn, Quantity: 3}); err != nil {
		t.Fatal(err)
	}
	if e.store.stock[p.ID] != 8 {
		t.Fatalf("expected 8, got %d", e.store.stock[p.ID])
	}

	_, err := e.inventory.RecordMovement(ctx, ports.RecordMovementCmd{ProductID: p.ID, Type: domain.MovementOut, Quantity: 9})
	var se *domain.StockError
	if !errors.As(err, &se) || se.Available != 8 || se.Requested != 9 {
		t.Fatalf("expected StockError, got %v", err)
	}

	if _, err := e.inventory.RecordMovement(ctx, ports.RecordMovementCmd{ProductID: 999, Type: domain.MovementIn, Quantity: 1}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := e.inventory.RecordMovement(ctx, ports.RecordMovementCmd{ProductID: p.ID, Type: "BAD", Quantity: 1}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected invalid, got %v", err)
	}

	movs, err := e.inventory.Movements(ctx, p.ID, 0)
	if err != nil || len(movs) != 2 {
		t.Fatalf("expected 2 movements, got %d %v", len(movs), err)
	}
	if _, err := e.inventory.Movements(ctx, 999, 10); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("movements of unknown product must be not found")
	}

	_ = e.products.Archive(ctx, p.ID)
	if _, err := e.inventory.RecordMovement(ctx, ports.RecordMovementCmd{ProductID: p.ID, Type: domain.MovementIn, Quantity: 1}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("archived product must reject movements, got %v", err)
	}
}

// --------------------------------------------------------------- SaleService

func TestSaleService_RegisterUsesCurrentPriceAndMergesLines(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	a := e.createProduct(t, "A", 250, 10)
	b := e.createProduct(t, "B", 1000, 2)

	sale, err := e.sales.Register(ctx, ports.RegisterSaleCmd{
		Notes: "counter",
		Items: []ports.SaleLineCmd{{ProductID: a.ID, Quantity: 2}, {ProductID: b.ID, Quantity: 1}, {ProductID: a.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sale.Items) != 2 || sale.Total() != 3*250+1000 {
		t.Fatalf("unexpected sale: %+v total=%d", sale.Items, sale.Total())
	}
	if e.store.stock[a.ID] != 7 || e.store.stock[b.ID] != 1 {
		t.Fatalf("stock not decremented: %v", e.store.stock)
	}
	got, err := e.sales.Get(ctx, sale.ID)
	if err != nil || got.ID != sale.ID {
		t.Fatalf("get: %v", err)
	}
	list, _ := e.sales.List(ctx, domain.DateRange{}, 0)
	if len(list) != 1 {
		t.Fatalf("expected 1 sale, got %d", len(list))
	}
}

func TestSaleService_RegisterFailures(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	a := e.createProduct(t, "A", 100, 3)
	archived := e.createProduct(t, "Z", 100, 3)
	_ = e.products.Archive(ctx, archived.ID)

	tests := []struct {
		name string
		cmd  ports.RegisterSaleCmd
		want error
	}{
		{"no items", ports.RegisterSaleCmd{}, domain.ErrInvalidInput},
		{"unknown product", ports.RegisterSaleCmd{Items: []ports.SaleLineCmd{{ProductID: 999, Quantity: 1}}}, domain.ErrNotFound},
		{"archived product", ports.RegisterSaleCmd{Items: []ports.SaleLineCmd{{ProductID: archived.ID, Quantity: 1}}}, domain.ErrInvalidInput},
		{"zero qty", ports.RegisterSaleCmd{Items: []ports.SaleLineCmd{{ProductID: a.ID, Quantity: 0}}}, domain.ErrInvalidInput},
		// 2 + 2 = 4 > 3: el chequeo de stock ocurre tras fusionar líneas.
		{"merged qty exceeds stock", ports.RegisterSaleCmd{Items: []ports.SaleLineCmd{{ProductID: a.ID, Quantity: 2}, {ProductID: a.ID, Quantity: 2}}}, domain.ErrInsufficientStock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := e.sales.Register(ctx, tt.cmd); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
	if e.store.stock[a.ID] != 3 {
		t.Fatalf("failed sales must not touch stock, got %d", e.store.stock[a.ID])
	}
}

// ------------------------------------------------------------- ReportService

func TestReportService_DashboardDefaultsAndClamps(t *testing.T) {
	rep := &fakeReports{summary: domain.DashboardSummary{SalesCount: 3}}
	svc := services.NewReportService(rep)
	ctx := context.Background()

	d, err := svc.Dashboard(ctx, ports.DashboardQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.gotLow != 5 || rep.gotTop != 5 || d.Summary.SalesCount != 3 || len(d.TopProducts) != 1 || len(d.Daily) != 1 {
		t.Fatalf("unexpected defaults: low=%d top=%d %+v", rep.gotLow, rep.gotTop, d)
	}
	_, _ = svc.Dashboard(ctx, ports.DashboardQuery{TopLimit: 1000, LowStockThreshold: 2})
	if rep.gotTop != 50 || rep.gotLow != 2 {
		t.Fatalf("top must clamp to 50, got %d", rep.gotTop)
	}

	lines, err := svc.SalesReport(ctx, domain.DateRange{})
	if err != nil || len(lines) != 1 {
		t.Fatalf("report: %v", err)
	}

	rep.err = errors.New("db down")
	if _, err := svc.Dashboard(ctx, ports.DashboardQuery{}); !errors.Is(err, rep.err) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
	if _, err := svc.SalesReport(ctx, domain.DateRange{}); !errors.Is(err, rep.err) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
