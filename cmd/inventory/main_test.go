package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/sqlite"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// TestWireAndSeed verifica que el grafo de dependencias se arma y que los
// datos de demostración pasan todas las reglas de negocio (y son idempotentes).
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
	app := wire(db)
	for i := 0; i < 2; i++ {
		if err := seed(ctx, app); err != nil {
			t.Fatalf("seed run %d: %v", i, err)
		}
	}
	ps, err := app.Products.List(ctx, ports.ProductFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 5 {
		t.Fatalf("expected 5 demo products (seed must be idempotent), got %d", len(ps))
	}
}
