package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/googleapi"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// --- Repository tiruan ---

type fakeTenants struct {
	items map[string]*domain.Tenant
}

func newFakeTenants() *fakeTenants { return &fakeTenants{items: map[string]*domain.Tenant{}} }

func (f *fakeTenants) Create(_ context.Context, t *domain.Tenant) error {
	clone := *t
	f.items[t.ID] = &clone
	return nil
}

func (f *fakeTenants) Update(_ context.Context, t *domain.Tenant) error {
	if _, ok := f.items[t.ID]; !ok {
		return apperr.NotFound("tenant tidak ditemukan")
	}
	clone := *t
	f.items[t.ID] = &clone
	return nil
}

func (f *fakeTenants) GetByID(_ context.Context, id string) (*domain.Tenant, error) {
	t, ok := f.items[id]
	if !ok {
		return nil, apperr.NotFound("tenant tidak ditemukan")
	}
	clone := *t
	return &clone, nil
}

func (f *fakeTenants) GetByCode(_ context.Context, code string) (*domain.Tenant, error) {
	for _, t := range f.items {
		if t.Code == code {
			clone := *t
			return &clone, nil
		}
	}
	return nil, apperr.NotFound("kode bisnis tidak ditemukan")
}

func (f *fakeTenants) GetByOwnerEmail(_ context.Context, email string) (*domain.Tenant, error) {
	for _, t := range f.items {
		if t.OwnerEmail == email {
			clone := *t
			return &clone, nil
		}
	}
	return nil, apperr.NotFound("tenant tidak ditemukan")
}

func (f *fakeTenants) List(context.Context) ([]*domain.Tenant, error) {
	out := []*domain.Tenant{}
	for _, t := range f.items {
		out = append(out, t)
	}
	return out, nil
}

type fakeEmployees struct{ items map[string][]domain.Employee }

func newFakeEmployees() *fakeEmployees { return &fakeEmployees{items: map[string][]domain.Employee{}} }

func (f *fakeEmployees) List(_ context.Context, tenantID string) ([]domain.Employee, error) {
	return f.items[tenantID], nil
}

func (f *fakeEmployees) Get(_ context.Context, tenantID, id string) (*domain.Employee, error) {
	for i := range f.items[tenantID] {
		if f.items[tenantID][i].ID == id {
			e := f.items[tenantID][i]
			return &e, nil
		}
	}
	return nil, apperr.NotFound("karyawan tidak ditemukan")
}

func (f *fakeEmployees) GetByEmail(_ context.Context, tenantID, email string) (*domain.Employee, error) {
	for i := range f.items[tenantID] {
		if f.items[tenantID][i].Email == email {
			e := f.items[tenantID][i]
			return &e, nil
		}
	}
	return nil, apperr.NotFound("karyawan tidak ditemukan")
}

func (f *fakeEmployees) Create(_ context.Context, tenantID string, e *domain.Employee) error {
	f.items[tenantID] = append(f.items[tenantID], *e)
	return nil
}

func (f *fakeEmployees) Update(_ context.Context, tenantID string, e *domain.Employee) error {
	for i := range f.items[tenantID] {
		if f.items[tenantID][i].ID == e.ID {
			f.items[tenantID][i] = *e
			return nil
		}
	}
	return apperr.NotFound("karyawan tidak ditemukan")
}

