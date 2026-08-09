package memory

import (
	"context"
	"io"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// Adapter tipis di bawah ini memetakan metode Store yang diberi nama unik
// menjadi antarmuka repository domain (Store tidak bisa mengimplementasikan
// beberapa antarmuka sekaligus karena nama metodenya bertabrakan).

// ProductRepo mengadaptasi Store menjadi domain.ProductRepository.
type ProductRepo struct{ s *Store }

// NewProductRepo membuat adapter repository produk in-memory.
func NewProductRepo(s *Store) *ProductRepo { return &ProductRepo{s: s} }

var _ domain.ProductRepository = (*ProductRepo)(nil)

// List meneruskan ke Store.
func (r *ProductRepo) List(ctx context.Context, tenantID string) ([]domain.Product, error) {
	return r.s.ListProducts(ctx, tenantID)
}

// Get meneruskan ke Store.
func (r *ProductRepo) Get(ctx context.Context, tenantID, productID string) (*domain.Product, error) {
	return r.s.GetProduct(ctx, tenantID, productID)
}

// Create meneruskan ke Store.
func (r *ProductRepo) Create(ctx context.Context, tenantID string, p *domain.Product) error {
	return r.s.CreateProduct(ctx, tenantID, p)
}

// Update meneruskan ke Store.
func (r *ProductRepo) Update(ctx context.Context, tenantID string, p *domain.Product) error {
	return r.s.UpdateProduct(ctx, tenantID, p)
}

// Delete meneruskan ke Store.
func (r *ProductRepo) Delete(ctx context.Context, tenantID, productID string) error {
	return r.s.DeleteProduct(ctx, tenantID, productID)
}

// AdjustStock meneruskan ke Store.
func (r *ProductRepo) AdjustStock(ctx context.Context, tenantID string, deltas map[string]int) error {
	return r.s.AdjustStockOf(ctx, tenantID, deltas)
}

// TransactionRepo mengadaptasi Store menjadi domain.TransactionRepository.
type TransactionRepo struct{ s *Store }

// NewTransactionRepo membuat adapter repository transaksi in-memory.
func NewTransactionRepo(s *Store) *TransactionRepo { return &TransactionRepo{s: s} }

var _ domain.TransactionRepository = (*TransactionRepo)(nil)

// Append meneruskan ke Store.
func (r *TransactionRepo) Append(ctx context.Context, tenantID string, lines []domain.TransactionLine) error {
	return r.s.AppendTransaction(ctx, tenantID, lines)
}

// ListLines meneruskan ke Store.
func (r *TransactionRepo) ListLines(ctx context.Context, tenantID string, from, to time.Time) ([]domain.TransactionLine, error) {
	return r.s.ListTransactionLines(ctx, tenantID, from, to)
}

// OrderRepo mengadaptasi Store menjadi domain.OrderRepository.
type OrderRepo struct{ s *Store }

// NewOrderRepo membuat adapter repository pesanan in-memory.
func NewOrderRepo(s *Store) *OrderRepo { return &OrderRepo{s: s} }

var _ domain.OrderRepository = (*OrderRepo)(nil)

// List meneruskan ke Store.
func (r *OrderRepo) List(ctx context.Context, tenantID string, f domain.OrderFilter) ([]domain.Order, error) {
	return r.s.ListOrders(ctx, tenantID, f)
}

// Get meneruskan ke Store.
func (r *OrderRepo) Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error) {
	return r.s.GetOrder(ctx, tenantID, orderID)
}

// Create meneruskan ke Store.
func (r *OrderRepo) Create(ctx context.Context, tenantID string, o *domain.Order) error {
	return r.s.CreateOrder(ctx, tenantID, o)
}

// Update meneruskan ke Store.
func (r *OrderRepo) Update(ctx context.Context, tenantID string, o *domain.Order) error {
	return r.s.UpdateOrder(ctx, tenantID, o)
}

// EmployeeRepo mengadaptasi Store menjadi domain.EmployeeRepository.
type EmployeeRepo struct{ s *Store }

// NewEmployeeRepo membuat adapter repository karyawan in-memory.
func NewEmployeeRepo(s *Store) *EmployeeRepo { return &EmployeeRepo{s: s} }

var _ domain.EmployeeRepository = (*EmployeeRepo)(nil)

// List meneruskan ke Store.
func (r *EmployeeRepo) List(ctx context.Context, tenantID string) ([]domain.Employee, error) {
	return r.s.ListEmployees(ctx, tenantID)
}

// Get meneruskan ke Store.
func (r *EmployeeRepo) Get(ctx context.Context, tenantID, employeeID string) (*domain.Employee, error) {
	return r.s.GetEmployee(ctx, tenantID, employeeID)
}

// GetByEmail meneruskan ke Store.
func (r *EmployeeRepo) GetByEmail(ctx context.Context, tenantID, email string) (*domain.Employee, error) {
	return r.s.GetEmployeeByEmail(ctx, tenantID, email)
}

// Create meneruskan ke Store.
func (r *EmployeeRepo) Create(ctx context.Context, tenantID string, e *domain.Employee) error {
	return r.s.CreateEmployee(ctx, tenantID, e)
}

// Update meneruskan ke Store.
func (r *EmployeeRepo) Update(ctx context.Context, tenantID string, e *domain.Employee) error {
	return r.s.UpdateEmployee(ctx, tenantID, e)
}

// Delete meneruskan ke Store.
func (r *EmployeeRepo) Delete(ctx context.Context, tenantID, employeeID string) error {
	return r.s.DeleteEmployee(ctx, tenantID, employeeID)
}

// Storage mengadaptasi Store menjadi domain.FileStorage.
type Storage struct{ s *Store }

// NewStorage membuat adapter penyimpanan berkas in-memory.
func NewStorage(s *Store) *Storage { return &Storage{s: s} }

var _ domain.FileStorage = (*Storage)(nil)

// Upload meneruskan ke Store.
func (r *Storage) Upload(ctx context.Context, tenantID, filename, contentType string, rd io.Reader) (string, string, error) {
	return r.s.UploadFile(ctx, tenantID, filename, contentType, rd)
}

// Delete meneruskan ke Store.
func (r *Storage) Delete(ctx context.Context, tenantID, fileID string) error {
	return r.s.DeleteFile(ctx, tenantID, fileID)
}
