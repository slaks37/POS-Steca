package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationFS memuat berkas SQL langsung ke dalam biner sehingga deployment
// cukup mengirim satu executable tanpa menyertakan folder migrations.
//
//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration adalah satu langkah migrasi terurut.
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// migrationsTable menyimpan versi yang sudah diterapkan.
const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER     PRIMARY KEY,
    name        TEXT        NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// advisoryLockID mengunci proses migrasi agar dua instance aplikasi yang
// start bersamaan tidak menjalankan skrip yang sama dua kali.
const advisoryLockID int64 = 8253771122

// LoadMigrations membaca seluruh migrasi terembed dan mengurutkannya menaik
// berdasarkan nomor versi pada nama berkas (mis. 0001_init.up.sql).
func LoadMigrations() ([]Migration, error) {
	return loadMigrations(migrationFS)
}

func loadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return nil, fmt.Errorf("membaca folder migrations: %w", err)
	}

	byVersion := map[int]*Migration{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, direction, err := parseMigrationName(entry.Name())
		if err != nil {
			return nil, err
		}
		body, err := fs.ReadFile(fsys, "migrations/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("membaca %s: %w", entry.Name(), err)
		}

		m, ok := byVersion[version]
		if !ok {
			m = &Migration{Version: version, Name: name}
			byVersion[version] = m
		}
		if direction == "up" {
			m.Up = string(body)
		} else {
			m.Down = string(body)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if strings.TrimSpace(m.Up) == "" {
			return nil, fmt.Errorf("migrasi versi %d tidak punya berkas .up.sql", m.Version)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })

	for i, m := range out {
		if i > 0 && m.Version == out[i-1].Version {
			return nil, fmt.Errorf("versi migrasi %d muncul lebih dari sekali", m.Version)
		}
	}
	return out, nil
}

// parseMigrationName memecah "0001_init.up.sql" menjadi versi, nama, dan arah.
func parseMigrationName(filename string) (version int, name string, direction string, err error) {
	base := strings.TrimSuffix(filename, ".sql")

	switch {
	case strings.HasSuffix(base, ".up"):
		direction = "up"
		base = strings.TrimSuffix(base, ".up")
	case strings.HasSuffix(base, ".down"):
		direction = "down"
		base = strings.TrimSuffix(base, ".down")
	default:
		return 0, "", "", fmt.Errorf("nama migrasi %q harus berakhiran .up.sql atau .down.sql", filename)
	}

	prefix, rest, found := strings.Cut(base, "_")
	if !found {
		return 0, "", "", fmt.Errorf("nama migrasi %q harus berformat <versi>_<nama>.<arah>.sql", filename)
	}
	version, convErr := strconv.Atoi(prefix)
	if convErr != nil || version <= 0 {
		return 0, "", "", fmt.Errorf("nomor versi pada %q tidak valid", filename)
	}
	return version, rest, direction, nil
}

// Migrate menerapkan seluruh migrasi yang belum pernah dijalankan. Aman
// dipanggil setiap kali aplikasi start: migrasi yang sudah tercatat dilewati.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := LoadMigrations()
	if err != nil {
		return err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("mengambil koneksi untuk migrasi: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("mengunci proses migrasi: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockID); err != nil {
			slog.Warn("gagal melepas kunci migrasi", "error", err)
		}
	}()

	if _, err := conn.Exec(ctx, migrationsTable); err != nil {
		return fmt.Errorf("menyiapkan tabel schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("membaca schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("membaca versi migrasi: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("membaca schema_migrations: %w", err)
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		// Setiap migrasi berjalan dalam satu transaksi: bila gagal di tengah,
		// tidak ada perubahan skema yang setengah jadi.
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("memulai transaksi migrasi %d: %w", m.Version, err)
		}
		if _, err := tx.Exec(ctx, m.Up); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("menjalankan migrasi %d (%s): %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.Version, m.Name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("mencatat migrasi %d: %w", m.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migrasi %d: %w", m.Version, err)
		}
		slog.Info("migrasi diterapkan", "versi", m.Version, "nama", m.Name)
	}
	return nil
}

// Rollback membatalkan satu migrasi terakhir yang tercatat. Dipakai manual
// saat pengembangan; produksi sebaiknya maju terus dengan migrasi baru.
func Rollback(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := LoadMigrations()
	if err != nil {
		return err
	}

	var version int
	err = pool.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		return fmt.Errorf("tidak ada migrasi yang bisa dibatalkan: %w", err)
	}

	for _, m := range migrations {
		if m.Version != version {
			continue
		}
		if strings.TrimSpace(m.Down) == "" {
			return fmt.Errorf("migrasi %d tidak menyediakan berkas .down.sql", version)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, m.Down); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("membatalkan migrasi %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		return tx.Commit(ctx)
	}
	return fmt.Errorf("migrasi versi %d tidak ditemukan di berkas", version)
}
