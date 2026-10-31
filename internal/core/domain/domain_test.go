package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
)

var now = time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)

func TestMoney(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cents int64
		want  string
	}{{0, "0.00"}, {5, "0.05"}, {123456, "1234.56"}, {-150, "-1.50"}}
	for _, c := range cases {
		if got := domain.Money(c.cents).String(); got != c.want {
			t.Errorf("Money(%d).String() = %q, want %q", c.cents, got, c.want)
		}
	}
	if domain.Money(250).Multiply(3) != 750 {
		t.Error("Multiply failed")
	}
	if _, err := domain.NewMoney(-1); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("negative money should be invalid, got %v", err)
	}
}

func TestNewProduct_Validation(t *testing.T) {
	t.Parallel()
	zero := int64(0)
	tests := []struct {
		name    string
		sku     string
		details domain.ProductDetails
		field   string // "" = válido
	}{
		{"valid", " ab-12 ", domain.ProductDetails{Name: "Laptop", Price: 100}, ""},
		{"empty sku", "", domain.ProductDetails{Name: "x"}, "sku"},
		{"bad sku chars", "AB 12", domain.ProductDetails{Name: "x"}, "sku"},
		{"long sku", strings.Repeat("A", 33), domain.ProductDetails{Name: "x"}, "sku"},
		{"empty name", "A1", domain.ProductDetails{Name: "   "}, "name"},
		{"long name", "A1", domain.ProductDetails{Name: strings.Repeat("n", 121)}, "name"},
		{"long description", "A1", domain.ProductDetails{Name: "x", Description: strings.Repeat("d", 501)}, "description"},
		{"negative price", "A1", domain.ProductDetails{Name: "x", Price: -1}, "price"},
		{"bad category", "A1", domain.ProductDetails{Name: "x", CategoryID: &zero}, "category_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := domain.NewProduct(tt.sku, tt.details, now)
			if tt.field == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if p.SKU != "AB-12" || !p.Active || !p.CreatedAt.Equal(now) {
					t.Fatalf("unexpected product: %+v", p)
				}
				return
			}
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.field {
				t.Fatalf("expected validation error on %q, got %v", tt.field, err)
			}
		})
	}
}

func TestProduct_Update(t *testing.T) {
	t.Parallel()
	p, _ := domain.NewProduct("A1", domain.ProductDetails{Name: "old", Price: 1}, now)
	later := now.Add(time.Hour)
	if err := p.Update(domain.ProductDetails{Name: "new", Price: 2}, later); err != nil {
		t.Fatal(err)
	}
	if p.Name != "new" || p.Price != 2 || !p.UpdatedAt.Equal(later) || p.SKU != "A1" {
		t.Fatalf("unexpected update result: %+v", p)
	}
	p.Active = false
	if err := p.Update(domain.ProductDetails{Name: "x"}, later); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("archived product must not be updated, got %v", err)
	}
}

func TestNewCategory(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewCategory(" ", "", now); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatal("empty name must fail")
	}
	if _, err := domain.NewCategory(strings.Repeat("c", 61), "", now); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatal("long name must fail")
	}
	c, err := domain.NewCategory(" Drinks ", " cold ", now)
	if err != nil || c.Name != "Drinks" || c.Description != "cold" {
		t.Fatalf("unexpected: %+v %v", c, err)
	}
}

func TestNewManualMovement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		typ       domain.MovementType
		qty       int
		reason    string
		wantDelta int
		wantErr   bool
	}{
		{"in", domain.MovementIn, 5, "", 5, false},
		{"out", domain.MovementOut, 5, "", -5, false},
		{"adjust negative", domain.MovementAdjust, -2, "count", -2, false},
		{"adjust positive", domain.MovementAdjust, 3, "count", 3, false},
		{"in zero", domain.MovementIn, 0, "", 0, true},
		{"out negative", domain.MovementOut, -1, "", 0, true},
		{"adjust zero", domain.MovementAdjust, 0, "count", 0, true},
		{"adjust no reason", domain.MovementAdjust, 2, "", 0, true},
		{"sale is not manual", domain.MovementSale, 1, "", 0, true},
		{"unknown type", "FOO", 1, "", 0, true},
		{"long reason", domain.MovementIn, 1, strings.Repeat("r", 201), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, err := domain.NewManualMovement(1, tt.typ, tt.qty, tt.reason, now)
			if tt.wantErr {
				if !errors.Is(err, domain.ErrInvalidInput) {
					t.Fatalf("expected invalid input, got %v", err)
				}
				return
			}
			if err != nil || m.Delta != tt.wantDelta {
				t.Fatalf("got %+v, %v", m, err)
			}
		})
	}
	if _, err := domain.NewManualMovement(0, domain.MovementIn, 1, "", now); err == nil {
		t.Fatal("product id 0 must fail")
	}
}

func TestStock_CanFulfill(t *testing.T) {
	t.Parallel()
	s := domain.Stock{Quantity: 3}
	if !s.CanFulfill(3) || s.CanFulfill(4) || s.CanFulfill(0) {
		t.Fatal("CanFulfill logic is wrong")
	}
}

func TestSale_AddItemMergesAndTotals(t *testing.T) {
	t.Parallel()
	s, err := domain.NewSale(" note ", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err == nil {
		t.Fatal("empty sale must be invalid")
	}
	_ = s.AddItem(1, 2, 100)
	_ = s.AddItem(2, 1, 50)
	_ = s.AddItem(1, 3, 100) // se fusiona con la línea del producto 1
	if len(s.Items) != 2 || s.Items[0].Quantity != 5 {
		t.Fatalf("items not merged: %+v", s.Items)
	}
	if s.Total() != 550 || s.UnitsSold() != 6 || s.Notes != "note" {
		t.Fatalf("total=%d units=%d", s.Total(), s.UnitsSold())
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		pid   int64
		qty   int
		price domain.Money
	}{{0, 1, 1}, {1, 0, 1}, {1, 1, -1}} {
		if err := s.AddItem(bad.pid, bad.qty, bad.price); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("AddItem(%v) should fail", bad)
		}
	}
}

func TestSale_MaxItems(t *testing.T) {
	t.Parallel()
	s, _ := domain.NewSale("", now)
	for i := 1; i <= 100; i++ {
		if err := s.AddItem(int64(i), 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddItem(101, 1, 1); err == nil {
		t.Fatal("101st product must fail")
	}
	if _, err := domain.NewSale(strings.Repeat("n", 201), now); err == nil {
		t.Fatal("long notes must fail")
	}
}

func TestDateRange(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewDateRange(now, now); err == nil {
		t.Fatal("empty range must fail")
	}
	r, err := domain.NewDateRange(now, now.Add(time.Hour))
	if err != nil || !r.From.Equal(now) {
		t.Fatalf("unexpected: %+v %v", r, err)
	}
}

func TestErrorsWrapping(t *testing.T) {
	t.Parallel()
	err := &domain.StockError{ProductID: 1, Requested: 5, Available: 2}
	if !errors.Is(err, domain.ErrInsufficientStock) || !strings.Contains(err.Error(), "requested 5") {
		t.Fatalf("bad StockError: %v", err)
	}
	if !strings.Contains(domain.NewValidationError("f", "m").Error(), "f: m") {
		t.Fatal("bad ValidationError message")
	}
}
