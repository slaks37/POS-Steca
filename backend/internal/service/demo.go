package service

import (
	"context"
	"strings"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// DemoService menyiapkan tenant contoh untuk mode pengembangan
// (POS_DATASTORE=memory), sehingga aplikasi bisa dicoba tanpa kredensial
// Google. Service ini tidak didaftarkan pada mode produksi.
type DemoService struct {
	tenants   domain.TenantStore
	employees domain.EmployeeRepository
	products  domain.ProductRepository
	prov      domain.Provisioner
	auth      *AuthService
}

// NewDemoService membuat service demo.
func NewDemoService(
	tenants domain.TenantStore,
	employees domain.EmployeeRepository,
	products domain.ProductRepository,
	prov domain.Provisioner,
	auth *AuthService,
) *DemoService {
	return &DemoService{tenants: tenants, employees: employees, products: products, prov: prov, auth: auth}
}

var demoProducts = []struct {
	Name     string
	Category string
	Price    float64
	Stock    int
	SKU      string
}{
	{"Nasi Goreng Spesial", "Makanan", 25000, 40, "MKN-001"},
	{"Ayam Geprek Sambal Bawang", "Makanan", 22000, 35, "MKN-002"},
	{"Mie Ayam Bakso", "Makanan", 20000, 30, "MKN-003"},
	{"Kwetiau Siram Seafood", "Makanan", 28000, 18, "MKN-004"},
	{"Es Teh Manis", "Minuman", 6000, 120, "MNM-001"},
	{"Es Jeruk Peras", "Minuman", 9000, 80, "MNM-002"},
	{"Kopi Susu Gula Aren", "Minuman", 18000, 60, "MNM-003"},
	{"Air Mineral 600ml", "Minuman", 5000, 4, "MNM-004"},
	{"Pisang Goreng Keju", "Camilan", 15000, 25, "CML-001"},
	{"Kentang Goreng", "Camilan", 17000, 3, "CML-002"},
}

// Bootstrap membuat (atau memakai ulang) tenant demo dan menerbitkan token
// pemilik. PIN kasir demo dikembalikan agar bisa dicoba dari layar login.
func (s *DemoService) Bootstrap(ctx context.Context, businessName string) (string, *Principal, *domain.Tenant, error) {
	businessName = strings.TrimSpace(businessName)
	if businessName == "" {
		businessName = "Warung Demo Steca"
	}
	email := "owner@demo.local"

	tenant, err := s.tenants.GetByOwnerEmail(ctx, email)
	if err != nil {
		if !isNotFound(err) {
			return "", nil, nil, err
		}
		now := time.Now().UTC()
		tenant = &domain.Tenant{
			ID:           NewTenantID(),
			Code:         NewTenantCode(),
			BusinessName: businessName,
			OwnerEmail:   email,
			OwnerName:    "Pemilik Demo",
			CreatedAt:    now,
			UpdatedAt:    now,
			RefreshToken: "demo",
		}
		if err := s.prov.Provision(ctx, tenant); err != nil {
			return "", nil, nil, err
		}
		if err := s.tenants.Create(ctx, tenant); err != nil {
			return "", nil, nil, err
		}
		if err := s.seed(ctx, tenant); err != nil {
			return "", nil, nil, err
		}
	}

	owner, err := s.employees.GetByEmail(ctx, tenant.ID, email)
	if err != nil {
		return "", nil, nil, err
	}
	principal := &Principal{
		UserID:   owner.ID,
		TenantID: tenant.ID,
		Name:     owner.Name,
		Email:    owner.Email,
		Role:     domain.RoleOwner,
	}
	token, err := s.auth.IssueToken(principal)
	if err != nil {
		return "", nil, nil, err
	}
	return token, principal, tenant, nil
}

func (s *DemoService) seed(ctx context.Context, tenant *domain.Tenant) error {
	ownerPIN, err := HashPIN("112233")
	if err != nil {
		return err
	}
	kasirPIN, err := HashPIN("123456")
	if err != nil {
		return err
	}
	staff := []domain.Employee{
		{ID: NewEmployeeID(), Name: "Pemilik Demo", Email: tenant.OwnerEmail, Role: domain.RoleOwner, Active: true, PINHash: ownerPIN, CreatedAt: timex.Now()},
		{ID: NewEmployeeID(), Name: "Kasir Demo", Email: "kasir@demo.local", Role: domain.RoleKasir, Active: true, PINHash: kasirPIN, CreatedAt: timex.Now()},
	}
	for i := range staff {
		if err := s.employees.Create(ctx, tenant.ID, &staff[i]); err != nil {
			return err
		}
	}
	for _, p := range demoProducts {
		item := &domain.Product{
			ID:       NewProductID(),
			Name:     p.Name,
			Category: p.Category,
			Price:    p.Price,
			Stock:    p.Stock,
			SKU:      p.SKU,
		}
		if err := s.products.Create(ctx, tenant.ID, item); err != nil {
			return err
		}
	}
	return nil
}
