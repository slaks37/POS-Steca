package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/googleapi"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// Principal adalah identitas pengguna yang sedang masuk.
type Principal struct {
	UserID   string      `json:"user_id"`
	TenantID string      `json:"tenant_id"`
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Role     domain.Role `json:"role"`
}

// Claims adalah isi JWT aplikasi.
type Claims struct {
	TenantID string      `json:"tid"`
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Role     domain.Role `json:"role"`
	jwt.RegisteredClaims
}

// GoogleAuthenticator adalah bagian OAuth Google yang dipakai AuthService.
// Dibuat sebagai antarmuka agar service tetap bisa diuji tanpa jaringan.
type GoogleAuthenticator interface {
	AuthCodeURL() (string, string, error)
	ConsumeState(state string) bool
	ExchangeUser(ctx context.Context, code string) (refreshToken string, info *googleapi.UserInfo, err error)
}

// AuthService menangani login pemilik (Google OAuth) dan login karyawan (PIN),
// serta onboarding tenant baru.
type AuthService struct {
	tenants   domain.TenantStore
	employees domain.EmployeeRepository
	prov      domain.Provisioner
	google    GoogleAuthenticator
	secret    []byte
	ttl       time.Duration
}

// NewAuthService membuat service autentikasi.
func NewAuthService(
	tenants domain.TenantStore,
	employees domain.EmployeeRepository,
	prov domain.Provisioner,
	google GoogleAuthenticator,
	secret string,
	ttl time.Duration,
) *AuthService {
	return &AuthService{
		tenants:   tenants,
		employees: employees,
		prov:      prov,
		google:    google,
		secret:    []byte(secret),
		ttl:       ttl,
	}
}

// GoogleLoginURL menghasilkan URL consent Google untuk onboarding pemilik.
func (s *AuthService) GoogleLoginURL() (string, error) {
	if s.google == nil {
		return "", apperr.BadRequest("login Google tidak aktif pada mode datastore ini")
	}
	url, _, err := s.google.AuthCodeURL()
	if err != nil {
		return "", apperr.Internal("gagal membuat URL login Google").WithCause(err)
	}
	return url, nil
}

// HandleGoogleCallback menyelesaikan alur OAuth: menukar code, membuat atau
// memperbarui tenant, menyiapkan folder Drive untuk gambar produk, lalu
// menerbitkan token aplikasi.
func (s *AuthService) HandleGoogleCallback(ctx context.Context, code, state string) (string, *Principal, error) {
	if s.google == nil {
		return "", nil, apperr.BadRequest("login Google tidak aktif pada mode datastore ini")
	}
	if !s.google.ConsumeState(state) {
		return "", nil, apperr.Unauthorized("state OAuth tidak valid atau kedaluwarsa")
	}
	refreshToken, info, err := s.google.ExchangeUser(ctx, code)
	if err != nil {
		return "", nil, apperr.Unauthorized("gagal memverifikasi akun Google").WithCause(err)
	}
	if info.Email == "" {
		return "", nil, apperr.Unauthorized("akun Google tidak memiliki email")
	}

	tenant, err := s.tenants.GetByOwnerEmail(ctx, info.Email)
	switch {
	case err == nil:
		if refreshToken != "" {
			tenant.RefreshToken = refreshToken
		}
		if tenant.RefreshToken == "" {
			return "", nil, apperr.Unauthorized("Google tidak mengirim refresh token, cabut akses aplikasi di akun Google lalu coba lagi")
		}
		if err := s.prov.Provision(ctx, tenant); err != nil {
			return "", nil, err
		}
		if err := s.tenants.Update(ctx, tenant); err != nil {
			return "", nil, err
		}
	case isNotFound(err):
		if refreshToken == "" {
			return "", nil, apperr.Unauthorized("Google tidak mengirim refresh token, cabut akses aplikasi di akun Google lalu coba lagi")
		}
		tenant, err = s.onboard(ctx, info, refreshToken)
		if err != nil {
			return "", nil, err
		}
	default:
		return "", nil, err
	}

	owner, err := s.ensureOwnerEmployee(ctx, tenant, info)
	if err != nil {
		return "", nil, err
	}

	principal := &Principal{
		UserID:   owner.ID,
		TenantID: tenant.ID,
		Name:     owner.Name,
		Email:    owner.Email,
		Role:     domain.RoleOwner,
	}
	token, err := s.IssueToken(principal)
	if err != nil {
		return "", nil, err
	}
	return token, principal, nil
}

