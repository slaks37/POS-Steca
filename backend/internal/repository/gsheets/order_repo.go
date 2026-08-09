package gsheets

import (
	"context"
	"sort"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// OrderRepository memetakan sheet "Orders" (modul manajemen pesanan).
type OrderRepository struct {
	p *Provider
}

// NewOrderRepository membuat repository pesanan berbasis Sheets.
func NewOrderRepository(p *Provider) *OrderRepository {
	return &OrderRepository{p: p}
}

var _ domain.OrderRepository = (*OrderRepository)(nil)

// List mengembalikan pesanan sesuai filter, terbaru lebih dulu.
func (r *OrderRepository) List(ctx context.Context, tenantID string, f domain.OrderFilter) ([]domain.Order, error) {
	orders, err := r.all(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Order, 0, len(orders))
	for _, o := range orders {
		if f.Status != "" && o.Status != f.Status {
			continue
		}
		if f.Source != "" && o.Source != f.Source {
			continue
		}
		if !f.From.IsZero() && o.CreatedAt.Before(f.From) {
			continue
		}
		if !f.To.IsZero() && !o.CreatedAt.Before(f.To) {
			continue
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Get mencari satu pesanan berdasarkan ID.
func (r *OrderRepository) Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error) {
	orders, err := r.all(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range orders {
		if orders[i].ID == orderID {
			o := orders[i]
			return &o, nil
		}
	}
	return nil, apperr.NotFound("pesanan tidak ditemukan")
}

// Create menambahkan pesanan baru.
func (r *OrderRepository) Create(ctx context.Context, tenantID string, o *domain.Order) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.Append(ctx, SheetOrders, [][]any{orderToRow(*o)}); err != nil {
		return apperr.Upstream("gagal menyimpan pesanan").WithCause(err)
	}
	return nil
}

// Update menimpa baris pesanan yang sudah ada.
func (r *OrderRepository) Update(ctx context.Context, tenantID string, o *domain.Order) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	rowNumber := o.RowNumber
	if rowNumber == 0 {
		existing, err := r.Get(ctx, tenantID, o.ID)
		if err != nil {
			return err
		}
		rowNumber = existing.RowNumber
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.UpdateRow(ctx, SheetOrders, rowNumber, orderToRow(*o)); err != nil {
		return apperr.Upstream("gagal memperbarui pesanan").WithCause(err)
	}
	o.RowNumber = rowNumber
	return nil
}

func (r *OrderRepository) all(ctx context.Context, tenantID string) ([]domain.Order, error) {
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := sh.SheetRows(ctx, SheetOrders, len(orderHeader))
	if err != nil {
		return nil, apperr.Upstream("gagal membaca daftar pesanan").WithCause(err)
	}
	orders := make([]domain.Order, 0, len(rows))
	for i, row := range rows {
		o := orderFromRow(row, i+2)
		if o.ID == "" {
			continue
		}
		orders = append(orders, o)
	}
	return orders, nil
}
