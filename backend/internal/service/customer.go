package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// RupiahPerPoint adalah nilai belanja yang menghasilkan satu poin loyalitas.
// Aturannya sengaja sederhana agar mudah dijelaskan ke pelanggan UMKM:
// setiap Rp 10.000 belanja = 1 poin.
const RupiahPerPoint = 10000

// PointsFor menghitung poin yang didapat dari satu nilai belanja.
func PointsFor(amount float64) int {
	if amount <= 0 {
		return 0
	}
	return int(amount / RupiahPerPoint)
}

// CustomerInput adalah payload pembuatan/perubahan pelanggan.
type CustomerInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// CustomerHistory adalah riwayat pembelian satu pelanggan.
type CustomerHistory struct {
	Customer domain.Customer `json:"customer"`
	Orders   []domain.Order  `json:"orders"`
}

// CustomerService mengelola data pelanggan dan poin loyalitasnya.
type CustomerService struct {
	customers domain.CustomerRepository
	orders    domain.OrderRepository
}

// NewCustomerService membuat service pelanggan.
func NewCustomerService(customers domain.CustomerRepository, orders domain.OrderRepository) *CustomerService {
	return &CustomerService{customers: customers, orders: orders}
}

// List mengembalikan pelanggan, opsional disaring nama atau nomor HP, dan
// diurutkan dari yang paling besar total belanjanya.
func (s *CustomerService) List(ctx context.Context, tenantID, query string) ([]domain.Customer, error) {
	items, err := s.customers.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query != "" {
		lower := strings.ToLower(query)
		digits := domain.NormalizePhone(query)
		filtered := make([]domain.Customer, 0, len(items))
		for _, c := range items {
			matchName := strings.Contains(strings.ToLower(c.Name), lower)
			matchPhone := digits != "" && strings.Contains(domain.NormalizePhone(c.Phone), digits)
			if matchName || matchPhone {
				filtered = append(filtered, c)
			}
		}
		items = filtered
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TotalSpent != items[j].TotalSpent {
			return items[i].TotalSpent > items[j].TotalSpent
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// Get mengambil satu pelanggan.
func (s *CustomerService) Get(ctx context.Context, tenantID, customerID string) (*domain.Customer, error) {
	return s.customers.Get(ctx, tenantID, customerID)
}

// Create mendaftarkan pelanggan baru. Nomor HP dipakai sebagai identitas unik
// bila diisi, sehingga kasir tidak membuat data ganda untuk orang yang sama.
func (s *CustomerService) Create(ctx context.Context, tenantID string, in CustomerInput) (*domain.Customer, error) {
	name := strings.TrimSpace(in.Name)
	phone := strings.TrimSpace(in.Phone)
	if name == "" {
		return nil, apperr.BadRequest("nama pelanggan wajib diisi")
	}
	if phone != "" {
		if existing, err := s.customers.GetByPhone(ctx, tenantID, phone); err == nil {
			return nil, apperr.Conflict("nomor HP tersebut sudah terdaftar atas nama %s", existing.Name)
		} else if !isNotFound(err) {
			return nil, err
		}
	}

	c := &domain.Customer{
		ID:        NewCustomerID(),
		Name:      name,
		Phone:     phone,
		CreatedAt: timex.Now(),
	}
	if err := s.customers.Create(ctx, tenantID, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Update mengubah nama atau nomor HP pelanggan.
func (s *CustomerService) Update(ctx context.Context, tenantID, customerID string, in CustomerInput) (*domain.Customer, error) {
	existing, err := s.customers.Get(ctx, tenantID, customerID)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		existing.Name = name
	}
	phone := strings.TrimSpace(in.Phone)
	if phone != "" && domain.NormalizePhone(phone) != domain.NormalizePhone(existing.Phone) {
		if other, err := s.customers.GetByPhone(ctx, tenantID, phone); err == nil && other.ID != customerID {
			return nil, apperr.Conflict("nomor HP tersebut sudah terdaftar atas nama %s", other.Name)
		} else if err != nil && !isNotFound(err) {
			return nil, err
		}
	}
	existing.Phone = phone

	if err := s.customers.Update(ctx, tenantID, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete menghapus pelanggan. Pesanan lama tetap tersimpan; kolom pelanggan
// pada pesanan tersebut hanya kehilangan tautannya.
func (s *CustomerService) Delete(ctx context.Context, tenantID, customerID string) error {
	return s.customers.Delete(ctx, tenantID, customerID)
}

// History mengembalikan pelanggan beserta riwayat pesanannya.
func (s *CustomerService) History(ctx context.Context, tenantID, customerID string) (*CustomerHistory, error) {
	customer, err := s.customers.Get(ctx, tenantID, customerID)
	if err != nil {
		return nil, err
	}
	all, err := s.orders.List(ctx, tenantID, domain.OrderFilter{})
	if err != nil {
		return nil, err
	}
	history := make([]domain.Order, 0, 16)
	for _, o := range all {
		if o.CustomerID == customerID {
			history = append(history, o)
		}
	}
	return &CustomerHistory{Customer: *customer, Orders: history}, nil
}

// resolveCheckoutCustomer menentukan pelanggan untuk sebuah transaksi kasir:
// memakai ID yang dipilih, mencocokkan nomor HP yang sudah terdaftar, atau
// mendaftarkan pelanggan baru. Mengembalikan nil untuk transaksi anonim.
func (s *CustomerService) resolveCheckoutCustomer(ctx context.Context, tenantID, customerID, name, phone string) (*domain.Customer, error) {
	customerID = strings.TrimSpace(customerID)
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)

	if customerID != "" {
		return s.customers.Get(ctx, tenantID, customerID)
	}
	if phone == "" {
		// Tanpa nomor HP, nama pelanggan hanya dicatat pada pesanan sebagai
		// keterangan; tidak ada kartu loyalitas yang dibuat.
		return nil, nil
	}

	existing, err := s.customers.GetByPhone(ctx, tenantID, phone)
	if err == nil {
		return existing, nil
	}
	if !isNotFound(err) {
		return nil, err
	}
	if name == "" {
		name = "Pelanggan " + phone
	}
	return s.Create(ctx, tenantID, CustomerInput{Name: name, Phone: phone})
}

// accrue menambahkan total belanja dan poin loyalitas setelah pembayaran.
func (s *CustomerService) accrue(ctx context.Context, tenantID string, customer *domain.Customer, amount float64, at time.Time) error {
	if customer == nil || amount <= 0 {
		return nil
	}
	customer.TotalSpent += amount
	customer.Points += PointsFor(amount)
	customer.LastPurchase = at
	return s.customers.Update(ctx, tenantID, customer)
}
