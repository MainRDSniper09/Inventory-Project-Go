package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/sqlite"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

type repos struct {
	db         *sql.DB
	tx         *sqlite.TxManager
	products   *sqlite.ProductRepository
	categories *sqlite.CategoryRepository
	stock      *sqlite.StockRepository
	sales      *sqlite.SaleRepository
	reports    *sqlite.ReportRepository
}

// newRepos crea una BD nueva en un directorio temporal por prueba: las
// pruebas quedan aisladas y pueden correr en paralelo.
func newRepos(t *testing.T) repos {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repos{
		db:         db,
		tx:         sqlite.NewTxManager(db),
		products:   sqlite.NewProductRepository(db),
		categories: sqlite.NewCategoryRepository(db),
		stock:      sqlite.NewStockRepository(db),
		sales:      sqlite.NewSaleRepository(db),
		reports:    sqlite.NewReportRepository(db),
	}
}

func mustProduct(t *testing.T, r repos, sku string, price int64, catID *int64) *domain.Product {
	t.Helper()
	p, err := domain.NewProduct(sku, domain.ProductDetails{Name: "Product " + sku, Price: domain.Money(price), CategoryID: catID}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.products.Create(context.Background(), p); err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p
}

func addStock(t *testing.T, r repos, productID int64, qty int) {
	t.Helper()
	m, err := domain.NewManualMovement(productID, domain.MovementIn, qty, "restock", t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.stock.RecordMovement(context.Background(), m); err != nil {
		t.Fatalf("record movement: %v", err)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	t.Parallel()
	r := newRepos(t)
	if err := sqlite.Migrate(context.Background(), r.db); err != nil {
		t.Fatalf("second migrate should be a no-op: %v", err)
	}
}

func TestProduct_CreateGetUpdateArchive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)

	cat, _ := domain.NewCategory("Drinks", "", t0)
	if err := r.categories.Create(ctx, cat); err != nil {
		t.Fatal(err)
	}
	p := mustProduct(t, r, "COKE-1", 150, &cat.ID)

	got, err := r.products.GetWithStock(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SKU != "COKE-1" || got.CategoryName != "Drinks" || got.Quantity != 0 || !got.Active {
		t.Fatalf("unexpected product: %+v", got)
	}

	if err := p.Update(domain.ProductDetails{Name: "Coke 600ml", Price: 175}, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := r.products.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := r.products.GetByID(ctx, p.ID)
	if reloaded.Name != "Coke 600ml" || reloaded.Price != 175 || reloaded.CategoryID != nil {
		t.Fatalf("update not persisted: %+v", reloaded)
	}

	if err := r.products.Archive(ctx, p.ID, t0); err != nil {
		t.Fatal(err)
	}
	if err := r.products.Archive(ctx, p.ID, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("archiving twice should be ErrNotFound, got %v", err)
	}
	list, _ := r.products.List(ctx, ports.ProductFilter{Limit: 10})
	if len(list) != 0 {
		t.Fatalf("archived product should be hidden, got %d", len(list))
	}
	list, _ = r.products.List(ctx, ports.ProductFilter{Limit: 10, IncludeArchived: true})
	if len(list) != 1 {
		t.Fatalf("expected archived product when IncludeArchived, got %d", len(list))
	}
}

func TestProduct_DuplicateSKU_IsConflict(t *testing.T) {
	t.Parallel()
	r := newRepos(t)
	mustProduct(t, r, "DUP", 100, nil)
	p, _ := domain.NewProduct("DUP", domain.ProductDetails{Name: "x", Price: 1}, t0)
	if err := r.products.Create(context.Background(), p); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestProduct_UnknownCategory_IsInvalidInput(t *testing.T) {
	t.Parallel()
	r := newRepos(t)
	bad := int64(999)
	p, _ := domain.NewProduct("X1", domain.ProductDetails{Name: "x", Price: 1, CategoryID: &bad}, t0)
	if err := r.products.Create(context.Background(), p); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput from FK, got %v", err)
	}
}

func TestProduct_ListFiltersAndEscapesLike(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	mustProduct(t, r, "A-1", 1, nil)
	mustProduct(t, r, "B_2", 1, nil)
	mustProduct(t, r, "B-3", 1, nil)

	got, err := r.products.List(ctx, ports.ProductFilter{Query: "b_", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	// "_" es comodín en LIKE; al escaparlo sólo debe coincidir "B_2".
	if len(got) != 1 || got[0].SKU != "B_2" {
		t.Fatalf("expected only B_2, got %+v", got)
	}
}

func TestStock_TriggersMaintainInventory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	p := mustProduct(t, r, "SKU1", 100, nil)

	addStock(t, r, p.ID, 10)
	out, _ := domain.NewManualMovement(p.ID, domain.MovementOut, 3, "damaged", t0)
	if err := r.stock.RecordMovement(ctx, out); err != nil {
		t.Fatal(err)
	}
	st, err := r.stock.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Quantity != 7 {
		t.Fatalf("expected 7 units, got %d", st.Quantity)
	}

	movs, _ := r.stock.ListMovements(ctx, p.ID, 10)
	if len(movs) != 2 {
		t.Fatalf("expected 2 movements, got %d", len(movs))
	}
}

func TestStock_NegativeIsRejectedByDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	p := mustProduct(t, r, "SKU1", 100, nil)
	addStock(t, r, p.ID, 2)

	out, _ := domain.NewManualMovement(p.ID, domain.MovementOut, 5, "", t0)
	err := r.stock.RecordMovement(ctx, out)
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
	st, _ := r.stock.Get(ctx, p.ID)
	if st.Quantity != 2 {
		t.Fatalf("stock must be unchanged, got %d", st.Quantity)
	}
}

func TestStock_LedgerIsAppendOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	p := mustProduct(t, r, "SKU1", 100, nil)
	addStock(t, r, p.ID, 1)
	if _, err := r.db.ExecContext(ctx, `UPDATE stock_movements SET delta = 100`); err == nil {
		t.Fatal("updating the ledger must fail")
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM stock_movements`); err == nil {
		t.Fatal("deleting from the ledger must fail")
	}
}

func TestSale_CreateDecrementsStockAndComputesTotal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	a := mustProduct(t, r, "A", 250, nil)
	b := mustProduct(t, r, "B", 1000, nil)
	addStock(t, r, a.ID, 10)
	addStock(t, r, b.ID, 5)

	sale, _ := domain.NewSale("first", t0.Add(time.Hour))
	_ = sale.AddItem(a.ID, 4, a.Price)
	_ = sale.AddItem(b.ID, 1, b.Price)
	if err := r.sales.Create(ctx, sale); err != nil {
		t.Fatal(err)
	}

	stA, _ := r.stock.Get(ctx, a.ID)
	stB, _ := r.stock.Get(ctx, b.ID)
	if stA.Quantity != 6 || stB.Quantity != 4 {
		t.Fatalf("unexpected stock A=%d B=%d", stA.Quantity, stB.Quantity)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT total_cents FROM sales WHERE id = ?`, sale.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2000 || sale.Total() != 2000 {
		t.Fatalf("expected total 2000, db=%d domain=%d", total, sale.Total())
	}

	got, err := r.sales.GetByID(ctx, sale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Notes != "first" {
		t.Fatalf("unexpected sale: %+v", got)
	}

	movs, _ := r.stock.ListMovements(ctx, a.ID, 10)
	if movs[0].Type != domain.MovementSale || movs[0].Delta != -4 || movs[0].SaleItemID == nil {
		t.Fatalf("expected SALE movement first, got %+v", movs[0])
	}
}

func TestSale_InsufficientStockRollsBackEverything(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	a := mustProduct(t, r, "A", 100, nil)
	b := mustProduct(t, r, "B", 100, nil)
	addStock(t, r, a.ID, 10)
	addStock(t, r, b.ID, 1)

	sale, _ := domain.NewSale("", t0)
	_ = sale.AddItem(a.ID, 2, a.Price)
	_ = sale.AddItem(b.ID, 5, b.Price) // excede
	if err := r.sales.Create(ctx, sale); !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	stA, _ := r.stock.Get(ctx, a.ID)
	if stA.Quantity != 10 {
		t.Fatalf("partial sale must be rolled back; A=%d", stA.Quantity)
	}
	var n int
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sales`).Scan(&n)
	if n != 0 {
		t.Fatalf("no sale row should exist, got %d", n)
	}
}

func TestTxManager_RollbackOnError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	boom := errors.New("boom")
	err := r.tx.WithinTx(ctx, func(ctx context.Context) error {
		mustProductCtx(ctx, t, r, "TX1")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
	list, _ := r.products.List(ctx, ports.ProductFilter{Limit: 10, IncludeArchived: true})
	if len(list) != 0 {
		t.Fatal("product must not persist after rollback")
	}
}

func mustProductCtx(ctx context.Context, t *testing.T, r repos, sku string) {
	t.Helper()
	p, _ := domain.NewProduct(sku, domain.ProductDetails{Name: sku, Price: 1}, t0)
	if err := r.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
}

func TestReports_DashboardQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := newRepos(t)
	a := mustProduct(t, r, "A", 100, nil)
	b := mustProduct(t, r, "B", 500, nil)
	c := mustProduct(t, r, "C", 50, nil) // sin stock → low stock
	_ = c
	addStock(t, r, a.ID, 20)
	addStock(t, r, b.ID, 10)

	day1 := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	outside := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	for _, s := range []struct {
		at  time.Time
		p   *domain.Product
		qty int
	}{{day1, a, 3}, {day1, b, 1}, {day2, a, 2}, {outside, b, 1}} {
		sale, _ := domain.NewSale("", s.at)
		_ = sale.AddItem(s.p.ID, s.qty, s.p.Price)
		if err := r.sales.Create(ctx, sale); err != nil {
			t.Fatal(err)
		}
	}

	rg, _ := domain.NewDateRange(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))

	sum, err := r.reports.Summary(ctx, rg, 5)
	if err != nil {
		t.Fatal(err)
	}
	if sum.SalesCount != 3 || sum.UnitsSold != 6 || sum.Revenue != 1000 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	if sum.ActiveProducts != 3 || sum.UnitsInStock != 15+8 || sum.LowStockCount != 1 {
		t.Fatalf("unexpected stock summary: %+v", sum)
	}
	if sum.InventoryValue != 15*100+8*500 {
		t.Fatalf("unexpected inventory value: %d", sum.InventoryValue)
	}

	top, _ := r.reports.TopProducts(ctx, rg, 5)
	if len(top) != 2 || top[0].SKU != "A" || top[0].UnitsSold != 5 || top[0].Revenue != 500 {
		t.Fatalf("unexpected top products: %+v", top)
	}

	daily, _ := r.reports.DailySales(ctx, rg)
	if len(daily) != 2 || daily[0].Day != "2026-10-05" || daily[0].Revenue != 800 || daily[1].UnitsSold != 2 {
		t.Fatalf("unexpected daily: %+v", daily)
	}

	low, _ := r.reports.LowStock(ctx, 5, 10)
	if len(low) != 1 || low[0].SKU != "C" {
		t.Fatalf("unexpected low stock: %+v", low)
	}

	lines, _ := r.reports.SalesLines(ctx, rg)
	if len(lines) != 3 || lines[0].Subtotal != 300 {
		t.Fatalf("unexpected lines: %+v", lines)
	}

	sales, _ := r.sales.List(ctx, rg, 10)
	if len(sales) != 3 || len(sales[0].Items) != 1 {
		t.Fatalf("unexpected sales list: %+v", sales)
	}
}
