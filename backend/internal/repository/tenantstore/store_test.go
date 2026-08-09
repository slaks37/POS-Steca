package tenantstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/crypto"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

func newStore(t *testing.T, dir string) *FileStore {
	t.Helper()
	sealer, err := crypto.NewSealer("kunci-uji-yang-panjang")
	if err != nil {
		t.Fatalf("membuat sealer: %v", err)
	}
	store, err := NewFileStore(dir, sealer)
	if err != nil {
		t.Fatalf("membuat store: %v", err)
	}
	return store
}

func sampleTenant() *domain.Tenant {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Tenant{
		ID:            "tnt_abc",
		Code:          "STC-ABCDE",
		BusinessName:  "Warung Uji",
		OwnerEmail:    "Owner@Warung.com",
		OwnerName:     "Bu Sri",
		FolderID:      "folder-1",
		SpreadsheetID: "sheet-1",
		CreatedAt:     now,
		UpdatedAt:     now,
		RefreshToken:  "1//refresh-token-rahasia",
	}
}

func TestCreateDanBacaUlangDariDisk(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	store := newStore(t, dir)
	if err := store.Create(ctx, sampleTenant()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Store baru membaca berkas yang sama dari disk.
	reopened := newStore(t, dir)
	got, err := reopened.GetByID(ctx, "tnt_abc")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.RefreshToken != "1//refresh-token-rahasia" {
		t.Errorf("refresh token = %q, ingin terdekripsi utuh", got.RefreshToken)
	}
	if got.BusinessName != "Warung Uji" || got.SpreadsheetID != "sheet-1" {
		t.Errorf("tenant = %+v", got)
	}
}

func TestRefreshTokenTersimpanTerenkripsi(t *testing.T) {
	dir := t.TempDir()
	store := newStore(t, dir)
	if err := store.Create(context.Background(), sampleTenant()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "tenants.json"))
	if err != nil {
		t.Fatalf("membaca berkas: %v", err)
	}
	if strings.Contains(string(raw), "1//refresh-token-rahasia") {
		t.Error("refresh token tidak boleh tersimpan sebagai teks biasa")
	}
	if !strings.Contains(string(raw), "refresh_token_enc") {
		t.Error("berkas harus menyimpan kolom refresh_token_enc")
	}
}

func TestPencarianTenant(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, t.TempDir())
	if err := store.Create(ctx, sampleTenant()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.GetByCode(ctx, "stc-abcde"); err != nil {
		t.Errorf("pencarian kode harus bebas huruf besar/kecil: %v", err)
	}
	if _, err := store.GetByOwnerEmail(ctx, "OWNER@warung.com"); err != nil {
		t.Errorf("pencarian email harus bebas huruf besar/kecil: %v", err)
	}
	if _, err := store.GetByCode(ctx, "STC-ZZZZZ"); err == nil {
		t.Error("kode asing seharusnya tidak ditemukan")
	}
	if list, err := store.List(ctx); err != nil || len(list) != 1 {
		t.Errorf("List = %d tenant, %v", len(list), err)
	}
}

func TestUpdateMempertahankanTokenSaatKosong(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := newStore(t, dir)
	if err := store.Create(ctx, sampleTenant()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Perubahan nama bisnis tanpa menyertakan refresh token.
	tenant, _ := store.GetByID(ctx, "tnt_abc")
	tenant.BusinessName = "Warung Baru"
	tenant.RefreshToken = ""
	if err := store.Update(ctx, tenant); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := store.GetByID(ctx, "tnt_abc")
	if got.BusinessName != "Warung Baru" {
		t.Errorf("nama bisnis = %q", got.BusinessName)
	}
	if got.RefreshToken != "1//refresh-token-rahasia" {
		t.Errorf("refresh token = %q, ingin dipertahankan", got.RefreshToken)
	}
}

func TestCreateGandaDitolak(t *testing.T) {
	ctx := context.Background()
	store := newStore(t, t.TempDir())
	if err := store.Create(ctx, sampleTenant()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Create(ctx, sampleTenant()); err == nil {
		t.Error("tenant dengan ID sama seharusnya ditolak")
	}
	if err := store.Update(ctx, &domain.Tenant{ID: "tnt_lain"}); err == nil {
		t.Error("update tenant tidak dikenal seharusnya gagal")
	}
}

func TestDirektoriDataDibuatOtomatis(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "data")
	store := newStore(t, dir)
	if _, err := store.List(context.Background()); err != nil {
		t.Fatalf("List pada store kosong: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("direktori data seharusnya dibuat otomatis: %v", err)
	}
}
