package gsheets

import (
	"context"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// TableRepository memetakan sheet "Tables" (denah meja bisnis F&B).
type TableRepository struct {
	p *Provider
}

// NewTableRepository membuat repository meja berbasis Sheets.
func NewTableRepository(p *Provider) *TableRepository {
	return &TableRepository{p: p}
}

var _ domain.TableRepository = (*TableRepository)(nil)

// List mengembalikan seluruh meja tenant.
func (r *TableRepository) List(ctx context.Context, tenantID string) ([]domain.Table, error) {
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := sh.SheetRows(ctx, SheetTables, len(tableHeader))
	if err != nil {
		return nil, apperr.Upstream("gagal membaca daftar meja").WithCause(err)
	}
	out := make([]domain.Table, 0, len(rows))
	for i, row := range rows {
		tb := tableFromRow(row, i+2)
		if tb.ID == "" {
			continue
		}
		out = append(out, tb)
	}
	return out, nil
}

// Get mencari meja berdasarkan ID.
func (r *TableRepository) Get(ctx context.Context, tenantID, tableID string) (*domain.Table, error) {
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == tableID {
			tb := items[i]
			return &tb, nil
		}
	}
	return nil, apperr.NotFound("meja tidak ditemukan")
}

// Create menambahkan meja baru.
func (r *TableRepository) Create(ctx context.Context, tenantID string, tb *domain.Table) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.Append(ctx, SheetTables, [][]any{tableToRow(*tb)}); err != nil {
		return apperr.Upstream("gagal menyimpan meja").WithCause(err)
	}
	return nil
}

// Update menimpa baris meja.
func (r *TableRepository) Update(ctx context.Context, tenantID string, tb *domain.Table) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	rowNumber := tb.RowNumber
	if rowNumber == 0 {
		existing, err := r.Get(ctx, tenantID, tb.ID)
		if err != nil {
			return err
		}
		rowNumber = existing.RowNumber
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.UpdateRow(ctx, SheetTables, rowNumber, tableToRow(*tb)); err != nil {
		return apperr.Upstream("gagal memperbarui meja").WithCause(err)
	}
	tb.RowNumber = rowNumber
	return nil
}

// Delete menghapus baris meja.
func (r *TableRepository) Delete(ctx context.Context, tenantID, tableID string) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	existing, err := r.Get(ctx, tenantID, tableID)
	if err != nil {
		return err
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.DeleteRow(ctx, SheetTables, existing.RowNumber); err != nil {
		return apperr.Upstream("gagal menghapus meja").WithCause(err)
	}
	return nil
}
