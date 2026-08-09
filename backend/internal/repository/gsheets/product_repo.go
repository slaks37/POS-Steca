package gsheets

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/googleapi"
)

// ProductRepository membaca dan menulis sheet "Products".
type ProductRepository struct {
	p        *Provider
	cacheTTL time.Duration

	mu    sync.RWMutex
	cache map[string]productCache
}

type productCache struct {
	items     []domain.Product
	expiresAt time.Time
}

// NewProductRepository membuat repository produk berbasis Google Sheets.
// Cache pendek dipakai karena kuota Sheets API terbatas dan katalog produk
// dibaca sangat sering oleh layar kasir.
func NewProductRepository(p *Provider, cacheTTL time.Duration) *ProductRepository {
	return &ProductRepository{p: p, cacheTTL: cacheTTL, cache: map[string]productCache{}}
}

var _ domain.ProductRepository = (*ProductRepository)(nil)

// List mengembalikan seluruh produk tenant.
func (r *ProductRepository) List(ctx context.Context, tenantID string) ([]domain.Product, error) {
	r.mu.RLock()
	if c, ok := r.cache[tenantID]; ok && time.Now().Before(c.expiresAt) {
		items := make([]domain.Product, len(c.items))
		copy(items, c.items)
		r.mu.RUnlock()
		return items, nil
	}
	r.mu.RUnlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := sh.SheetRows(ctx, SheetProducts, len(productHeader))
	if err != nil {
		return nil, apperr.Upstream("gagal membaca daftar produk").WithCause(err)
	}

	items := make([]domain.Product, 0, len(rows))
	for i, row := range rows {
		p := productFromRow(row, i+2) // +2: baris 1 header, indeks 0-based
		if p.ID == "" && p.Name == "" {
			continue
		}
		items = append(items, p)
	}

	r.mu.Lock()
	r.cache[tenantID] = productCache{items: items, expiresAt: time.Now().Add(r.cacheTTL)}
	r.mu.Unlock()

	out := make([]domain.Product, len(items))
	copy(out, items)
	return out, nil
}

func (r *ProductRepository) invalidate(tenantID string) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()
}

// Get mencari satu produk berdasarkan ID.
func (r *ProductRepository) Get(ctx context.Context, tenantID, productID string) (*domain.Product, error) {
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == productID {
			p := items[i]
			return &p, nil
		}
	}
	return nil, apperr.NotFound("produk tidak ditemukan")
}

// Create menambahkan produk baru sebagai baris di akhir sheet.
func (r *ProductRepository) Create(ctx context.Context, tenantID string, p *domain.Product) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.Append(ctx, SheetProducts, [][]any{productToRow(*p)}); err != nil {
		return apperr.Upstream("gagal menyimpan produk").WithCause(err)
	}
	r.invalidate(tenantID)
	return nil
}

// Update menimpa baris produk yang sudah ada.
func (r *ProductRepository) Update(ctx context.Context, tenantID string, p *domain.Product) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	rowNumber, err := r.rowNumberOf(ctx, tenantID, p.ID)
	if err != nil {
		return err
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.UpdateRow(ctx, SheetProducts, rowNumber, productToRow(*p)); err != nil {
		return apperr.Upstream("gagal memperbarui produk").WithCause(err)
	}
	p.RowNumber = rowNumber
	r.invalidate(tenantID)
	return nil
}

// Delete menghapus baris produk.
func (r *ProductRepository) Delete(ctx context.Context, tenantID, productID string) error {
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	rowNumber, err := r.rowNumberOf(ctx, tenantID, productID)
	if err != nil {
		return err
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := sh.DeleteRow(ctx, SheetProducts, rowNumber); err != nil {
		return apperr.Upstream("gagal menghapus produk").WithCause(err)
	}
	r.invalidate(tenantID)
	return nil
}

// AdjustStock menerapkan perubahan stok beberapa produk dalam satu batch.
func (r *ProductRepository) AdjustStock(ctx context.Context, tenantID string, deltas map[string]int) error {
	if len(deltas) == 0 {
		return nil
	}
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	r.invalidate(tenantID)
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return err
	}
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}

	stockCol := googleapi.ColumnLetter(5) // kolom "Stok"
	updates := make([]googleapi.CellUpdate, 0, len(deltas))
	for i := range items {
		delta, ok := deltas[items[i].ID]
		if !ok || delta == 0 {
			continue
		}
		newStock := items[i].Stock + delta
		if newStock < 0 {
			newStock = 0
		}
		updates = append(updates, googleapi.CellUpdate{
			Range:  fmt.Sprintf("'%s'!%s%d", SheetProducts, stockCol, items[i].RowNumber),
			Values: []any{newStock},
		})
	}
	if err := sh.BatchUpdate(ctx, updates); err != nil {
		return apperr.Upstream("gagal memperbarui stok").WithCause(err)
	}
	r.invalidate(tenantID)
	return nil
}

func (r *ProductRepository) rowNumberOf(ctx context.Context, tenantID, productID string) (int, error) {
	r.invalidate(tenantID)
	items, err := r.List(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	for _, p := range items {
		if strings.EqualFold(p.ID, productID) {
			return p.RowNumber, nil
		}
	}
	return 0, apperr.NotFound("produk tidak ditemukan")
}
