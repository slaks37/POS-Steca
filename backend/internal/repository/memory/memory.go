// Package memory menyediakan implementasi seluruh port repository di dalam
// memori. Driver ini HANYA untuk pengembangan lokal dan demo (POS_DATASTORE=memory)
// sehingga aplikasi bisa dijalankan tanpa kredensial Google. Data hilang saat
// proses berhenti.
package memory

import (
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// Store menampung seluruh data seluruh tenant di dalam memori.
type Store struct {
	mu sync.RWMutex

	tenants   map[string]*domain.Tenant
	products  map[string][]domain.Product
	txLines   map[string][]domain.TransactionLine
	orders    map[string][]domain.Order
	employees map[string][]domain.Employee
	customers map[string][]domain.Customer
	tables    map[string][]domain.Table
	files     map[string]map[string]storedFile

	fileBaseURL string
}

type storedFile struct {
	name        string
	contentType string
	data        []byte
}

// NewStore membuat penyimpanan in-memory. fileBaseURL dipakai untuk membentuk
// URL gambar yang dilayani backend, misal "http://localhost:8080/api/v1/files".
func NewStore(fileBaseURL string) *Store {
	return &Store{
		tenants:     map[string]*domain.Tenant{},
		products:    map[string][]domain.Product{},
		txLines:     map[string][]domain.TransactionLine{},
		orders:      map[string][]domain.Order{},
		employees:   map[string][]domain.Employee{},
		customers:   map[string][]domain.Customer{},
		tables:      map[string][]domain.Table{},
		files:       map[string]map[string]storedFile{},
		fileBaseURL: strings.TrimSuffix(fileBaseURL, "/"),
	}
}

// ---------- TenantStore ----------

var _ domain.TenantStore = (*Store)(nil)

// Create menyimpan tenant baru.
func (s *Store) Create(_ context.Context, t *domain.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[t.ID]; ok {
		return apperr.Conflict("tenant sudah terdaftar")
	}
	clone := *t
	s.tenants[t.ID] = &clone
	return nil
}

// Update memperbarui tenant.
func (s *Store) Update(_ context.Context, t *domain.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[t.ID]; !ok {
		return apperr.NotFound("tenant tidak ditemukan")
	}
	t.UpdatedAt = time.Now().UTC()
	clone := *t
	s.tenants[t.ID] = &clone
	return nil
}

// GetByID mencari tenant berdasarkan ID.
func (s *Store) GetByID(_ context.Context, id string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, apperr.NotFound("tenant tidak ditemukan")
	}
	clone := *t
	return &clone, nil
}

// GetByCode mencari tenant berdasarkan kode bisnis.
func (s *Store) GetByCode(_ context.Context, code string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, t := range s.tenants {
		if strings.ToUpper(t.Code) == code {
			clone := *t
			return &clone, nil
		}
	}
	return nil, apperr.NotFound("kode bisnis tidak ditemukan")
}

// GetByOwnerEmail mencari tenant berdasarkan email pemilik.
func (s *Store) GetByOwnerEmail(_ context.Context, email string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	email = strings.ToLower(strings.TrimSpace(email))
	for _, t := range s.tenants {
		if strings.EqualFold(t.OwnerEmail, email) {
			clone := *t
			return &clone, nil
		}
	}
	return nil, apperr.NotFound("tenant tidak ditemukan")
}

// List mengembalikan seluruh tenant.
func (s *Store) List(_ context.Context) ([]*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		clone := *t
		out = append(out, &clone)
	}
	return out, nil
}

// Provision menandai tenant sebagai siap pakai (tanpa Drive sungguhan).
func (s *Store) Provision(_ context.Context, t *domain.Tenant) error {
	t.FolderID = "memory-folder-" + t.ID
	t.SpreadsheetID = "memory-sheet-" + t.ID
	return nil
}

var _ domain.Provisioner = (*Store)(nil)

// ---------- ProductRepository ----------
//
// Metode di bawah ini diberi nama unik karena satu Store menampung banyak
// jenis data; adapter di adapters.go yang memetakannya ke antarmuka domain.

// ListProducts mengembalikan katalog produk tenant.
func (s *Store) ListProducts(_ context.Context, tenantID string) ([]domain.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Product, len(s.products[tenantID]))
	copy(out, s.products[tenantID])
	return out, nil
}

// GetProduct mencari produk berdasarkan ID.
func (s *Store) GetProduct(ctx context.Context, tenantID, productID string) (*domain.Product, error) {
	items, _ := s.ListProducts(ctx, tenantID)
	for i := range items {
		if items[i].ID == productID {
			p := items[i]
			return &p, nil
		}
	}
	return nil, apperr.NotFound("produk tidak ditemukan")
}

// CreateProduct menambahkan produk.
func (s *Store) CreateProduct(_ context.Context, tenantID string, p *domain.Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[tenantID] = append(s.products[tenantID], *p)
	return nil
}

