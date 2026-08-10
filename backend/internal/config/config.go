// Package config memuat konfigurasi aplikasi dari environment variable.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Datastore menentukan backend penyimpanan yang dipakai.
type Datastore string

const (
	// DatastorePostgres adalah satu-satunya mode produksi: seluruh data
	// terstruktur berada di PostgreSQL, sementara Google Drive dipakai untuk
	// gambar produk dan OAuth2 untuk login pemilik.
	DatastorePostgres Datastore = "postgres"
	// DatastoreMemory memakai penyimpanan in-memory untuk pengembangan lokal
	// dan mode demo, tanpa kredensial maupun database apa pun.
	DatastoreMemory Datastore = "memory"
)

// Config adalah seluruh konfigurasi runtime backend.
type Config struct {
	Port        string
	Env         string
	Datastore   Datastore
	FrontendURL string
	CORSOrigins []string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	JWTSecret string
	JWTTTL    time.Duration

	EncryptionKey string

	// DatabaseURL wajib diisi pada mode postgres, contoh:
	// postgres://pos:pos@localhost:5432/pos_steca?sslmode=disable
	DatabaseURL   string
	RunMigrations bool
}

// Load membaca konfigurasi dan memvalidasinya sesuai mode datastore.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               env("PORT", "8080"),
		Env:                env("APP_ENV", "development"),
		Datastore:          Datastore(env("POS_DATASTORE", string(DatastorePostgres))),
		FrontendURL:        strings.TrimSuffix(env("FRONTEND_URL", "http://localhost:5173"), "/"),
		GoogleClientID:     env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: env("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  env("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		JWTSecret:          env("JWT_SECRET", ""),
		JWTTTL:             envDuration("JWT_TTL", 12*time.Hour),
		DatabaseURL:        env("DATABASE_URL", ""),
		RunMigrations:      envBool("RUN_MIGRATIONS", true),
		EncryptionKey:      env("ENCRYPTION_KEY", ""),
	}

	origins := env("CORS_ORIGINS", cfg.FrontendURL)
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, strings.TrimSuffix(o, "/"))
		}
	}

	switch cfg.Datastore {
	case DatastorePostgres, DatastoreMemory:
	default:
		return nil, fmt.Errorf(
			"POS_DATASTORE tidak dikenal: %q (pilih postgres atau memory)", cfg.Datastore)
	}

	if cfg.JWTSecret == "" {
		if cfg.Datastore == DatastoreMemory {
			// Mode pengembangan boleh berjalan tanpa secret eksplisit.
			cfg.JWTSecret = "pos-steca-development-secret-change-me"
		} else {
			return nil, fmt.Errorf("JWT_SECRET wajib diisi")
		}
	}

	if cfg.Datastore == DatastorePostgres {
		missing := []string{}
		if cfg.DatabaseURL == "" {
			missing = append(missing, "DATABASE_URL")
		}
		if cfg.GoogleClientID == "" {
			missing = append(missing, "GOOGLE_CLIENT_ID")
		}
		if cfg.GoogleClientSecret == "" {
			missing = append(missing, "GOOGLE_CLIENT_SECRET")
		}
		if cfg.EncryptionKey == "" {
			missing = append(missing, "ENCRYPTION_KEY")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("konfigurasi wajib belum diisi: %s", strings.Join(missing, ", "))
		}
	}

	return cfg, nil
}

// IsProduction menandai mode rilis (mematikan endpoint bantu pengembangan).
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production")
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

// envBool membaca nilai boolean yang toleran terhadap penulisan umum.
func envBool(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	if d, err := time.ParseDuration(strings.TrimSpace(raw)); err == nil {
		return d
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		return time.Duration(secs) * time.Second
	}
	return fallback
}
