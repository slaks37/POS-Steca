package gsheets

import (
	"context"
	"strings"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// EmployeeRepository memetakan sheet "Employees".
type EmployeeRepository struct {
	p *Provider
}

// NewEmployeeRepository membuat repository karyawan berbasis Sheets.
func NewEmployeeRepository(p *Provider) *EmployeeRepository {
	return &EmployeeRepository{p: p}
}

var _ domain.EmployeeRepository = (*EmployeeRepository)(nil)

// List mengembalikan seluruh karyawan tenant.
func (r *EmployeeRepository) List(ctx context.Context, tenantID string) ([]domain.Employee, error) {
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := sh.SheetRows(ctx, SheetEmployees, len(employeeHeader))
	if err != nil {
		return nil, apperr.Upstream("gagal membaca data karyawan").WithCause(err)
	}
	out := make([]domain.Employee, 0, len(rows))
	for i, row := range rows {
		e := employeeFromRow(row, i+2)
		if e.ID == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// Get mencari karyawan berdasarkan ID.
func (r *EmployeeRepository) Get(ctx context.Context, tenantID, employeeID string) (*domain.Employee, error) {
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == employeeID {
			e := items[i]
			return &e, nil
		}
	}
	return nil, apperr.NotFound("karyawan tidak ditemukan")
}

// GetByEmail mencari karyawan berdasarkan email (dipakai saat login PIN).
func (r *EmployeeRepository) GetByEmail(ctx context.Context, tenantID, email string) (*domain.Employee, error) {
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for i := range items {
		if items[i].Email == email {
			e := items[i]
			return &e, nil
		}
	}
	return nil, apperr.NotFound("karyawan tidak ditemukan")
}

// Create menambahkan karyawan baru.
func (r *EmployeeRepository) Create(ctx context.Context, tenantID string, e *domain.Employee) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.Append(ctx, SheetEmployees, [][]any{employeeToRow(*e)}); err != nil {
		return apperr.Upstream("gagal menyimpan karyawan").WithCause(err)
	}
	return nil
}

// Update menimpa baris karyawan.
func (r *EmployeeRepository) Update(ctx context.Context, tenantID string, e *domain.Employee) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	rowNumber := e.RowNumber
	if rowNumber == 0 {
		existing, err := r.Get(ctx, tenantID, e.ID)
		if err != nil {
			return err
		}
		rowNumber = existing.RowNumber
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.UpdateRow(ctx, SheetEmployees, rowNumber, employeeToRow(*e)); err != nil {
		return apperr.Upstream("gagal memperbarui karyawan").WithCause(err)
	}
	e.RowNumber = rowNumber
	return nil
}

// Delete menghapus baris karyawan.
func (r *EmployeeRepository) Delete(ctx context.Context, tenantID, employeeID string) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	existing, err := r.Get(ctx, tenantID, employeeID)
	if err != nil {
		return err
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.DeleteRow(ctx, SheetEmployees, existing.RowNumber); err != nil {
		return apperr.Upstream("gagal menghapus karyawan").WithCause(err)
	}
	return nil
}
