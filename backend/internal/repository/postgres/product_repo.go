package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// price di-cast ke float8 agar pemetaan ke float64 pada domain tidak
// bergantung pada konversi implisit driver terhadap tipe NUMERIC.
const productColumns = `id, name, category, price::float8, stock, sku, image_url, image_id`

// ProductRepository memetakan tabel products, selalu di-scope per tenant.
type ProductRepository struct {
	pool *pgxpool.Pool
}

// NewProductRepository membuat repository produk berbasis PostgreSQL.
func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

var _ domain.ProductRepository = (*ProductRepository)(nil)

// List mengembalikan seluruh produk milik satu tenant.
func (r *ProductRepository) List(ctx context.Context, tenantID string) ([]domain.Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+productColumns+` FROM products WHERE tenant_id = $1 ORDER BY lower(category), lower(name)`,
		tenantID)
	if err != nil {
		return nil, wrapDB(err, "produk")
	}
	defer rows.Close()

	out := []domain.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "produk")
	}
	return out, nil
}

// Get mengambil satu produk milik tenant.
func (r *ProductRepository) Get(ctx context.Context, tenantID, productID string) (*domain.Product, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+productColumns+` FROM products WHERE tenant_id = $1 AND id = $2`,
		tenantID, productID)
	p, err := scanProduct(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("produk tidak ditemukan")
		}
		return nil, err
	}
	return &p, nil
}

// Create menambahkan produk baru.
func (r *ProductRepository) Create(ctx context.Context, tenantID string, p *domain.Product) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO products (tenant_id, id, name, category, price, stock, sku, image_url, image_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID, p.ID, p.Name, p.Category, p.Price, p.Stock, p.SKU, p.ImageURL, p.ImageID)
	if err != nil {
		return wrapDB(err, "produk")
	}
	return nil
}

// Update mengubah produk milik tenant.
func (r *ProductRepository) Update(ctx context.Context, tenantID string, p *domain.Product) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE products SET
			name = $3, category = $4, price = $5, stock = $6,
			sku = $7, image_url = $8, image_id = $9, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, p.ID, p.Name, p.Category, p.Price, p.Stock, p.SKU, p.ImageURL, p.ImageID)
	if err != nil {
		return wrapDB(err, "produk")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("produk tidak ditemukan")
	}
	return nil
}

// Delete menghapus produk milik tenant.
func (r *ProductRepository) Delete(ctx context.Context, tenantID, productID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM products WHERE tenant_id = $1 AND id = $2`, tenantID, productID)
	if err != nil {
		return wrapDB(err, "produk")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("produk tidak ditemukan")
	}
	return nil
}

// AdjustStock menerapkan perubahan stok beberapa produk sekaligus dalam satu
// transaksi. GREATEST(...) menjaga stok tidak pernah negatif, sekaligus
// mencegah kondisi balapan antar kasir karena perhitungan terjadi di database.
func (r *ProductRepository) AdjustStock(ctx context.Context, tenantID string, deltas map[string]int) error {
	if len(deltas) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return wrapDB(err, "stok produk")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for productID, delta := range deltas {
		if delta == 0 {
			continue
		}
		batch.Queue(`
			UPDATE products SET stock = GREATEST(stock + $3, 0), updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, productID, delta)
	}
	if batch.Len() == 0 {
		return nil
	}

	results := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			results.Close()
			return wrapDB(err, "stok produk")
		}
	}
	if err := results.Close(); err != nil {
		return wrapDB(err, "stok produk")
	}
	if err := tx.Commit(ctx); err != nil {
		return wrapDB(err, "stok produk")
	}
	return nil
}

// scanProduct memetakan satu baris hasil query menjadi model domain.
func scanProduct(row rowScanner) (domain.Product, error) {
	var p domain.Product
	err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.Stock, &p.SKU, &p.ImageURL, &p.ImageID)
	if err != nil {
		if isNoRows(err) {
			return domain.Product{}, apperr.NotFound("produk tidak ditemukan")
		}
		return domain.Product{}, wrapDB(err, "produk")
	}
	p.Name = strings.TrimSpace(p.Name)
	return p, nil
}
