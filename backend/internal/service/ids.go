// Package service berisi seluruh aturan bisnis POS Steca. Layer ini hanya
// bergantung pada port di package domain, bukan pada Gin maupun Google API.
package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/slaks37/pos-steca/backend/internal/timex"
)

const idAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // tanpa huruf/angka ambigu

func randomString(n int) string {
	var sb strings.Builder
	max := big.NewInt(int64(len(idAlphabet)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand hanya gagal pada kondisi sistem yang sangat tidak
			// biasa; panik lebih aman daripada menghasilkan ID yang bisa ditebak.
			panic(fmt.Sprintf("gagal membuat ID acak: %v", err))
		}
		sb.WriteByte(idAlphabet[idx.Int64()])
	}
	return sb.String()
}

// NewTenantID membuat ID internal tenant.
func NewTenantID() string { return "tnt_" + strings.ToLower(randomString(12)) }

// NewTenantCode membuat kode bisnis yang diketik kasir saat login.
func NewTenantCode() string { return "STC-" + randomString(5) }

// NewProductID membuat ID produk.
func NewProductID() string { return "PRD-" + randomString(8) }

// NewEmployeeID membuat ID karyawan.
func NewEmployeeID() string { return "EMP-" + randomString(8) }

// NewTransactionID membuat ID transaksi berformat TRX-YYYYMMDD-XXXXX agar
// mudah dicari langsung di Google Sheets.
func NewTransactionID() string {
	return fmt.Sprintf("TRX-%s-%s", timex.Now().Format("20060102"), randomString(5))
}

// NewOrderID membuat ID internal pesanan.
func NewOrderID() string { return "ORD-" + randomString(10) }

// NewOrderCode membuat nomor antrian pendek yang dibacakan ke pelanggan.
func NewOrderCode() string {
	return fmt.Sprintf("#%s-%s", timex.Now().Format("0102"), randomString(3))
}
