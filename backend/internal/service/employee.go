package service

import (
	"context"
	"sort"
	"strings"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// EmployeeInput adalah payload pembuatan/perubahan karyawan.
type EmployeeInput struct {
	Name   string      `json:"name"`
	Email  string      `json:"email"`
	Role   domain.Role `json:"role"`
	PIN    string      `json:"pin"`
	Active *bool       `json:"active"`
}

// EmployeeService mengelola karyawan dan hak aksesnya.
type EmployeeService struct {
	employees domain.EmployeeRepository
	tenants   domain.TenantStore
}

// NewEmployeeService membuat service karyawan.
func NewEmployeeService(employees domain.EmployeeRepository, tenants domain.TenantStore) *EmployeeService {
	return &EmployeeService{employees: employees, tenants: tenants}
}

// List mengembalikan daftar karyawan tenant.
func (s *EmployeeService) List(ctx context.Context, tenantID string) ([]domain.Employee, error) {
	items, err := s.employees.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Role != items[j].Role {
			return items[i].Role == domain.RoleOwner
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// Create menambahkan karyawan baru beserta PIN login-nya.
func (s *EmployeeService) Create(ctx context.Context, tenantID string, in EmployeeInput) (*domain.Employee, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if name == "" {
		return nil, apperr.BadRequest("nama karyawan wajib diisi")
	}
	if !strings.Contains(email, "@") {
		return nil, apperr.BadRequest("email karyawan tidak valid")
	}
	role := domain.Role(strings.ToLower(string(in.Role)))
	if role == "" {
		role = domain.RoleKasir
	}
	if !role.Valid() {
		return nil, apperr.BadRequest("role harus owner atau kasir")
	}
	if in.PIN == "" {
		return nil, apperr.BadRequest("PIN wajib diisi")
	}
	hash, err := HashPIN(in.PIN)
	if err != nil {
		return nil, err
	}

	if _, err := s.employees.GetByEmail(ctx, tenantID, email); err == nil {
		return nil, apperr.Conflict("email tersebut sudah terdaftar sebagai karyawan")
	} else if !isNotFound(err) {
		return nil, err
	}

	active := true
	if in.Active != nil {
		active = *in.Active
	}
	e := &domain.Employee{
		ID:        NewEmployeeID(),
		Name:      name,
		Email:     email,
		Role:      role,
		Active:    active,
		PINHash:   hash,
		CreatedAt: timex.Now(),
	}
	if err := s.employees.Create(ctx, tenantID, e); err != nil {
		return nil, err
	}
	return e, nil
}

// Update mengubah data karyawan. PIN hanya diganti bila diisi.
func (s *EmployeeService) Update(ctx context.Context, tenantID, employeeID string, in EmployeeInput) (*domain.Employee, error) {
	existing, err := s.employees.Get(ctx, tenantID, employeeID)
	if err != nil {
		return nil, err
	}
	tenant, err := s.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if name := strings.TrimSpace(in.Name); name != "" {
		existing.Name = name
	}
	if role := domain.Role(strings.ToLower(string(in.Role))); role != "" {
		if !role.Valid() {
			return nil, apperr.BadRequest("role harus owner atau kasir")
		}
		if strings.EqualFold(existing.Email, tenant.OwnerEmail) && role != domain.RoleOwner {
			return nil, apperr.Conflict("role pemilik akun Google tidak bisa diturunkan")
		}
		existing.Role = role
	}
	if in.Active != nil {
		if !*in.Active && strings.EqualFold(existing.Email, tenant.OwnerEmail) {
			return nil, apperr.Conflict("akun pemilik tidak bisa dinonaktifkan")
		}
		existing.Active = *in.Active
	}
	if in.PIN != "" {
		hash, err := HashPIN(in.PIN)
		if err != nil {
			return nil, err
		}
		existing.PINHash = hash
	}

	if err := s.employees.Update(ctx, tenantID, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete menghapus karyawan. Pemilik akun Google tidak boleh dihapus.
func (s *EmployeeService) Delete(ctx context.Context, tenantID, employeeID string) error {
	existing, err := s.employees.Get(ctx, tenantID, employeeID)
	if err != nil {
		return err
	}
	tenant, err := s.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return err
	}
	if strings.EqualFold(existing.Email, tenant.OwnerEmail) {
		return apperr.Conflict("akun pemilik tidak bisa dihapus")
	}
	return s.employees.Delete(ctx, tenantID, employeeID)
}
