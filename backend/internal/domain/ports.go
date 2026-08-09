package domain

import (
	"context"
	"io"
	"time"
)

// TenantStore menyimpan metadata tenant beserta refresh token Google-nya.
// Ini satu-satunya penyimpanan lokal di luar Google Drive/Sheets: dibutuhkan
// sebagai "bootstrap" agar backend tahu spreadsheet mana milik siapa.
type TenantStore interface {
	Create(ctx context.Context, t *Tenant) error
	Update(ctx context.Context, t *Tenant) error
	GetByID(ctx context.Context, id string) (*Tenant, error)
	GetByCode(ctx context.Context, code string) (*Tenant, error)
	GetByOwnerEmail(ctx context.Context, email string) (*Tenant, error)
	List(ctx context.Context) ([]*Tenant, error)
}

// ProductRepository memetakan sheet "Products" milik satu tenant.
type ProductRepository interface {
	List(ctx context.Context, tenantID string) ([]Product, error)
	Get(ctx context.Context, tenantID, productID string) (*Product, error)
	Create(ctx context.Context, tenantID string, p *Product) error
	Update(ctx context.Context, tenantID string, p *Product) error
	Delete(ctx context.Context, tenantID, productID string) error
	// AdjustStock mengurangi (delta negatif) atau menambah stok beberapa
	// produk sekaligus dalam satu operasi tulis.
	AdjustStock(ctx context.Context, tenantID string, deltas map[string]int) error
}

// TransactionRepository memetakan sheet "Transactions" milik satu tenant.
type TransactionRepository interface {
	Append(ctx context.Context, tenantID string, lines []TransactionLine) error
	// ListLines mengembalikan seluruh baris transaksi pada rentang waktu
	// [from, to). Nilai nol pada from/to berarti tanpa batas.
	ListLines(ctx context.Context, tenantID string, from, to time.Time) ([]TransactionLine, error)
}

// OrderRepository memetakan sheet "Orders" milik satu tenant.
type OrderRepository interface {
	List(ctx context.Context, tenantID string, f OrderFilter) ([]Order, error)
	Get(ctx context.Context, tenantID, orderID string) (*Order, error)
	Create(ctx context.Context, tenantID string, o *Order) error
	Update(ctx context.Context, tenantID string, o *Order) error
}

// OrderFilter menyaring daftar pesanan.
type OrderFilter struct {
	Status OrderStatus
	Source OrderSource
	From   time.Time
	To     time.Time
}

// EmployeeRepository memetakan sheet "Employees" milik satu tenant.
type EmployeeRepository interface {
	List(ctx context.Context, tenantID string) ([]Employee, error)
	Get(ctx context.Context, tenantID, employeeID string) (*Employee, error)
	GetByEmail(ctx context.Context, tenantID, email string) (*Employee, error)
	Create(ctx context.Context, tenantID string, e *Employee) error
	Update(ctx context.Context, tenantID string, e *Employee) error
	Delete(ctx context.Context, tenantID, employeeID string) error
}

// FileStorage adalah abstraksi penyimpanan gambar produk (Google Drive).
type FileStorage interface {
	// Upload menyimpan berkas ke folder milik tenant dan mengembalikan
	// ID berkas serta URL yang bisa ditampilkan di frontend.
	Upload(ctx context.Context, tenantID, filename, contentType string, r io.Reader) (fileID string, url string, err error)
	Delete(ctx context.Context, tenantID, fileID string) error
}

// Provisioner menyiapkan folder Drive dan spreadsheet saat onboarding tenant.
type Provisioner interface {
	Provision(ctx context.Context, t *Tenant) error
}
