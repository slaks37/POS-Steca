package gsheets

import (
	"strconv"
	"testing"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

func TestParseFloatToleransiFormatRupiah(t *testing.T) {
	cases := map[string]float64{
		"":            0,
		"15000":       15000,
		"15.000,50":   15000.50,
		"Rp 15.000":   15000,
		"12,5":        12.5,
		"bukan angka": 0,
	}
	for input, want := range cases {
		if got := parseFloat(input); got != want {
			t.Errorf("parseFloat(%q) = %v, ingin %v", input, got, want)
		}
	}
}

func TestProductRoundTrip(t *testing.T) {
	original := domain.Product{
		ID:       "PRD-123",
		Name:     "Es Teh Manis",
		Category: "Minuman",
		Price:    6000,
		Stock:    42,
		SKU:      "MNM-001",
		ImageURL: "https://drive.google.com/thumbnail?id=abc",
		ImageID:  "abc",
	}

	row := productToRow(original)
	if len(row) != len(productHeader) {
		t.Fatalf("jumlah kolom = %d, ingin %d", len(row), len(productHeader))
	}

	cells := make([]string, len(row))
	for i, v := range row {
		cells[i] = asCell(v)
	}
	got := productFromRow(cells, 5)

	if got.RowNumber != 5 {
		t.Errorf("RowNumber = %d, ingin 5", got.RowNumber)
	}
	got.RowNumber = 0
	if got != original {
		t.Errorf("hasil round-trip = %+v, ingin %+v", got, original)
	}
}

func TestProductFromRowBarisPendek(t *testing.T) {
	// Google Sheets memangkas sel kosong di ujung kanan baris.
	p := productFromRow([]string{"PRD-1", "Kopi"}, 2)
	if p.ID != "PRD-1" || p.Name != "Kopi" {
		t.Fatalf("baris pendek tidak terbaca: %+v", p)
	}
	if p.Price != 0 || p.Stock != 0 || p.ImageURL != "" {
		t.Errorf("kolom kosong harus bernilai nol, dapat %+v", p)
	}
}

func TestOrderRoundTripDenganItemJSON(t *testing.T) {
	now := timex.Now().Truncate(time.Second)
	original := domain.Order{
		ID:            "ORD-1",
		Code:          "#0101-ABC",
		TransactionID: "TRX-20260101-XYZ12",
		CreatedAt:     now,
		UpdatedAt:     now,
		Status:        domain.OrderStatusDiproses,
		Source:        domain.OrderSourceOnline,
		CustomerName:  "Siti",
		TableNo:       "A2",
		Items: []domain.OrderItem{
			{ProductID: "PRD-1", Name: "Nasi Goreng", Qty: 2, UnitPrice: 25000, Note: "pedas"},
		},
		Total:   50000,
		Note:    "bungkus",
		Cashier: "Kasir Demo",
	}

	row := orderToRow(original)
	cells := make([]string, len(row))
	for i, v := range row {
		cells[i] = asCell(v)
	}
	got := orderFromRow(cells, 3)

	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, ingin %v", got.CreatedAt, original.CreatedAt)
	}
	if got.Status != original.Status || got.Source != original.Source {
		t.Errorf("status/sumber tidak cocok: %v/%v", got.Status, got.Source)
	}
	if len(got.Items) != 1 || got.Items[0] != original.Items[0] {
		t.Errorf("item pesanan = %+v, ingin %+v", got.Items, original.Items)
	}
	if got.Total != original.Total || got.Note != original.Note {
		t.Errorf("total/catatan tidak cocok: %v/%q", got.Total, got.Note)
	}
}

func TestEmployeeStatusNonaktif(t *testing.T) {
	e := domain.Employee{ID: "EMP-1", Name: "Budi", Email: "Budi@Toko.com", Role: domain.RoleKasir, Active: false}
	row := employeeToRow(e)
	cells := make([]string, len(row))
	for i, v := range row {
		cells[i] = asCell(v)
	}
	got := employeeFromRow(cells, 2)

	if got.Active {
		t.Error("karyawan nonaktif terbaca sebagai aktif")
	}
	if got.Email != "budi@toko.com" {
		t.Errorf("email = %q, ingin dinormalkan ke huruf kecil", got.Email)
	}
}

func TestTransactionLineRoundTrip(t *testing.T) {
	now := timex.Now().Truncate(time.Second)
	original := domain.TransactionLine{
		Date:          now,
		TransactionID: "TRX-20260101-AAAAA",
		Item:          "Kopi Susu",
		Qty:           3,
		UnitPrice:     18000,
		Total:         54000,
		PaymentMethod: domain.PaymentQRIS,
		Cashier:       "Ani",
	}
	row := transactionLineToRow(original)
	cells := make([]string, len(row))
	for i, v := range row {
		cells[i] = asCell(v)
	}
	got := transactionLineFromRow(cells)

	if !got.Date.Equal(original.Date) {
		t.Errorf("Tanggal = %v, ingin %v", got.Date, original.Date)
	}
	got.Date = original.Date
	if got != original {
		t.Errorf("round-trip = %+v, ingin %+v", got, original)
	}
}

// asCell meniru cara Google Sheets mengembalikan nilai sel sebagai string.
func asCell(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}