func (f *fakeEmployees) Delete(_ context.Context, tenantID, id string) error {
	list := f.items[tenantID]
	for i := range list {
		if list[i].ID == id {
			f.items[tenantID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return apperr.NotFound("karyawan tidak ditemukan")
}

type fakeProvisioner struct{ calls int }

func (f *fakeProvisioner) Provision(_ context.Context, t *domain.Tenant) error {
	f.calls++
	t.FolderID = "folder-" + t.ID
	return nil
}

type fakeGoogle struct {
	state        string
	refreshToken string
	info         *googleapi.UserInfo
	exchangeErr  error
}

func (f *fakeGoogle) AuthCodeURL() (string, string, error) {
	f.state = "state-uji"
	return "https://accounts.google.com/o/oauth2/auth?state=state-uji", f.state, nil
}

func (f *fakeGoogle) ConsumeState(state string) bool {
	if state != f.state || f.state == "" {
		return false
	}
	f.state = "" // sekali pakai
	return true
}

func (f *fakeGoogle) ExchangeUser(context.Context, string) (string, *googleapi.UserInfo, error) {
	if f.exchangeErr != nil {
		return "", nil, f.exchangeErr
	}
	return f.refreshToken, f.info, nil
}

func newAuthFixture() (*AuthService, *fakeTenants, *fakeEmployees, *fakeProvisioner, *fakeGoogle) {
	tenants := newFakeTenants()
	employees := newFakeEmployees()
	prov := &fakeProvisioner{}
	google := &fakeGoogle{
		refreshToken: "refresh-abc",
		info:         &googleapi.UserInfo{Email: "owner@warung.com", Name: "Bu Sri"},
	}
	svc := NewAuthService(tenants, employees, prov, google, "rahasia-uji", time.Hour)
	return svc, tenants, employees, prov, google
}

// --- Pengujian ---

func TestOnboardingGoogleMembuatTenantDanOwner(t *testing.T) {
	svc, tenants, employees, prov, google := newAuthFixture()
	ctx := context.Background()

	url, err := svc.GoogleLoginURL()
	if err != nil || url == "" {
		t.Fatalf("GoogleLoginURL = %q, %v", url, err)
	}

	token, principal, err := svc.HandleGoogleCallback(ctx, "code-abc", google.state)
	if err != nil {
		t.Fatalf("callback gagal: %v", err)
	}
	if token == "" {
		t.Error("token aplikasi kosong")
	}
	if principal.Role != domain.RoleOwner {
		t.Errorf("role = %q, ingin owner", principal.Role)
	}
	if len(tenants.items) != 1 {
		t.Fatalf("jumlah tenant = %d, ingin 1", len(tenants.items))
	}
	tenant, _ := tenants.GetByOwnerEmail(ctx, "owner@warung.com")
	if tenant.FolderID == "" {
		t.Error("folder Drive harus disiapkan saat onboarding")
	}
	if tenant.Code == "" {
		t.Error("kode bisnis harus dibuat")
	}
	if prov.calls != 1 {
		t.Errorf("Provision dipanggil %d kali, ingin 1", prov.calls)
	}
	if got := employees.items[tenant.ID]; len(got) != 1 || got[0].Role != domain.RoleOwner {
		t.Errorf("karyawan owner = %+v, ingin satu owner", got)
	}
}

func TestLoginUlangTidakMembuatTenantBaru(t *testing.T) {
	svc, tenants, employees, _, google := newAuthFixture()
	ctx := context.Background()

	if _, _, err := svc.HandleGoogleCallback(ctx, "code", mustState(t, svc, google)); err != nil {
		t.Fatalf("onboarding gagal: %v", err)
	}
	if _, _, err := svc.HandleGoogleCallback(ctx, "code", mustState(t, svc, google)); err != nil {
		t.Fatalf("login ulang gagal: %v", err)
	}

	if len(tenants.items) != 1 {
		t.Errorf("jumlah tenant = %d, ingin tetap 1", len(tenants.items))
	}
	for id, list := range employees.items {
		if len(list) != 1 {
			t.Errorf("tenant %s punya %d karyawan, ingin 1", id, len(list))
		}
	}
}

func TestCallbackMenolakStateTidakValid(t *testing.T) {
	svc, _, _, _, _ := newAuthFixture()
	_, _, err := svc.HandleGoogleCallback(context.Background(), "code", "state-palsu")
	if err == nil {
		t.Fatal("state palsu seharusnya ditolak")
	}
	if code := codeOf(t, err); code != "unauthorized" {
		t.Errorf("kode error = %q, ingin unauthorized", code)
	}
}

func TestCallbackTanpaRefreshTokenDitolak(t *testing.T) {
	svc, _, _, _, google := newAuthFixture()
	google.refreshToken = ""

	_, _, err := svc.HandleGoogleCallback(context.Background(), "code", mustState(t, svc, google))
	if err == nil {
		t.Fatal("tanpa refresh token seharusnya ditolak")
	}
	if code := codeOf(t, err); code != "unauthorized" {
		t.Errorf("kode error = %q, ingin unauthorized", code)
	}
}

func TestStaffLoginDenganPIN(t *testing.T) {
	svc, tenants, employees, _, _ := newAuthFixture()
	ctx := context.Background()

	hash, err := HashPIN("123456")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	tenant := &domain.Tenant{ID: "T1", Code: "STC-ABCDE", OwnerEmail: "owner@warung.com", BusinessName: "Warung"}
	_ = tenants.Create(ctx, tenant)
	_ = employees.Create(ctx, "T1", &domain.Employee{
		ID: "EMP-1", Name: "Ani", Email: "ani@warung.com", Role: domain.RoleKasir,
		Active: true, PINHash: hash, CreatedAt: timex.Now(),
	})

	token, principal, err := svc.StaffLogin(ctx, "STC-ABCDE", "Ani@Warung.com", "123456")
	if err != nil {
		t.Fatalf("login kasir gagal: %v", err)
	}
	if principal.Role != domain.RoleKasir || principal.TenantID != "T1" {
		t.Errorf("principal = %+v", principal)
	}

	parsed, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("token hasil login tidak bisa diparse: %v", err)
	}
	if parsed.UserID != "EMP-1" || parsed.Role != domain.RoleKasir {
		t.Errorf("klaim token = %+v", parsed)
	}
}

func TestStaffLoginGagal(t *testing.T) {
	svc, tenants, employees, _, _ := newAuthFixture()
	ctx := context.Background()

	hash, _ := HashPIN("123456")
	_ = tenants.Create(ctx, &domain.Tenant{ID: "T1", Code: "STC-ABCDE", OwnerEmail: "owner@warung.com"})
	_ = employees.Create(ctx, "T1", &domain.Employee{
		ID: "EMP-1", Name: "Ani", Email: "ani@warung.com", Role: domain.RoleKasir, Active: true, PINHash: hash,
	})
	_ = employees.Create(ctx, "T1", &domain.Employee{
		ID: "EMP-2", Name: "Budi", Email: "budi@warung.com", Role: domain.RoleKasir, Active: false, PINHash: hash,
	})

	cases := []struct {
		nama             string
		code, email, pin string
		kode             string
	}{
		{"PIN salah", "STC-ABCDE", "ani@warung.com", "000000", "unauthorized"},
		{"email tidak terdaftar", "STC-ABCDE", "entah@warung.com", "123456", "unauthorized"},
		{"kode bisnis salah", "STC-ZZZZZ", "ani@warung.com", "123456", "unauthorized"},
		{"field kosong", "", "", "", "bad_request"},
		{"akun nonaktif", "STC-ABCDE", "budi@warung.com", "123456", "forbidden"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			if _, _, err := svc.StaffLogin(ctx, tc.code, tc.email, tc.pin); err == nil {
				t.Fatal("seharusnya gagal")
			} else if got := codeOf(t, err); got != tc.kode {
				t.Errorf("kode error = %q, ingin %q", got, tc.kode)
			}
		})
	}
}

func TestParseTokenMenolakTokenRusak(t *testing.T) {
	svc, _, _, _, _ := newAuthFixture()

	for _, raw := range []string{"", "bukan.jwt.sama.sekali", "eyJhbGciOiJub25lIn0.e30."} {
		if _, err := svc.ParseToken(raw); err == nil {
			t.Errorf("ParseToken(%q) seharusnya gagal", raw)
		}
	}

	// Token yang ditandatangani secret lain harus ditolak.
	lain := NewAuthService(newFakeTenants(), newFakeEmployees(), &fakeProvisioner{}, nil, "secret-lain", time.Hour)
	token, err := lain.IssueToken(&Principal{UserID: "EMP-1", TenantID: "T1", Role: domain.RoleOwner})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := svc.ParseToken(token); err == nil {
		t.Error("token dengan secret berbeda seharusnya ditolak")
	}
}

func TestTokenKedaluwarsaDitolak(t *testing.T) {
	svc := NewAuthService(newFakeTenants(), newFakeEmployees(), &fakeProvisioner{}, nil, "rahasia-uji", -time.Minute)
	token, err := svc.IssueToken(&Principal{UserID: "EMP-1", TenantID: "T1", Role: domain.RoleOwner})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := svc.ParseToken(token); err == nil {
		t.Error("token kedaluwarsa seharusnya ditolak")
	}
}

func TestHashPINMemvalidasiPanjang(t *testing.T) {
	if _, err := HashPIN("123"); err == nil {
		t.Error("PIN terlalu pendek seharusnya ditolak")
	}
	if _, err := HashPIN("1234567890123"); err == nil {
		t.Error("PIN terlalu panjang seharusnya ditolak")
	}
	if _, err := HashPIN("1234"); err != nil {
		t.Errorf("PIN 4 karakter seharusnya diterima: %v", err)
	}
}

func TestUpdateBusinessProfile(t *testing.T) {
	svc, tenants, _, _, _ := newAuthFixture()
	ctx := context.Background()
	_ = tenants.Create(ctx, &domain.Tenant{ID: "T1", Code: "STC-A", BusinessName: "Lama"})

	updated, err := svc.UpdateBusinessProfile(ctx, "T1", "  Warung Baru  ")
	if err != nil {
		t.Fatalf("update gagal: %v", err)
	}
	if updated.BusinessName != "Warung Baru" {
		t.Errorf("nama bisnis = %q, ingin \"Warung Baru\"", updated.BusinessName)
	}
	if _, err := svc.UpdateBusinessProfile(ctx, "T1", "   "); err == nil {
		t.Error("nama kosong seharusnya ditolak")
	}
}

func TestGoogleLoginTidakAktifTanpaAuthenticator(t *testing.T) {
	svc := NewAuthService(newFakeTenants(), newFakeEmployees(), &fakeProvisioner{}, nil, "rahasia", time.Hour)
	if _, err := svc.GoogleLoginURL(); err == nil {
		t.Error("login Google seharusnya tidak tersedia pada mode memory")
	}
	if _, _, err := svc.HandleGoogleCallback(context.Background(), "code", "state"); err == nil {
		t.Error("callback Google seharusnya tidak tersedia pada mode memory")
	}
}

func TestIsNotFoundHanyaUntukErrorNotFound(t *testing.T) {
	if !isNotFound(apperr.NotFound("hilang")) {
		t.Error("apperr.NotFound harus dikenali")
	}
	if isNotFound(apperr.Conflict("bentrok")) || isNotFound(errors.New("lainnya")) {
		t.Error("error selain not_found tidak boleh dikenali sebagai not found")
	}
}

func mustState(t *testing.T, svc *AuthService, google *fakeGoogle) string {
	t.Helper()
	if _, err := svc.GoogleLoginURL(); err != nil {
		t.Fatalf("GoogleLoginURL: %v", err)
	}
	return google.state
}
