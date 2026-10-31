package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// demoClock es un reloj controlable: nos permite "viajar en el tiempo" para
// generar historial de ventas. Es un buen ejemplo de por qué Clock es un
// PUERTO: el mismo caso de uso funciona con el reloj real o con este.
type demoClock struct{ t time.Time }

func (c *demoClock) Now() time.Time { return c.t }

const demoDays = 14

// seed carga datos de demostración SÓLO si no hay productos. Usa los mismos
// casos de uso que la API, así los datos pasan por todas las validaciones.
// Genera ~2 semanas de ventas deterministas (misma semilla = mismos datos).
func seed(ctx context.Context, db *sql.DB, now time.Time) error {
	start := now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -(demoDays - 1))
	clk := &demoClock{t: start.Add(8 * time.Hour)}
	app := wire(db, clk)

	existing, err := app.Products.List(ctx, ports.ProductFilter{Limit: 1, IncludeArchived: true})
	if err != nil || len(existing) > 0 {
		return err
	}

	catIDs := map[string]int64{}
	for _, c := range []struct{ name, desc string }{
		{"Bebidas", "Refrescos, agua y jugos"},
		{"Snacks", "Botanas y galletas"},
		{"Limpieza", "Artículos de aseo"},
		{"Abarrotes", "Despensa básica"},
	} {
		created, err := app.Categories.Create(ctx, c.name, c.desc)
		if err != nil {
			return err
		}
		catIDs[c.name] = created.ID
	}

	type demo struct {
		sku, name, cat string
		price          int64
		stock          int
		weight         int // popularidad relativa en las ventas simuladas
	}
	catalog := []demo{
		{"BEB-COLA-600", "Refresco cola 600ml", "Bebidas", 1800, 160, 9},
		{"BEB-AGUA-1L", "Agua natural 1L", "Bebidas", 1200, 150, 8},
		{"BEB-JUGO-NAR", "Jugo de naranja 1L", "Bebidas", 3200, 50, 4},
		{"SNK-PAPAS-45", "Papas fritas 45g", "Snacks", 1500, 90, 6},
		{"SNK-GALL-CHOC", "Galletas de chocolate", "Snacks", 2200, 4, 0}, // queda con stock bajo
		{"SNK-MANI-100", "Maní salado 100g", "Snacks", 1900, 40, 2},
		{"LIM-DET-1KG", "Detergente en polvo 1kg", "Limpieza", 4500, 30, 2},
		{"LIM-JAB-3PK", "Jabón de tocador x3", "Limpieza", 3900, 12, 1},
		{"ABA-ARROZ-1KG", "Arroz blanco 1kg", "Abarrotes", 2800, 70, 4},
		{"ABA-ACEITE-1L", "Aceite vegetal 1L", "Abarrotes", 7600, 25, 2},
	}

	type pick struct {
		id     int64
		weight int
	}
	var picks []pick
	totalWeight := 0
	for _, d := range catalog {
		id := catIDs[d.cat]
		p, err := app.Products.Create(ctx, ports.CreateProductCmd{
			SKU: d.sku, Name: d.name, CategoryID: &id, PriceCents: d.price, InitialStock: d.stock,
		})
		if err != nil {
			return err
		}
		if d.weight > 0 {
			picks = append(picks, pick{p.ID, d.weight})
			totalWeight += d.weight
		}
	}

	rng := rand.New(rand.NewPCG(2026, 10)) //nolint:gosec // datos demo, no criptografía
	choose := func() int64 {
		n := rng.IntN(totalWeight)
		for _, p := range picks {
			if n < p.weight {
				return p.id
			}
			n -= p.weight
		}
		return picks[0].id
	}

	for day := 0; day < demoDays; day++ {
		sales := 2 + rng.IntN(4)
		if wd := start.AddDate(0, 0, day).Weekday(); wd == time.Saturday || wd == time.Sunday {
			sales += 2 // más movimiento en fin de semana
		}
		// Horas ordenadas: los IDs de venta siguen el orden cronológico.
		times := make([]time.Time, sales)
		for s := range times {
			times[s] = start.AddDate(0, 0, day).Add(time.Duration(9*60+rng.IntN(11*60)) * time.Minute)
			if times[s].After(now) { // nada de ventas "en el futuro" dentro del día de hoy
				times[s] = now.Add(-time.Duration(rng.IntN(120)+1) * time.Minute)
			}
		}
		slices.SortFunc(times, time.Time.Compare)
		for _, at := range times {
			clk.t = at
			lines := make([]ports.SaleLineCmd, 1+rng.IntN(3))
			for i := range lines {
				lines[i] = ports.SaleLineCmd{ProductID: choose(), Quantity: 1 + rng.IntN(3)}
			}
			_, err := app.Sales.Register(ctx, ports.RegisterSaleCmd{Notes: "demo", Items: lines})
			if errors.Is(err, domain.ErrInsufficientStock) {
				continue // el azar agotó un producto: se omite esa venta
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
