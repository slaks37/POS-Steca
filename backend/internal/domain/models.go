// Package domain berisi model inti dan kontrak (port) repository POS Steca.
// Layer ini tidak boleh bergantung pada Gin, Google API, maupun detail
// penyimpanan apa pun.
package domain

import (
	"strings"
	"time"
)

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

// TableStatus adalah kondisi meja pada denah bisnis F&B.
type TableStatus string

const (
	// TableStatusKosong berarti meja siap dipakai pelanggan berikutnya.
	TableStatusKosong TableStatus = "kosong"
	// TableStatusTerisi berarti ada pesanan aktif di meja tersebut.
	TableStatusTerisi TableStatus = "terisi"
	// TableStatusDibersihkan berarti tamu sudah pergi dan meja perlu dirapikan.
	TableStatusDibersihkan TableStatus = "dibersihkan"
)

// Valid memastikan status meja dikenal sistem.
func (s TableStatus) Valid() bool {
	switch s {
	case TableStatusKosong, TableStatusTerisi, TableStatusDibersihkan:
		return true
	}
	return false
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

// Tenant merepresentasikan satu akun bisnis (UMKM). Data terstrukturnya
// berada di PostgreSQL; FolderID menunjuk folder Google Drive milik pemilik
// akun tempat gambar produk disimpan.
type Tenant struct {
	ID           string    `json:"id"`
	Code         string    `json:"code"`
	BusinessName string    `json:"business_name"`
	OwnerEmail   string    `json:"owner_email"`
	OwnerName    string    `json:"owner_name"`
	FolderID     string    `json:"folder_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// RefreshToken disimpan terenkripsi oleh TenantStore dan tidak pernah
	// dikirim ke klien.
	RefreshToken string `json:"-"`
}

// FolderURL mengembalikan tautan folder Drive tempat gambar produk tenant
// disimpan.
func (t Tenant) FolderURL() string {
	if t.FolderID == "" {
		return ""
	}
	return "https://drive.google.com/drive/folders/" + t.FolderID
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
}

// Product adalah satu item jualan. ImageID menyimpan file ID Google Drive
// agar gambar lama bisa dibersihkan saat produk diubah atau dihapus.
type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Stock    int     `json:"stock"`
	SKU      string  `json:"sku"`
	ImageURL string  `json:"image_url"`
	ImageID  string  `json:"image_id"`
}

// NormalizePhone menyeragamkan nomor HP Indonesia agar satu pelanggan tidak
// tercatat ganda hanya karena beda penulisan ("0812-3456", "+62 812 3456").
func NormalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	s := digits.String()
	switch {
	case strings.HasPrefix(s, "62"):
		s = "0" + strings.TrimPrefix(s, "62")
	case s != "" && !strings.HasPrefix(s, "0"):
		s = "0" + s
	}
	return s
}

// Customer adalah pelanggan terdaftar pada program loyalitas sederhana.
// Transaksi tanpa pelanggan (anonim) tetap diperbolehkan.
type Customer struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Phone        string    `json:"phone"`
	TotalSpent   float64   `json:"total_spent"`
	Points       int       `json:"points"`
	LastPurchase time.Time `json:"last_purchase"`
	CreatedAt    time.Time `json:"created_at"`
}

// Table adalah satu meja pada denah bisnis F&B.
type Table struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Capacity      int         `json:"capacity"`
	Status        TableStatus `json:"status"`
	Area          string      `json:"area"`
	ActiveOrderID string      `json:"active_order_id"`
	UpdatedAt     time.Time   `json:"updated_at"`
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

// TransactionLine adalah satu baris pada tabel transaction_lines yang bersifat
// append-only. Satu transaksi dengan tiga item menghasilkan tiga baris dengan
// TransactionID yang sama.
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

	// Keterangan loyalitas untuk dicetak di struk. Kosong pada transaksi
	// anonim.
	CustomerName string `json:"customer_name,omitempty"`
	PointsEarned int    `json:"points_earned,omitempty"`
	TotalPoints  int    `json:"total_points,omitempty"`
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

	// CustomerID dan TableID menghubungkan pesanan ke modul CRM dan denah
	// meja. Keduanya boleh kosong: pesanan anonim dan pesanan bawa pulang
	// tetap sah.
	CustomerID string `json:"customer_id"`
	TableID    string `json:"table_id"`
}
