package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

const orderColumns = `id, code, transaction_id, status, source, customer_id, customer_name,
	table_id, table_no, items, total::float8, note, cashier, created_at, updated_at`

// OrderRepository memetakan tabel orders (modul manajemen pesanan).
type OrderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository membuat repository pesanan berbasis PostgreSQL.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

var _ domain.OrderRepository = (*OrderRepository)(nil)

// orderFilterClause menyusun potongan WHERE tambahan beserta argumennya.
// Argumen pertama ($1) selalu tenant_id, jadi penomoran dimulai dari $2.
// Dipisah sebagai fungsi murni supaya mudah diuji tanpa database.
func orderFilterClause(f domain.OrderFilter) (string, []any) {
	var (
		clauses []string
		args    []any
	)
	next := func() string { return fmt.Sprintf("$%d", len(args)+2) }

	if f.Status != "" {
		clauses = append(clauses, "status = "+next())
		args = append(args, string(f.Status))
	}
	if f.Source != "" {
		clauses = append(clauses, "source = "+next())
		args = append(args, string(f.Source))
	}
	if !f.From.IsZero() {
		clauses = append(clauses, "created_at >= "+next())
		args = append(args, f.From.UTC())
	}
	if !f.To.IsZero() {
		clauses = append(clauses, "created_at < "+next())
		args = append(args, f.To.UTC())
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

// List mengembalikan pesanan sesuai filter, terbaru lebih dulu.
func (r *OrderRepository) List(ctx context.Context, tenantID string, f domain.OrderFilter) ([]domain.Order, error) {
	extra, args := orderFilterClause(f)
	query := `SELECT ` + orderColumns + ` FROM orders WHERE tenant_id = $1` + extra +
		` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, append([]any{tenantID}, args...)...)
	if err != nil {
		return nil, wrapDB(err, "pesanan")
	}
	defer rows.Close()

	out := []domain.Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "pesanan")
	}
	return out, nil
}

// Get mengambil satu pesanan milik tenant.
func (r *OrderRepository) Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE tenant_id = $1 AND id = $2`,
		tenantID, orderID)
	o, err := scanOrder(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("pesanan tidak ditemukan")
		}
		return nil, err
	}
	return &o, nil
}

// Create menambahkan pesanan baru.
func (r *OrderRepository) Create(ctx context.Context, tenantID string, o *domain.Order) error {
	items, err := marshalItems(o.Items)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO orders (tenant_id, id, code, transaction_id, status, source,
			customer_id, customer_name, table_id, table_no, items, total, note, cashier,
			created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		tenantID, o.ID, o.Code, o.TransactionID, string(o.Status), string(o.Source),
		o.CustomerID, o.CustomerName, o.TableID, o.TableNo, items, o.Total, o.Note, o.Cashier,
		o.CreatedAt.UTC(), o.UpdatedAt.UTC())
	if err != nil {
		return wrapDB(err, "pesanan")
	}
	return nil
}

// Update menimpa pesanan milik tenant.
func (r *OrderRepository) Update(ctx context.Context, tenantID string, o *domain.Order) error {
	items, err := marshalItems(o.Items)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE orders SET
			code = $3, transaction_id = $4, status = $5, source = $6,
			customer_id = $7, customer_name = $8, table_id = $9, table_no = $10,
			items = $11, total = $12, note = $13, cashier = $14, updated_at = $15
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, o.ID, o.Code, o.TransactionID, string(o.Status), string(o.Source),
		o.CustomerID, o.CustomerName, o.TableID, o.TableNo, items, o.Total, o.Note, o.Cashier,
		o.UpdatedAt.UTC())
	if err != nil {
		return wrapDB(err, "pesanan")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("pesanan tidak ditemukan")
	}
	return nil
}

func marshalItems(items []domain.OrderItem) ([]byte, error) {
	if items == nil {
		items = []domain.OrderItem{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, apperr.Internal("gagal menyiapkan item pesanan").WithCause(err)
	}
	return raw, nil
}

func scanOrder(row rowScanner) (domain.Order, error) {
	var (
		o     domain.Order
		items []byte
	)
	err := row.Scan(&o.ID, &o.Code, &o.TransactionID, &o.Status, &o.Source,
		&o.CustomerID, &o.CustomerName, &o.TableID, &o.TableNo, &items,
		&o.Total, &o.Note, &o.Cashier, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Order{}, apperr.NotFound("pesanan tidak ditemukan")
		}
		return domain.Order{}, wrapDB(err, "pesanan")
	}

	if len(items) > 0 {
		if err := json.Unmarshal(items, &o.Items); err != nil {
			return domain.Order{}, apperr.Internal("item pesanan tidak bisa dibaca").WithCause(err)
		}
	}
	if o.Items == nil {
		o.Items = []domain.OrderItem{}
	}
	o.CreatedAt = localTime(o.CreatedAt)
	o.UpdatedAt = localTime(o.UpdatedAt)
	return o, nil
}
