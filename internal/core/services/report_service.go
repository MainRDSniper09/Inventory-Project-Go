package services

import (
	"context"
	"fmt"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/domain"
	"github.com/MainRDSniper09/Inventory-Project-Go/internal/core/ports"
)

const (
	defaultLowStockThreshold = 5
	defaultTopLimit          = 5
	maxTopLimit              = 50
)

// ReportService implementa ports.ReportService.
type ReportService struct {
	reports ports.ReportRepository
}

var _ ports.ReportService = (*ReportService)(nil)

// NewReportService construye el servicio.
func NewReportService(r ports.ReportRepository) *ReportService {
	return &ReportService{reports: r}
}

// Dashboard compone todos los indicadores del tablero.
func (s *ReportService) Dashboard(ctx context.Context, q ports.DashboardQuery) (*domain.Dashboard, error) {
	if q.LowStockThreshold <= 0 {
		q.LowStockThreshold = defaultLowStockThreshold
	}
	if q.TopLimit <= 0 {
		q.TopLimit = defaultTopLimit
	}
	if q.TopLimit > maxTopLimit {
		q.TopLimit = maxTopLimit
	}

	summary, err := s.reports.Summary(ctx, q.Range, q.LowStockThreshold)
	if err != nil {
		return nil, fmt.Errorf("dashboard summary: %w", err)
	}
	top, err := s.reports.TopProducts(ctx, q.Range, q.TopLimit)
	if err != nil {
		return nil, fmt.Errorf("dashboard top products: %w", err)
	}
	daily, err := s.reports.DailySales(ctx, q.Range)
	if err != nil {
		return nil, fmt.Errorf("dashboard daily sales: %w", err)
	}
	low, err := s.reports.LowStock(ctx, q.LowStockThreshold, 20)
	if err != nil {
		return nil, fmt.Errorf("dashboard low stock: %w", err)
	}
	return &domain.Dashboard{Summary: summary, TopProducts: top, Daily: daily, LowStock: low}, nil
}

// SalesReport devuelve el detalle plano de ventas para exportar.
func (s *ReportService) SalesReport(ctx context.Context, r domain.DateRange) ([]domain.SaleReportLine, error) {
	lines, err := s.reports.SalesLines(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("sales report: %w", err)
	}
	return lines, nil
}
