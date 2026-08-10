package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

const customerColumns = `id, name, phone, total_spent::float8, points, last_purchase, created_at`

// CustomerRepository memetakan tabel customers (CRM & loyalitas).
type CustomerRepository struct {
	pool *pgxpool.Pool
}

// NewCustomerRepository membuat repository pelanggan berbasis PostgreSQL.
func NewCustomerRepository(pool *pgxpool.Pool) *CustomerRepository {
	return &CustomerRepository{pool: pool}
}

var _ domain.CustomerRepository = (*CustomerRepository)(nil)

// List mengembalikan pelanggan satu tenant, terbesar total belanjanya dulu.
func (r *CustomerRepository) List(ctx context.Context, tenantID string) ([]domain.Customer, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+customerColumns+` FROM customers WHERE tenant_id = $1
		 ORDER BY total_spent DESC, lower(name)`, tenantID)
	if err != nil {
		return nil, wrapDB(err, "pelanggan")
	}
	defer rows.Close()

	out := []domain.Customer{}
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "pelanggan")
	}
	return out, nil
}

// Get mengambil pelanggan berdasarkan ID.
func (r *CustomerRepository) Get(ctx context.Context, tenantID, customerID string) (*domain.Customer, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+customerColumns+` FROM customers WHERE tenant_id = $1 AND id = $2`,
		tenantID, customerID)
	c, err := scanCustomer(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetByPhone mencari pelanggan lewat nomor HP yang sudah dinormalkan, sehingga
// "0812…", "+62812…", dan "62812…" mengarah ke pelanggan yang sama.
func (r *CustomerRepository) GetByPhone(ctx context.Context, tenantID, phone string) (*domain.Customer, error) {
	normalized := domain.NormalizePhone(phone)
	if normalized == "" {
		return nil, apperr.NotFound("pelanggan tidak ditemukan")
	}
	row := r.pool.QueryRow(ctx,
		`SELECT `+customerColumns+` FROM customers WHERE tenant_id = $1 AND phone_normalized = $2`,
		tenantID, normalized)
	c, err := scanCustomer(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create menambahkan pelanggan baru.
func (r *CustomerRepository) Create(ctx context.Context, tenantID string, c *domain.Customer) error {
	createdAt := c.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO customers (tenant_id, id, name, phone, phone_normalized,
			total_spent, points, last_purchase, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID, c.ID, c.Name, strings.TrimSpace(c.Phone), domain.NormalizePhone(c.Phone),
		c.TotalSpent, c.Points, timePtr(c.LastPurchase), createdAt.UTC())
	if err != nil {
		return wrapDB(err, "pelanggan")
	}
	return nil
}

// Update menimpa data pelanggan, termasuk akumulasi poin loyalitas.
func (r *CustomerRepository) Update(ctx context.Context, tenantID string, c *domain.Customer) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE customers SET
			name = $3, phone = $4, phone_normalized = $5,
			total_spent = $6, points = $7, last_purchase = $8
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, c.ID, c.Name, strings.TrimSpace(c.Phone), domain.NormalizePhone(c.Phone),
		c.TotalSpent, c.Points, timePtr(c.LastPurchase))
	if err != nil {
		return wrapDB(err, "pelanggan")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("pelanggan tidak ditemukan")
	}
	return nil
}

// Delete menghapus pelanggan milik tenant.
func (r *CustomerRepository) Delete(ctx context.Context, tenantID, customerID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM customers WHERE tenant_id = $1 AND id = $2`, tenantID, customerID)
	if err != nil {
		return wrapDB(err, "pelanggan")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("pelanggan tidak ditemukan")
	}
	return nil
}

func scanCustomer(row rowScanner) (domain.Customer, error) {
	var (
		c            domain.Customer
		lastPurchase *time.Time
	)
	err := row.Scan(&c.ID, &c.Name, &c.Phone, &c.TotalSpent, &c.Points, &lastPurchase, &c.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Customer{}, apperr.NotFound("pelanggan tidak ditemukan")
		}
		return domain.Customer{}, wrapDB(err, "pelanggan")
	}
	c.LastPurchase = nullableTime(lastPurchase)
	c.CreatedAt = localTime(c.CreatedAt)
	return c, nil
}
