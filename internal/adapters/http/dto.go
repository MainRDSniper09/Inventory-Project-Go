package http

import (
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
)

// Los DTOs (Data Transfer Objects) desacoplan el contrato JSON público de las
// entidades internas: podemos refactorizar el dominio sin romper clientes.

// ---------- requests

type categoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type createProductRequest struct {
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	CategoryID   *int64 `json:"category_id"`
	PriceCents   int64  `json:"price_cents"`
	InitialStock int    `json:"initial_stock"`
}

type updateProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CategoryID  *int64 `json:"category_id"`
	PriceCents  int64  `json:"price_cents"`
}

type movementRequest struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
	Reason   string `json:"reason"`
}

type saleRequest struct {
	Notes string `json:"notes"`
	Items []struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	} `json:"items"`
}

// ---------- responses

type categoryResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func toCategory(c domain.Category) categoryResponse {
	return categoryResponse{ID: c.ID, Name: c.Name, Description: c.Description, CreatedAt: c.CreatedAt}
}

type productResponse struct {
	ID           int64     `json:"id"`
	SKU          string    `json:"sku"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	CategoryID   *int64    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	PriceCents   int64     `json:"price_cents"`
	Price        string    `json:"price"`
	Quantity     int       `json:"quantity"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toProduct(p domain.ProductWithStock) productResponse {
	return productResponse{
		ID: p.ID, SKU: p.SKU, Name: p.Name, Description: p.Description,
		CategoryID: p.CategoryID, CategoryName: p.CategoryName,
		PriceCents: p.Price.Cents(), Price: p.Price.String(), Quantity: p.Quantity,
		Active: p.Active, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toProducts(ps []domain.ProductWithStock) []productResponse {
	out := make([]productResponse, len(ps))
	for i, p := range ps {
		out[i] = toProduct(p)
	}
	return out
}

type movementResponse struct {
	ID         int64     `json:"id"`
	ProductID  int64     `json:"product_id"`
	Type       string    `json:"type"`
	Delta      int       `json:"delta"`
	Reason     string    `json:"reason"`
	SaleItemID *int64    `json:"sale_item_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func toMovement(m domain.StockMovement) movementResponse {
	return movementResponse{
		ID: m.ID, ProductID: m.ProductID, Type: string(m.Type), Delta: m.Delta,
		Reason: m.Reason, SaleItemID: m.SaleItemID, CreatedAt: m.CreatedAt,
	}
}

type saleItemResponse struct {
	ProductID      int64 `json:"product_id"`
	Quantity       int   `json:"quantity"`
	UnitPriceCents int64 `json:"unit_price_cents"`
	SubtotalCents  int64 `json:"subtotal_cents"`
}

type saleResponse struct {
	ID         int64              `json:"id"`
	Notes      string             `json:"notes"`
	TotalCents int64              `json:"total_cents"`
	Total      string             `json:"total"`
	UnitsSold  int                `json:"units_sold"`
	Items      []saleItemResponse `json:"items"`
	CreatedAt  time.Time          `json:"created_at"`
}

func toSale(s domain.Sale) saleResponse {
	items := make([]saleItemResponse, len(s.Items))
	for i, it := range s.Items {
		items[i] = saleItemResponse{
			ProductID: it.ProductID, Quantity: it.Quantity,
			UnitPriceCents: it.UnitPrice.Cents(), SubtotalCents: it.Subtotal().Cents(),
		}
	}
	return saleResponse{
		ID: s.ID, Notes: s.Notes, TotalCents: s.Total().Cents(), Total: s.Total().String(),
		UnitsSold: s.UnitsSold(), Items: items, CreatedAt: s.CreatedAt,
	}
}

type dashboardResponse struct {
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
	Summary struct {
		ActiveProducts      int    `json:"active_products"`
		UnitsInStock        int    `json:"units_in_stock"`
		InventoryValueCents int64  `json:"inventory_value_cents"`
		InventoryValue      string `json:"inventory_value"`
		SalesCount          int    `json:"sales_count"`
		UnitsSold           int    `json:"units_sold"`
		RevenueCents        int64  `json:"revenue_cents"`
		Revenue             string `json:"revenue"`
		LowStockThreshold   int    `json:"low_stock_threshold"`
		LowStockCount       int    `json:"low_stock_count"`
	} `json:"summary"`
	TopProducts []productSalesResponse `json:"top_products"`
	Daily       []dailyResponse        `json:"daily"`
	LowStock    []productResponse      `json:"low_stock"`
}

type productSalesResponse struct {
	ProductID    int64  `json:"product_id"`
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	UnitsSold    int    `json:"units_sold"`
	RevenueCents int64  `json:"revenue_cents"`
}

type dailyResponse struct {
	Day          string `json:"day"`
	SalesCount   int    `json:"sales_count"`
	UnitsSold    int    `json:"units_sold"`
	RevenueCents int64  `json:"revenue_cents"`
}

func toDashboard(d domain.Dashboard) dashboardResponse {
	var r dashboardResponse
	s := d.Summary
	r.From, r.To = s.Range.From, s.Range.To
	r.Summary.ActiveProducts = s.ActiveProducts
	r.Summary.UnitsInStock = s.UnitsInStock
	r.Summary.InventoryValueCents = s.InventoryValue.Cents()
	r.Summary.InventoryValue = s.InventoryValue.String()
	r.Summary.SalesCount = s.SalesCount
	r.Summary.UnitsSold = s.UnitsSold
	r.Summary.RevenueCents = s.Revenue.Cents()
	r.Summary.Revenue = s.Revenue.String()
	r.Summary.LowStockThreshold = s.LowStockThreshold
	r.Summary.LowStockCount = s.LowStockCount

	r.TopProducts = make([]productSalesResponse, len(d.TopProducts))
	for i, p := range d.TopProducts {
		r.TopProducts[i] = productSalesResponse{p.ProductID, p.SKU, p.Name, p.UnitsSold, p.Revenue.Cents()}
	}
	r.Daily = make([]dailyResponse, len(d.Daily))
	for i, x := range d.Daily {
		r.Daily[i] = dailyResponse{x.Day, x.SalesCount, x.UnitsSold, x.Revenue.Cents()}
	}
	r.LowStock = toProducts(d.LowStock)
	return r
}

type reportLineResponse struct {
	SaleID         int64     `json:"sale_id"`
	SoldAt         time.Time `json:"sold_at"`
	SKU            string    `json:"sku"`
	Product        string    `json:"product"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	SubtotalCents  int64     `json:"subtotal_cents"`
}
