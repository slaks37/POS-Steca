// Package postgres mengimplementasikan port repository domain di atas
// PostgreSQL — satu-satunya sumber kebenaran data terstruktur aplikasi.
//
// Aturan multi-tenant: SETIAP query menyertakan predikat tenant_id sehingga
// data satu bisnis tidak mungkin terbaca oleh bisnis lain. Tabel pun memakai
// kunci utama gabungan (tenant_id, id).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// Kode error PostgreSQL yang perlu dipetakan ke error aplikasi.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

// Connect membuka pool koneksi dan memastikan database benar-benar bisa
// dihubungi sebelum aplikasi menyatakan dirinya siap.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL tidak valid: %w", err)
	}
	// Batas konservatif agar cocok dengan kuota koneksi provider managed
	// PostgreSQL yang umumnya kecil pada paket awal.
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 10
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("membuka koneksi database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database tidak merespons: %w", err)
	}
	return pool, nil
}

// wrapDB memetakan error driver menjadi error aplikasi berbahasa Indonesia.
// entity dipakai untuk menyusun pesan yang mudah dimengerti kasir.
func wrapDB(err error, entity string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return apperr.Conflict("%s dengan data serupa sudah ada", entity).WithCause(err)
		case pgForeignKeyViolation:
			return apperr.BadRequest("%s merujuk data yang tidak ada", entity).WithCause(err)
		case pgCheckViolation:
			return apperr.BadRequest("nilai %s tidak memenuhi aturan penyimpanan", entity).WithCause(err)
		}
	}
	return apperr.Internal("gagal mengakses data %s", entity).WithCause(err)
}

// rowScanner menyamakan pgx.Row dan pgx.Rows sehingga fungsi pemetaan baris
// bisa dipakai ulang — dan bisa diuji dengan scanner tiruan.
type rowScanner interface {
	Scan(dest ...any) error
}

// localTime menyeragamkan waktu hasil query ke zona waktu aplikasi. Nilai di
// database selalu TIMESTAMPTZ (UTC), tampilannya yang mengikuti WIB.
func localTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.In(timex.Location())
}

// nullableTime membaca kolom TIMESTAMPTZ yang boleh NULL.
func nullableTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return localTime(*t)
}

// timePtr mengubah waktu nol menjadi NULL saat menulis ke database.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}
