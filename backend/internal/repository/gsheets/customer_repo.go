package gsheets

import (
	"context"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// CustomerRepository memetakan sheet "Customers" (modul CRM & loyalitas).
type CustomerRepository struct {
	p *Provider
}

// NewCustomerRepository membuat repository pelanggan berbasis Sheets.
func NewCustomerRepository(p *Provider) *CustomerRepository {
	return &CustomerRepository{p: p}
}

var _ domain.CustomerRepository = (*CustomerRepository)(nil)

// List mengembalikan seluruh pelanggan tenant.
func (r *CustomerRepository) List(ctx context.Context, tenantID string) ([]domain.Customer, error) {
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := sh.SheetRows(ctx, SheetCustomers, len(customerHeader))
	if err != nil {
		return nil, apperr.Upstream("gagal membaca data pelanggan").WithCause(err)
	}
	out := make([]domain.Customer, 0, len(rows))
	for i, row := range rows {
		c := customerFromRow(row, i+2)
		if c.ID == "" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// Get mencari pelanggan berdasarkan ID.
func (r *CustomerRepository) Get(ctx context.Context, tenantID, customerID string) (*domain.Customer, error) {
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == customerID {
			c := items[i]
			return &c, nil
		}
	}
	return nil, apperr.NotFound("pelanggan tidak ditemukan")
}

// GetByPhone mencari pelanggan berdasarkan nomor HP, dipakai kasir untuk
// mengenali pelanggan lama tanpa perlu mencari manual.
func (r *CustomerRepository) GetByPhone(ctx context.Context, tenantID, phone string) (*domain.Customer, error) {
	phone = domain.NormalizePhone(phone)
	if phone == "" {
		return nil, apperr.NotFound("pelanggan tidak ditemukan")
	}
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if domain.NormalizePhone(items[i].Phone) == phone {
			c := items[i]
			return &c, nil
		}
	}
	return nil, apperr.NotFound("pelanggan tidak ditemukan")
}

// Create menambahkan pelanggan baru.
func (r *CustomerRepository) Create(ctx context.Context, tenantID string, c *domain.Customer) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.Append(ctx, SheetCustomers, [][]any{customerToRow(*c)}); err != nil {
		return apperr.Upstream("gagal menyimpan pelanggan").WithCause(err)
	}
	return nil
}

// Update menimpa baris pelanggan.
func (r *CustomerRepository) Update(ctx context.Context, tenantID string, c *domain.Customer) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	rowNumber := c.RowNumber
	if rowNumber == 0 {
		existing, err := r.Get(ctx, tenantID, c.ID)
		if err != nil {
			return err
		}
		rowNumber = existing.RowNumber
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.UpdateRow(ctx, SheetCustomers, rowNumber, customerToRow(*c)); err != nil {
		return apperr.Upstream("gagal memperbarui pelanggan").WithCause(err)
	}
	c.RowNumber = rowNumber
	return nil
}

// Delete menghapus baris pelanggan.
func (r *CustomerRepository) Delete(ctx context.Context, tenantID, customerID string) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	existing, err := r.Get(ctx, tenantID, customerID)
	if err != nil {
		return err
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.DeleteRow(ctx, SheetCustomers, existing.RowNumber); err != nil {
		return apperr.Upstream("gagal menghapus pelanggan").WithCause(err)
	}
	return nil
}
