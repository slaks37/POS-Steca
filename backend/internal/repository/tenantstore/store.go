// Package tenantstore menyimpan daftar tenant beserta refresh token Google
// mereka. Ini satu-satunya data yang berada di luar Google Drive: backend
// perlu tahu spreadsheet dan folder milik siapa sebelum bisa menghubungi
// Google API atas nama tenant tersebut.
package tenantstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/crypto"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

type record struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	BusinessName    string    `json:"business_name"`
	OwnerEmail      string    `json:"owner_email"`
	OwnerName       string    `json:"owner_name"`
	FolderID        string    `json:"folder_id"`
	SpreadsheetID   string    `json:"spreadsheet_id"`
	RefreshTokenEnc string    `json:"refresh_token_enc"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FileStore adalah TenantStore berbasis satu berkas JSON. Refresh token
// selalu tersimpan dalam bentuk terenkripsi AES-256-GCM.
type FileStore struct {
	path   string
	sealer *crypto.Sealer

	mu      sync.RWMutex
	records map[string]record
}

var _ domain.TenantStore = (*FileStore)(nil)

// NewFileStore memuat (atau membuat) berkas penyimpanan tenant.
func NewFileStore(dir string, sealer *crypto.Sealer) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("menyiapkan direktori data: %w", err)
	}
	s := &FileStore{
		path:    filepath.Join(dir, "tenants.json"),
		sealer:  sealer,
		records: map[string]record{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("membaca berkas tenant: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}
	var items []record
	if err := json.Unmarshal(raw, &items); err != nil {
		return fmt.Errorf("membaca berkas tenant: %w", err)
	}
	for _, r := range items {
		s.records[r.ID] = r
	}
	return nil
}

// flush menulis ulang berkas secara atomik. Pemanggil harus memegang s.mu.
func (s *FileStore) flush() error {
	items := make([]record, 0, len(s.records))
	for _, r := range s.records {
		items = append(items, r)
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("menulis berkas tenant: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("menulis berkas tenant: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("menyimpan berkas tenant: %w", err)
	}
	return nil
}

func (s *FileStore) toDomain(r record) (*domain.Tenant, error) {
	token, err := s.sealer.Open(r.RefreshTokenEnc)
	if err != nil {
		return nil, apperr.Internal("gagal membuka kredensial Google tenant").WithCause(err)
	}
	return &domain.Tenant{
		ID:            r.ID,
		Code:          r.Code,
		BusinessName:  r.BusinessName,
		OwnerEmail:    r.OwnerEmail,
		OwnerName:     r.OwnerName,
		FolderID:      r.FolderID,
		SpreadsheetID: r.SpreadsheetID,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		RefreshToken:  token,
	}, nil
}

func (s *FileStore) toRecord(t *domain.Tenant, previous string) (record, error) {
	enc := previous
	if t.RefreshToken != "" {
		var err error
		enc, err = s.sealer.Seal(t.RefreshToken)
		if err != nil {
			return record{}, apperr.Internal("gagal mengenkripsi kredensial Google").WithCause(err)
		}
	}
	return record{
		ID:              t.ID,
		Code:            t.Code,
		BusinessName:    t.BusinessName,
		OwnerEmail:      strings.ToLower(t.OwnerEmail),
		OwnerName:       t.OwnerName,
		FolderID:        t.FolderID,
		SpreadsheetID:   t.SpreadsheetID,
		RefreshTokenEnc: enc,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}, nil
}

// Create menyimpan tenant baru.
func (s *FileStore) Create(_ context.Context, t *domain.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[t.ID]; exists {
		return apperr.Conflict("tenant sudah terdaftar")
	}
	r, err := s.toRecord(t, "")
	if err != nil {
		return err
	}
	s.records[t.ID] = r
	return s.flush()
}

// Update memperbarui tenant yang sudah ada.
func (s *FileStore) Update(_ context.Context, t *domain.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.records[t.ID]
	if !ok {
		return apperr.NotFound("tenant tidak ditemukan")
	}
	t.UpdatedAt = time.Now().UTC()
	r, err := s.toRecord(t, prev.RefreshTokenEnc)
	if err != nil {
		return err
	}
	r.CreatedAt = prev.CreatedAt
	s.records[t.ID] = r
	return s.flush()
}

// GetByID mencari tenant berdasarkan ID internal.
func (s *FileStore) GetByID(_ context.Context, id string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[id]
	if !ok {
		return nil, apperr.NotFound("tenant tidak ditemukan")
	}
	return s.toDomain(r)
}

// GetByCode mencari tenant berdasarkan kode bisnis (dipakai saat login kasir).
func (s *FileStore) GetByCode(_ context.Context, code string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, r := range s.records {
		if strings.ToUpper(r.Code) == code {
			return s.toDomain(r)
		}
	}
	return nil, apperr.NotFound("kode bisnis tidak ditemukan")
}

// GetByOwnerEmail mencari tenant berdasarkan email pemilik.
func (s *FileStore) GetByOwnerEmail(_ context.Context, email string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	email = strings.ToLower(strings.TrimSpace(email))
	for _, r := range s.records {
		if r.OwnerEmail == email {
			return s.toDomain(r)
		}
	}
	return nil, apperr.NotFound("tenant tidak ditemukan")
}

// List mengembalikan seluruh tenant terdaftar.
func (s *FileStore) List(_ context.Context) ([]*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Tenant, 0, len(s.records))
	for _, r := range s.records {
		t, err := s.toDomain(r)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
