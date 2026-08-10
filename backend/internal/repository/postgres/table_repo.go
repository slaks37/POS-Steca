package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

const tableColumns = `id, name, capacity, status, area, active_order_id, updated_at`

// TableRepository memetakan tabel tables (denah meja F&B).
type TableRepository struct {
	pool *pgxpool.Pool
}

// NewTableRepository membuat repository meja berbasis PostgreSQL.
func NewTableRepository(pool *pgxpool.Pool) *TableRepository {
	return &TableRepository{pool: pool}
}

var _ domain.TableRepository = (*TableRepository)(nil)

// List mengembalikan seluruh meja milik tenant. Pengurutan natural
// ("Meja 2" sebelum "Meja 10") tetap ditangani service agar aturannya sama
// untuk semua driver penyimpanan.
func (r *TableRepository) List(ctx context.Context, tenantID string) ([]domain.Table, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+tableColumns+` FROM tables WHERE tenant_id = $1 ORDER BY lower(area), lower(name)`,
		tenantID)
	if err != nil {
		return nil, wrapDB(err, "meja")
	}
	defer rows.Close()

	out := []domain.Table{}
	for rows.Next() {
		tb, err := scanTable(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tb)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "meja")
	}
	return out, nil
}

// Get mengambil satu meja milik tenant.
func (r *TableRepository) Get(ctx context.Context, tenantID, tableID string) (*domain.Table, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+tableColumns+` FROM tables WHERE tenant_id = $1 AND id = $2`,
		tenantID, tableID)
	tb, err := scanTable(row)
	if err != nil {
		return nil, err
	}
	return &tb, nil
}

// Create menambahkan meja baru.
func (r *TableRepository) Create(ctx context.Context, tenantID string, tb *domain.Table) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tables (tenant_id, id, name, capacity, status, area, active_order_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, tb.ID, tb.Name, tb.Capacity, string(tb.Status), tb.Area,
		tb.ActiveOrderID, tb.UpdatedAt.UTC())
	if err != nil {
		return wrapDB(err, "meja")
	}
	return nil
}

// Update menimpa data meja milik tenant.
func (r *TableRepository) Update(ctx context.Context, tenantID string, tb *domain.Table) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE tables SET name = $3, capacity = $4, status = $5, area = $6,
			active_order_id = $7, updated_at = $8
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, tb.ID, tb.Name, tb.Capacity, string(tb.Status), tb.Area,
		tb.ActiveOrderID, tb.UpdatedAt.UTC())
	if err != nil {
		return wrapDB(err, "meja")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("meja tidak ditemukan")
	}
	return nil
}

// Delete menghapus meja milik tenant.
func (r *TableRepository) Delete(ctx context.Context, tenantID, tableID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM tables WHERE tenant_id = $1 AND id = $2`, tenantID, tableID)
	if err != nil {
		return wrapDB(err, "meja")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("meja tidak ditemukan")
	}
	return nil
}

func scanTable(row rowScanner) (domain.Table, error) {
	var tb domain.Table
	err := row.Scan(&tb.ID, &tb.Name, &tb.Capacity, &tb.Status, &tb.Area,
		&tb.ActiveOrderID, &tb.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Table{}, apperr.NotFound("meja tidak ditemukan")
		}
		return domain.Table{}, wrapDB(err, "meja")
	}
	if !tb.Status.Valid() {
		tb.Status = domain.TableStatusKosong
	}
	tb.UpdatedAt = localTime(tb.UpdatedAt)
	return tb, nil
}
