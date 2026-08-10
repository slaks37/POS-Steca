package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/crypto"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// tenantColumns dipakai bersama oleh seluruh query baca tenant.
const tenantColumns = `id, code, business_name, owner_email, owner_name,
	folder_id, refresh_token_enc, created_at, updated_at`

// TenantStore menyimpan daftar tenant di PostgreSQL. Refresh token Google
// dienkripsi AES-256-GCM oleh internal/crypto; database hanya menerima
// ciphertext, tidak pernah token mentah.
type TenantStore struct {
	pool   *pgxpool.Pool
	sealer *crypto.Sealer
}

// NewTenantStore membuat penyimpanan tenant berbasis PostgreSQL.
func NewTenantStore(pool *pgxpool.Pool, sealer *crypto.Sealer) *TenantStore {
	return &TenantStore{pool: pool, sealer: sealer}
}

var _ domain.TenantStore = (*TenantStore)(nil)

// Create menyimpan tenant baru beserta refresh token terenkripsi.
func (s *TenantStore) Create(ctx context.Context, t *domain.Tenant) error {
	enc, err := s.sealer.Seal(t.RefreshToken)
	if err != nil {
		return apperr.Internal("gagal mengenkripsi kredensial Google").WithCause(err)
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	_, err = s.pool.Exec(ctx, `
		INSERT INTO tenants (id, code, business_name, owner_email, owner_name,
			folder_id, refresh_token_enc, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.Code, t.BusinessName, strings.ToLower(strings.TrimSpace(t.OwnerEmail)), t.OwnerName,
		t.FolderID, enc, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return wrapDB(err, "tenant")
	}
	return nil
}

// Update memperbarui tenant. Refresh token yang kosong tidak menimpa nilai
// lama, sehingga pemanggil boleh menyimpan perubahan profil tanpa memegang
// token Google.
func (s *TenantStore) Update(ctx context.Context, t *domain.Tenant) error {
	t.UpdatedAt = time.Now().UTC()

	var encPtr *string
	if t.RefreshToken != "" {
		enc, err := s.sealer.Seal(t.RefreshToken)
		if err != nil {
			return apperr.Internal("gagal mengenkripsi kredensial Google").WithCause(err)
		}
		encPtr = &enc
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE tenants SET
			code              = $2,
			business_name     = $3,
			owner_email       = $4,
			owner_name        = $5,
			folder_id         = $6,
			refresh_token_enc = COALESCE($7, refresh_token_enc),
			updated_at        = $8
		WHERE id = $1`,
		t.ID, t.Code, t.BusinessName, strings.ToLower(strings.TrimSpace(t.OwnerEmail)), t.OwnerName,
		t.FolderID, encPtr, t.UpdatedAt)
	if err != nil {
		return wrapDB(err, "tenant")
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("tenant tidak ditemukan")
	}
	return nil
}

// GetByID mencari tenant berdasarkan ID internal.
func (s *TenantStore) GetByID(ctx context.Context, id string) (*domain.Tenant, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE id = $1`, id)
	return s.scanTenant(row)
}

// GetByCode mencari tenant berdasarkan kode bisnis (dipakai saat login kasir).
func (s *TenantStore) GetByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+tenantColumns+` FROM tenants WHERE upper(code) = upper($1)`,
		strings.TrimSpace(code))
	t, err := s.scanTenant(row)
	if err != nil && isNoRows(err) {
		return nil, apperr.NotFound("kode bisnis tidak ditemukan")
	}
	return t, err
}

// GetByOwnerEmail mencari tenant berdasarkan email pemilik.
func (s *TenantStore) GetByOwnerEmail(ctx context.Context, email string) (*domain.Tenant, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+tenantColumns+` FROM tenants WHERE lower(owner_email) = lower($1)`,
		strings.TrimSpace(email))
	return s.scanTenant(row)
}

// List mengembalikan seluruh tenant terdaftar.
func (s *TenantStore) List(ctx context.Context) ([]*domain.Tenant, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tenantColumns+` FROM tenants ORDER BY created_at`)
	if err != nil {
		return nil, wrapDB(err, "tenant")
	}
	defer rows.Close()

	out := []*domain.Tenant{}
	for rows.Next() {
		t, err := s.scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "tenant")
	}
	return out, nil
}

func (s *TenantStore) scanTenant(row rowScanner) (*domain.Tenant, error) {
	var (
		t   domain.Tenant
		enc string
	)
	err := row.Scan(&t.ID, &t.Code, &t.BusinessName, &t.OwnerEmail, &t.OwnerName,
		&t.FolderID, &enc, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, apperr.NotFound("tenant tidak ditemukan")
		}
		return nil, wrapDB(err, "tenant")
	}

	token, err := s.sealer.Open(enc)
	if err != nil {
		return nil, apperr.Internal("gagal membuka kredensial Google tenant").WithCause(err)
	}
	t.RefreshToken = token
	t.CreatedAt = localTime(t.CreatedAt)
	t.UpdatedAt = localTime(t.UpdatedAt)
	return &t, nil
}

func isNoRows(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == "not_found"
}
