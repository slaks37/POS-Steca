// Package domain berisi model inti dan kontrak (port) repository POS Steca.
// Layer ini tidak boleh bergantung pada Gin, Google API, maupun detail
// penyimpanan apa pun.
package domain

import "time"

// Role menentukan level akses pengguna di dalam satu tenant.
type Role string

const (
	RoleOwner Role = "owner"
	RoleKasir Role = "kasir"
)

// Valid memastikan role berada pada daftar yang dikenal sistem.
func (r Role) Valid() bool {
	return r == RoleOwner || r == RoleKasir
}

// OrderStatus mengikuti alur pesanan F&B ala Trofi.
type OrderStatus string

const (
	OrderStatusBaru     OrderStatus = "baru"
	OrderStatusDiproses OrderStatus = "diproses"
	OrderStatusSelesai  OrderStatus = "selesai"
	OrderStatusBatal    OrderStatus = "batal"
)

// Valid memastikan status pesanan dikenal sistem.
func (s OrderStatus) Valid() bool {
	switch s {
	case OrderStatusBaru, OrderStatusDiproses, OrderStatusSelesai, OrderStatusBatal:
		return true
	}
	return false
}

// OrderSource membedakan pesanan dari kasir langsung dan kanal online.
type OrderSource string

const (
	OrderSourceKasir  OrderSource = "kasir"
	OrderSourceOnline OrderSource = "online"
)

// Valid memastikan sumber pesanan dikenal sistem.
func (s OrderSource) Valid() bool {
	return s == OrderSourceKasir || s == OrderSourceOnline
}

// Metode pembayaran yang didukung modul kasir. QRIS dan kartu masih
// placeholder: transaksi tercatat lunas tanpa panggilan ke payment gateway.
const (
	PaymentTunai = "tunai"
	PaymentQRIS  = "qris"
	PaymentKartu = "kartu"
)

// ValidPayment memvalidasi metode pembayaran.
func ValidPayment(m string) bool {
	switch m {
	case PaymentTunai, PaymentQRIS, PaymentKartu:
		return true
	}
	return false
}

// Tenant merepresentasikan satu akun bisnis (UMKM) beserta lokasi
// penyimpanannya di Google Drive milik pemilik akun.
type Tenant struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	BusinessName  string    `json:"business_name"`
	OwnerEmail    string    `json:"owner_email"`
	OwnerName     string    `json:"owner_name"`
	FolderID      string    `json:"folder_id"`
	SpreadsheetID string    `json:"spreadsheet_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// RefreshToken disimpan terenkripsi oleh TenantStore dan tidak pernah
	// dikirim ke klien.
	RefreshToken string `json:"-"`
}

// FolderURL mengembalikan tautan folder Drive milik tenant.
func (t Tenant) FolderURL() string {
	if t.FolderID == "" {
		return ""
	}
	return "https://drive.google.com/drive/folders/" + t.FolderID
}

// SpreadsheetURL mengembalikan tautan spreadsheet datastore tenant.
func (t Tenant) SpreadsheetURL() string {
	if t.SpreadsheetID == "" {
		return ""
	}
	return "https://docs.google.com/spreadsheets/d/" + t.SpreadsheetID
}

// Employee adalah karyawan yang bisa masuk ke aplikasi. Owner otomatis
// dibuat saat onboarding, kasir ditambahkan manual oleh owner.
type Employee struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Active    bool      `json:"active"`
	PINHash   string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`

	// RowNumber adalah nomor baris pada sheet sumber (1-based, termasuk
	// header). Nol berarti baris belum diketahui.
	RowNumber int `json:"-"`
}

// Product adalah satu item jualan yang tersimpan di sheet "Products".
type Product struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
	SKU       string  `json:"sku"`
	ImageURL  string  `json:"image_url"`
	ImageID   string  `json:"image_id"`
	RowNumber int     `json:"-"`
}

// OrderItem adalah baris item pada satu pesanan/transaksi.
type OrderItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Qty       int     `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
	Note      string  `json:"note,omitempty"`
}

// Subtotal menghitung total baris item.
func (i OrderItem) Subtotal() float64 {
	return i.UnitPrice * float64(i.Qty)
}

// TransactionLine adalah satu baris pada sheet "Transactions". Satu transaksi
// dengan tiga item akan menghasilkan tiga baris dengan TransactionID sama.
type TransactionLine struct {
	Date          time.Time `json:"date"`
	TransactionID string    `json:"transaction_id"`
	Item          string    `json:"item"`
	Qty           int       `json:"qty"`
	UnitPrice     float64   `json:"unit_price"`
	Total         float64   `json:"total"`
	PaymentMethod string    `json:"payment_method"`
	Cashier       string    `json:"cashier"`
}

// Transaction adalah tampilan tergabung dari beberapa TransactionLine.
type Transaction struct {
	ID            string      `json:"id"`
	Date          time.Time   `json:"date"`
	Items         []OrderItem `json:"items"`
	Total         float64     `json:"total"`
	PaymentMethod string      `json:"payment_method"`
	Cashier       string      `json:"cashier"`
	AmountPaid    float64     `json:"amount_paid,omitempty"`
	Change        float64     `json:"change,omitempty"`
}

// Order adalah pesanan yang dilacak status pengerjaannya (modul ala Trofi).
type Order struct {
	ID            string      `json:"id"`
	Code          string      `json:"code"`
	TransactionID string      `json:"transaction_id"`
	Status        OrderStatus `json:"status"`
	Source        OrderSource `json:"source"`
	CustomerName  string      `json:"customer_name"`
	TableNo       string      `json:"table_no"`
	Items         []OrderItem `json:"items"`
	Total         float64     `json:"total"`
	Note          string      `json:"note"`
	Cashier       string      `json:"cashier"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	RowNumber     int         `json:"-"`
}
