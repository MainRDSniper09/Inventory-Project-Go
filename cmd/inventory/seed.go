package main

import (
	"context"

	httpadapter "github.com/MainRDSniper09/Inventory-Project-Go/internal/adapters/http"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// seed carga datos de demostración SÓLO si no hay productos. Usa los mismos
// casos de uso que la API: los datos pasan por todas las validaciones.
func seed(ctx context.Context, app httpadapter.Services) error {
	existing, err := app.Products.List(ctx, ports.ProductFilter{Limit: 1, IncludeArchived: true})
	if err != nil || len(existing) > 0 {
		return err
	}

	catIDs := map[string]int64{}
	for _, name := range []string{"Bebidas", "Snacks", "Limpieza"} {
		c, err := app.Categories.Create(ctx, name, "")
		if err != nil {
			return err
		}
		catIDs[name] = c.ID
	}

	type demo struct {
		sku, name, cat string
		price          int64
		stock          int
	}
	ids := map[string]int64{}
	for _, d := range []demo{
		{"BEB-COLA-600", "Refresco cola 600ml", "Bebidas", 1800, 48},
		{"BEB-AGUA-1L", "Agua natural 1L", "Bebidas", 1200, 60},
		{"SNK-PAPAS-45", "Papas fritas 45g", "Snacks", 1500, 30},
		{"SNK-GALL-CHOC", "Galletas de chocolate", "Snacks", 2200, 4},
		{"LIM-DET-1KG", "Detergente en polvo 1kg", "Limpieza", 4500, 12},
	} {
		id := catIDs[d.cat]
		p, err := app.Products.Create(ctx, ports.CreateProductCmd{
			SKU: d.sku, Name: d.name, CategoryID: &id, PriceCents: d.price, InitialStock: d.stock,
		})
		if err != nil {
			return err
		}
		ids[d.sku] = p.ID
	}

	for _, sale := range [][]ports.SaleLineCmd{
		{{ProductID: ids["BEB-COLA-600"], Quantity: 3}, {ProductID: ids["SNK-PAPAS-45"], Quantity: 2}},
		{{ProductID: ids["BEB-AGUA-1L"], Quantity: 6}},
		{{ProductID: ids["LIM-DET-1KG"], Quantity: 1}, {ProductID: ids["BEB-COLA-600"], Quantity: 2}},
	} {
		if _, err := app.Sales.Register(ctx, ports.RegisterSaleCmd{Notes: "demo", Items: sale}); err != nil {
			return err
		}
	}
	return nil
}
