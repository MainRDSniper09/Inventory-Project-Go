package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/sqlite"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/platform/clock"
)

// TestWireAndSeed verifica que el grafo de dependencias se arma, que los
// datos demo pasan todas las reglas de negocio, que generan historial de
// varios días sin fechas futuras y que el seed es idempotente.
func TestWireAndSeed(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := sqlite.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 31, 10, 30, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if err := seed(ctx, db, now); err != nil {
			t.Fatalf("seed run %d: %v", i, err)
		}
	}

	app := wire(db, clock.System{})
	ps, err := app.Products.List(ctx, ports.ProductFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 10 {
		t.Fatalf("expected 10 demo products (seed must be idempotent), got %d", len(ps))
	}

	rg, _ := domain.NewDateRange(now.AddDate(0, 0, -30), now.Add(time.Second))
	d, err := app.Reports.Dashboard(ctx, ports.DashboardQuery{Range: rg})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Daily) != demoDays {
		t.Fatalf("expected sales on %d days, got %d", demoDays, len(d.Daily))
	}
	if d.Summary.LowStockCount == 0 {
		t.Fatal("demo data should include at least one low-stock product")
	}

	all, _ := domain.NewDateRange(now.AddDate(-1, 0, 0), now.AddDate(1, 0, 0))
	sales, _ := app.Sales.List(ctx, all, 500)
	for _, s := range sales {
		if s.CreatedAt.After(now) {
			t.Fatalf("sale %d is in the future: %s", s.ID, s.CreatedAt)
		}
	}
}