func (s *AuthService) onboard(ctx context.Context, info *googleapi.UserInfo, refreshToken string) (*domain.Tenant, error) {
	name := info.Name
	if name == "" {
		name = strings.Split(info.Email, "@")[0]
	}
	now := time.Now().UTC()
	tenant := &domain.Tenant{
		ID:           NewTenantID(),
		Code:         NewTenantCode(),
		BusinessName: name,
		OwnerEmail:   strings.ToLower(info.Email),
		OwnerName:    name,
		CreatedAt:    now,
		UpdatedAt:    now,
		RefreshToken: refreshToken,
	}
	if err := s.prov.Provision(ctx, tenant); err != nil {
		return nil, err
	}
	if err := s.tenants.Create(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

func (s *AuthService) ensureOwnerEmployee(ctx context.Context, tenant *domain.Tenant, info *googleapi.UserInfo) (*domain.Employee, error) {
	existing, err := s.employees.GetByEmail(ctx, tenant.ID, tenant.OwnerEmail)
	if err == nil {
		if !existing.Active || existing.Role != domain.RoleOwner {
			existing.Active = true
			existing.Role = domain.RoleOwner
			if err := s.employees.Update(ctx, tenant.ID, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	if !isNotFound(err) {
		return nil, err
	}

	name := info.Name
	if name == "" {
		name = tenant.OwnerName
	}
	owner := &domain.Employee{
		ID:        NewEmployeeID(),
		Name:      name,
		Email:     tenant.OwnerEmail,
		Role:      domain.RoleOwner,
		Active:    true,
		CreatedAt: timex.Now(),
	}
	if err := s.employees.Create(ctx, tenant.ID, owner); err != nil {
		return nil, err
	}
	return owner, nil
}

// StaffLogin memvalidasi login karyawan memakai kode bisnis, email, dan PIN.
func (s *AuthService) StaffLogin(ctx context.Context, tenantCode, email, pin string) (string, *Principal, error) {
	tenantCode = strings.TrimSpace(tenantCode)
	email = strings.ToLower(strings.TrimSpace(email))
	if tenantCode == "" || email == "" || pin == "" {
		return "", nil, apperr.BadRequest("kode bisnis, email, dan PIN wajib diisi")
	}

	tenant, err := s.tenants.GetByCode(ctx, tenantCode)
	if err != nil {
		if isNotFound(err) {
			return "", nil, apperr.Unauthorized("kode bisnis, email, atau PIN salah")
		}
		return "", nil, err
	}

	employee, err := s.employees.GetByEmail(ctx, tenant.ID, email)
	if err != nil {
		if isNotFound(err) {
			return "", nil, apperr.Unauthorized("kode bisnis, email, atau PIN salah")
		}
		return "", nil, err
	}
	if !employee.Active {
		return "", nil, apperr.Forbidden("akun karyawan sedang nonaktif")
	}
	if employee.PINHash == "" {
		return "", nil, apperr.Unauthorized("PIN belum diatur, minta pemilik untuk mengatur ulang")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(employee.PINHash), []byte(pin)); err != nil {
		return "", nil, apperr.Unauthorized("kode bisnis, email, atau PIN salah")
	}

	principal := &Principal{
		UserID:   employee.ID,
		TenantID: tenant.ID,
		Name:     employee.Name,
		Email:    employee.Email,
		Role:     employee.Role,
	}
	token, err := s.IssueToken(principal)
	if err != nil {
		return "", nil, err
	}
	return token, principal, nil
}

// IssueToken menerbitkan JWT aplikasi untuk principal tertentu.
func (s *AuthService) IssueToken(p *Principal) (string, error) {
	now := time.Now()
	claims := Claims{
		TenantID: p.TenantID,
		Name:     p.Name,
		Email:    p.Email,
		Role:     p.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
			Issuer:    "pos-steca",
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", apperr.Internal("gagal menerbitkan token").WithCause(err)
	}
	return signed, nil
}

// ParseToken memvalidasi JWT dan mengembalikan principal-nya.
func (s *AuthService) ParseToken(raw string) (*Principal, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode tanda tangan tidak didukung")
		}
		return s.secret, nil
	}, jwt.WithIssuer("pos-steca"), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, apperr.Unauthorized("sesi tidak valid atau sudah berakhir")
	}
	if claims.TenantID == "" || claims.Subject == "" {
		return nil, apperr.Unauthorized("sesi tidak valid")
	}
	return &Principal{
		UserID:   claims.Subject,
		TenantID: claims.TenantID,
		Name:     claims.Name,
		Email:    claims.Email,
		Role:     claims.Role,
	}, nil
}

// Tenant mengembalikan data tenant untuk ditampilkan di frontend.
func (s *AuthService) Tenant(ctx context.Context, tenantID string) (*domain.Tenant, error) {
	return s.tenants.GetByID(ctx, tenantID)
}

// UpdateBusinessProfile mengganti nama bisnis tenant.
func (s *AuthService) UpdateBusinessProfile(ctx context.Context, tenantID, businessName string) (*domain.Tenant, error) {
	businessName = strings.TrimSpace(businessName)
	if businessName == "" {
		return nil, apperr.BadRequest("nama bisnis wajib diisi")
	}
	tenant, err := s.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	tenant.BusinessName = businessName
	if err := s.tenants.Update(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// HashPIN membuat hash bcrypt dari PIN karyawan.
func HashPIN(pin string) (string, error) {
	if len(pin) < 4 || len(pin) > 12 {
		return "", apperr.BadRequest("PIN harus 4-12 karakter")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", apperr.Internal("gagal mengenkripsi PIN").WithCause(err)
	}
	return string(hash), nil
}

func isNotFound(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == "not_found"
}
