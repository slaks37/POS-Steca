package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

const employeeColumns = `id, name, email, role, active, pin_hash, created_at`

// EmployeeRepository memetakan tabel employees.
type EmployeeRepository struct {
	pool *pgxpool.Pool
}

// NewEmployeeRepository membuat repository karyawan berbasis PostgreSQL.
func NewEmployeeRepository(pool *pgxpool.Pool) *EmployeeRepository {
	return &EmployeeRepository{pool: pool}
}

var _ domain.EmployeeRepository = (*EmployeeRepository)(nil)

// List mengembalikan karyawan satu tenant: owner lebih dulu, lalu urut nama.
func (r *EmployeeRepository) List(ctx context.Context, tenantID string) ([]domain.Employee, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+employeeColumns+` FROM employees WHERE tenant_id = $1
		 ORDER BY (role = 'owner') DESC, lower(name)`, tenantID)
	if err != nil {
		return nil, wrapDB(err, "karyawan")
	}
	defer rows.Close()

	out := []domain.Employee{}
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "karyawan")
	}
	return out, nil
}

// Get mengambil karyawan berdasarkan ID.
func (r *EmployeeRepository) Get(ctx context.Context, tenantID, employeeID string) (*domain.Employee, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+employeeColumns+` FROM employees WHERE tenant_id = $1 AND id = $2`,
		tenantID, employeeID)
	e, err := scanEmployee(row)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetByEmail mengambil karyawan berdasarkan email (dipakai saat login PIN).
func (r *EmployeeRepository) GetByEmail(ctx context.Context, tenantID, email string) (*domain.Employee, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+employeeColumns+` FROM employees WHERE tenant_id = $1 AND lower(email) = lower($2)`,
		tenantID, strings.TrimSpace(email))
	e, err := scanEmployee(row)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Create menambahkan karyawan baru.
func (r *EmployeeRepository) Create(ctx context.Context, tenantID string, e *domain.Employee) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO employees (tenant_id, id, name, email, role, active, pin_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, e.ID, e.Name, strings.ToLower(strings.TrimSpace(e.Email)),
		string(e.Role), e.Active, e.PINHash, e.CreatedAt.UTC())
	if err != nil {
		return wrapDB(err, "karyawan")
	}
	return nil
}

// Update mengubah karyawan milik tenant.
func (r *EmployeeRepository) Update(ctx context.Context, tenantID string, e *domain.Employee) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE employees SET name = $3, email = $4, role = $5, active = $6, pin_hash = $7
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, e.ID, e.Name, strings.ToLower(strings.TrimSpace(e.Email)),
		string(e.Role), e.Active, e.PINHash)
	if err != nil {
		return wrapDB(err, "karyawan")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("karyawan tidak ditemukan")
	}
	return nil
}

// Delete menghapus karyawan milik tenant.
func (r *EmployeeRepository) Delete(ctx context.Context, tenantID, employeeID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM employees WHERE tenant_id = $1 AND id = $2`, tenantID, employeeID)
	if err != nil {
		return wrapDB(err, "karyawan")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("karyawan tidak ditemukan")
	}
	return nil
}

func scanEmployee(row rowScanner) (domain.Employee, error) {
	var e domain.Employee
	err := row.Scan(&e.ID, &e.Name, &e.Email, &e.Role, &e.Active, &e.PINHash, &e.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Employee{}, apperr.NotFound("karyawan tidak ditemukan")
		}
		return domain.Employee{}, wrapDB(err, "karyawan")
	}
	e.Email = strings.ToLower(e.Email)
	e.CreatedAt = localTime(e.CreatedAt)
	return e, nil
}
