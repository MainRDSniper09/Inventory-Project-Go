package services_test

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

// Estos FAKES implementan los puertos de salida en memoria. Gracias a la
// arquitectura hexagonal podemos probar TODA la lógica de aplicación sin
// tocar SQLite: las pruebas son rápidas, deterministas y sin efectos.

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// memStore comparte estado entre los fakes, como lo haría una BD real.
type memStore struct {
	mu         sync.Mutex
	nextID     int64
	products   map[int64]domain.Product
	categories map[int64]domain.Category
	stock      map[int64]int
	movements  []domain.StockMovement
	sales      []domain.Sale

	failOn map[string]error // inyección de fallos por método
}

func newStore() *memStore {
	return &memStore{
		products:   map[int64]domain.Product{},
		categories: map[int64]domain.Category{},
		stock:      map[int64]int{},
		failOn:     map[string]error{},
	}
}

func (s *memStore) id() int64 { s.nextID++; return s.nextID }

// ---- TxManager: sin transacciones reales; registra cuántas veces se usó.
type fakeTx struct{ calls int }

func (f *fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

// ---- ProductRepository
type fakeProducts struct{ *memStore }

func (f fakeProducts) Create(_ context.Context, p *domain.Product) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failOn["products.Create"]; err != nil {
		return err
	}
	for _, e := range f.products {
		if e.SKU == p.SKU {
			return domain.ErrConflict
		}
	}
	p.ID = f.id()
	f.products[p.ID] = *p
	f.stock[p.ID] = 0
	return nil
}

func (f fakeProducts) Update(_ context.Context, p *domain.Product) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.products[p.ID]; !ok {
		return domain.ErrNotFound
	}
	f.products[p.ID] = *p
	return nil
}

func (f fakeProducts) GetByID(_ context.Context, id int64) (*domain.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &p, nil
}

func (f fakeProducts) GetWithStock(ctx context.Context, id int64) (*domain.ProductWithStock, error) {
	p, err := f.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return &domain.ProductWithStock{Product: *p, Quantity: f.stock[id]}, nil
}

func (f fakeProducts) List(_ context.Context, flt ports.ProductFilter) ([]domain.ProductWithStock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.ProductWithStock{}
	for _, p := range f.products {
		if !flt.IncludeArchived && !p.Active {
			continue
		}
		out = append(out, domain.ProductWithStock{Product: p, Quantity: f.stock[p.ID]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	lastLimit = flt.Limit
	return out, nil
}

var lastLimit int // permite verificar el clamp de paginación

func (f fakeProducts) Archive(_ context.Context, id int64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok || !p.Active {
		return domain.ErrNotFound
	}
	p.Active = false
	p.UpdatedAt = at
	f.products[id] = p
	return nil
}

// ---- CategoryRepository
type fakeCategories struct{ *memStore }

func (f fakeCategories) Create(_ context.Context, c *domain.Category) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c.ID = f.id()
	f.categories[c.ID] = *c
	return nil
}

func (f fakeCategories) GetByID(_ context.Context, id int64) (*domain.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.categories[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &c, nil
}

func (f fakeCategories) List(context.Context) ([]domain.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Category{}
	for _, c := range f.categories {
		out = append(out, c)
	}
	return out, nil
}

// ---- StockRepository
type fakeStock struct{ *memStore }

func (f fakeStock) Get(_ context.Context, id int64) (domain.Stock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	q, ok := f.stock[id]
	if !ok {
		return domain.Stock{}, domain.ErrNotFound
	}
	return domain.Stock{ProductID: id, Quantity: q}, nil
}

func (f fakeStock) RecordMovement(_ context.Context, m *domain.StockMovement) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failOn["stock.RecordMovement"]; err != nil {
		return err
	}
	if f.stock[m.ProductID]+m.Delta < 0 {
		return domain.ErrInsufficientStock
	}
	m.ID = f.id()
	f.stock[m.ProductID] += m.Delta
	f.movements = append(f.movements, *m)
	return nil
}

func (f fakeStock) ListMovements(_ context.Context, id int64, limit int) ([]domain.StockMovement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.StockMovement{}
	for _, m := range f.movements {
		if m.ProductID == id {
			out = append(out, m)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---- SaleRepository (simula el trigger que descuenta stock)
type fakeSales struct{ *memStore }

func (f fakeSales) Create(_ context.Context, s *domain.Sale) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s.ID = f.id()
	for i := range s.Items {
		s.Items[i].SaleID = s.ID
		f.stock[s.Items[i].ProductID] -= s.Items[i].Quantity
	}
	f.sales = append(f.sales, *s)
	return nil
}

func (f fakeSales) GetByID(_ context.Context, id int64) (*domain.Sale, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sales {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f fakeSales) List(_ context.Context, _ domain.DateRange, _ int) ([]domain.Sale, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.Sale(nil), f.sales...), nil
}

// ---- ReportRepository
type fakeReports struct {
	summary domain.DashboardSummary
	err     error
	gotTop  int
	gotLow  int
}

func (f *fakeReports) Summary(_ context.Context, r domain.DateRange, th int) (domain.DashboardSummary, error) {
	f.gotLow = th
	s := f.summary
	s.Range, s.LowStockThreshold = r, th
	return s, f.err
}

func (f *fakeReports) TopProducts(_ context.Context, _ domain.DateRange, limit int) ([]domain.ProductSales, error) {
	f.gotTop = limit
	return []domain.ProductSales{{SKU: "A"}}, nil
}

func (f *fakeReports) DailySales(context.Context, domain.DateRange) ([]domain.DailySales, error) {
	return []domain.DailySales{{Day: "2026-10-31"}}, nil
}

func (f *fakeReports) LowStock(context.Context, int, int) ([]domain.ProductWithStock, error) {
	return []domain.ProductWithStock{}, nil
}

func (f *fakeReports) SalesLines(context.Context, domain.DateRange) ([]domain.SaleReportLine, error) {
	return []domain.SaleReportLine{{SaleID: 1}}, f.err
}