// UpdateProduct memperbarui produk.
func (s *Store) UpdateProduct(_ context.Context, tenantID string, p *domain.Product) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.products[tenantID]
	for i := range list {
		if list[i].ID == p.ID {
			list[i] = *p
			return nil
		}
	}
	return apperr.NotFound("produk tidak ditemukan")
}

// DeleteProduct menghapus produk.
func (s *Store) DeleteProduct(_ context.Context, tenantID, productID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.products[tenantID]
	for i := range list {
		if list[i].ID == productID {
			s.products[tenantID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return apperr.NotFound("produk tidak ditemukan")
}

// AdjustStockOf menerapkan perubahan stok.
func (s *Store) AdjustStockOf(_ context.Context, tenantID string, deltas map[string]int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.products[tenantID]
	for i := range list {
		if delta, ok := deltas[list[i].ID]; ok {
			list[i].Stock += delta
			if list[i].Stock < 0 {
				list[i].Stock = 0
			}
		}
	}
	return nil
}

// ---------- TransactionRepository ----------

// AppendTransaction menambahkan baris transaksi.
func (s *Store) AppendTransaction(_ context.Context, tenantID string, lines []domain.TransactionLine) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txLines[tenantID] = append(s.txLines[tenantID], lines...)
	return nil
}

// ListTransactionLines membaca baris transaksi pada rentang waktu.
func (s *Store) ListTransactionLines(_ context.Context, tenantID string, from, to time.Time) ([]domain.TransactionLine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.TransactionLine, 0, len(s.txLines[tenantID]))
	for _, l := range s.txLines[tenantID] {
		if !from.IsZero() && l.Date.Before(from) {
			continue
		}
		if !to.IsZero() && !l.Date.Before(to) {
			continue
		}
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	return out, nil
}

// ---------- OrderRepository ----------

// ListOrders mengembalikan pesanan sesuai filter.
func (s *Store) ListOrders(_ context.Context, tenantID string, f domain.OrderFilter) ([]domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Order, 0, len(s.orders[tenantID]))
	for _, o := range s.orders[tenantID] {
		if f.Status != "" && o.Status != f.Status {
			continue
		}
		if f.Source != "" && o.Source != f.Source {
			continue
		}
		if !f.From.IsZero() && o.CreatedAt.Before(f.From) {
			continue
		}
		if !f.To.IsZero() && !o.CreatedAt.Before(f.To) {
			continue
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// GetOrder mencari pesanan berdasarkan ID.
func (s *Store) GetOrder(_ context.Context, tenantID, orderID string) (*domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.orders[tenantID] {
		if o.ID == orderID {
			clone := o
			return &clone, nil
		}
	}
	return nil, apperr.NotFound("pesanan tidak ditemukan")
}

// CreateOrder menambahkan pesanan.
func (s *Store) CreateOrder(_ context.Context, tenantID string, o *domain.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[tenantID] = append(s.orders[tenantID], *o)
	return nil
}

// UpdateOrder memperbarui pesanan.
func (s *Store) UpdateOrder(_ context.Context, tenantID string, o *domain.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.orders[tenantID]
	for i := range list {
		if list[i].ID == o.ID {
			list[i] = *o
			return nil
		}
	}
	return apperr.NotFound("pesanan tidak ditemukan")
}

// ---------- EmployeeRepository ----------

// ListEmployees mengembalikan karyawan tenant.
func (s *Store) ListEmployees(_ context.Context, tenantID string) ([]domain.Employee, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Employee, len(s.employees[tenantID]))
	copy(out, s.employees[tenantID])
	return out, nil
}

// GetEmployee mencari karyawan berdasarkan ID.
func (s *Store) GetEmployee(ctx context.Context, tenantID, employeeID string) (*domain.Employee, error) {
	items, _ := s.ListEmployees(ctx, tenantID)
	for i := range items {
		if items[i].ID == employeeID {
			e := items[i]
			return &e, nil
		}
	}
	return nil, apperr.NotFound("karyawan tidak ditemukan")
}

// GetEmployeeByEmail mencari karyawan berdasarkan email.
func (s *Store) GetEmployeeByEmail(ctx context.Context, tenantID, email string) (*domain.Employee, error) {
	items, _ := s.ListEmployees(ctx, tenantID)
	email = strings.ToLower(strings.TrimSpace(email))
	for i := range items {
		if strings.EqualFold(items[i].Email, email) {
			e := items[i]
			return &e, nil
		}
	}
	return nil, apperr.NotFound("karyawan tidak ditemukan")
}

// CreateEmployee menambahkan karyawan.
func (s *Store) CreateEmployee(_ context.Context, tenantID string, e *domain.Employee) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.employees[tenantID] = append(s.employees[tenantID], *e)
	return nil
}

// UpdateEmployee memperbarui karyawan.
func (s *Store) UpdateEmployee(_ context.Context, tenantID string, e *domain.Employee) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.employees[tenantID]
	for i := range list {
		if list[i].ID == e.ID {
			list[i] = *e
			return nil
		}
	}
	return apperr.NotFound("karyawan tidak ditemukan")
}

// DeleteEmployee menghapus karyawan.
func (s *Store) DeleteEmployee(_ context.Context, tenantID, employeeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.employees[tenantID]
	for i := range list {
		if list[i].ID == employeeID {
			s.employees[tenantID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return apperr.NotFound("karyawan tidak ditemukan")
}

// ---------- CustomerRepository ----------

// ListCustomers mengembalikan pelanggan tenant.
func (s *Store) ListCustomers(_ context.Context, tenantID string) ([]domain.Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Customer, len(s.customers[tenantID]))
	copy(out, s.customers[tenantID])
	return out, nil
}

// GetCustomer mencari pelanggan berdasarkan ID.
func (s *Store) GetCustomer(ctx context.Context, tenantID, customerID string) (*domain.Customer, error) {
	items, _ := s.ListCustomers(ctx, tenantID)
	for i := range items {
		if items[i].ID == customerID {
			c := items[i]
			return &c, nil
		}
	}
	return nil, apperr.NotFound("pelanggan tidak ditemukan")
}

// GetCustomerByPhone mencari pelanggan berdasarkan nomor HP.
func (s *Store) GetCustomerByPhone(ctx context.Context, tenantID, phone string) (*domain.Customer, error) {
	phone = domain.NormalizePhone(phone)
	if phone == "" {
		return nil, apperr.NotFound("pelanggan tidak ditemukan")
	}
	items, _ := s.ListCustomers(ctx, tenantID)
	for i := range items {
		if domain.NormalizePhone(items[i].Phone) == phone {
			c := items[i]
			return &c, nil
		}
	}
	return nil, apperr.NotFound("pelanggan tidak ditemukan")
}

// CreateCustomer menambahkan pelanggan.
func (s *Store) CreateCustomer(_ context.Context, tenantID string, c *domain.Customer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customers[tenantID] = append(s.customers[tenantID], *c)
	return nil
}

// UpdateCustomer memperbarui pelanggan.
func (s *Store) UpdateCustomer(_ context.Context, tenantID string, c *domain.Customer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.customers[tenantID]
	for i := range list {
		if list[i].ID == c.ID {
			list[i] = *c
			return nil
		}
	}
	return apperr.NotFound("pelanggan tidak ditemukan")
}

// DeleteCustomer menghapus pelanggan.
func (s *Store) DeleteCustomer(_ context.Context, tenantID, customerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.customers[tenantID]
	for i := range list {
		if list[i].ID == customerID {
			s.customers[tenantID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return apperr.NotFound("pelanggan tidak ditemukan")
}

// ---------- TableRepository ----------

// ListTables mengembalikan meja tenant.
func (s *Store) ListTables(_ context.Context, tenantID string) ([]domain.Table, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Table, len(s.tables[tenantID]))
	copy(out, s.tables[tenantID])
	return out, nil
}

// GetTable mencari meja berdasarkan ID.
func (s *Store) GetTable(ctx context.Context, tenantID, tableID string) (*domain.Table, error) {
	items, _ := s.ListTables(ctx, tenantID)
	for i := range items {
		if items[i].ID == tableID {
			tb := items[i]
			return &tb, nil
		}
	}
	return nil, apperr.NotFound("meja tidak ditemukan")
}

// CreateTable menambahkan meja.
func (s *Store) CreateTable(_ context.Context, tenantID string, tb *domain.Table) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tables[tenantID] = append(s.tables[tenantID], *tb)
	return nil
}

// UpdateTable memperbarui meja.
func (s *Store) UpdateTable(_ context.Context, tenantID string, tb *domain.Table) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.tables[tenantID]
	for i := range list {
		if list[i].ID == tb.ID {
			list[i] = *tb
			return nil
		}
	}
	return apperr.NotFound("meja tidak ditemukan")
}

// DeleteTable menghapus meja.
func (s *Store) DeleteTable(_ context.Context, tenantID, tableID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.tables[tenantID]
	for i := range list {
		if list[i].ID == tableID {
			s.tables[tenantID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return apperr.NotFound("meja tidak ditemukan")
}

// ---------- FileStorage ----------

// UploadFile menyimpan gambar di memori dan mengembalikan URL yang dilayani
// backend.
func (s *Store) UploadFile(_ context.Context, tenantID, filename, contentType string, r io.Reader) (string, string, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", "", apperr.Internal("gagal membaca berkas").WithCause(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files[tenantID] == nil {
		s.files[tenantID] = map[string]storedFile{}
	}
	id := filename
	s.files[tenantID][id] = storedFile{name: filename, contentType: contentType, data: buf.Bytes()}
	return id, s.fileBaseURL + "/" + tenantID + "/" + id, nil
}

// DeleteFile menghapus gambar dari memori.
func (s *Store) DeleteFile(_ context.Context, tenantID, fileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files[tenantID], fileID)
	return nil
}

// ReadFile mengembalikan isi berkas untuk dilayani lewat HTTP.
func (s *Store) ReadFile(tenantID, fileID string) ([]byte, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.files[tenantID][fileID]
	if !ok {
		return nil, "", false
	}
	return f.data, f.contentType, true
}
