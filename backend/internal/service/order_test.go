package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// --- Repository tiruan ---

type fakeProducts struct {
	items       []domain.Product
	stockCalls  []map[string]int
	failAdjust  bool
	failListErr error
}

func (f *fakeProducts) List(context.Context, string) ([]domain.Product, error) {
	if f.failListErr != nil {
		return nil, f.failListErr
	}
	out := make([]domain.Product, len(f.items))
	copy(out, f.items)
	return out, nil
}

func (f *fakeProducts) Get(_ context.Context, _, id string) (*domain.Product, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			p := f.items[i]
			return &p, nil
		}
	}
	return nil, apperr.NotFound("produk tidak ditemukan")
}

func (f *fakeProducts) Create(_ context.Context, _ string, p *domain.Product) error {
	f.items = append(f.items, *p)
	return nil
}

func (f *fakeProducts) Update(_ context.Context, _ string, p *domain.Product) error {
	for i := range f.items {
		if f.items[i].ID == p.ID {
			f.items[i] = *p
			return nil
		}
	}
	return apperr.NotFound("produk tidak ditemukan")
}

func (f *fakeProducts) Delete(_ context.Context, _, id string) error {
	for i := range f.items {
		if f.items[i].ID == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return apperr.NotFound("produk tidak ditemukan")
}

func (f *fakeProducts) AdjustStock(_ context.Context, _ string, deltas map[string]int) error {
	f.stockCalls = append(f.stockCalls, deltas)
	if f.failAdjust {
		return errors.New("sheets sedang bermasalah")
	}
	for i := range f.items {
		if d, ok := deltas[f.items[i].ID]; ok {
			f.items[i].Stock += d
		}
	}
	return nil
}

type fakeOrders struct{ items []domain.Order }

func (f *fakeOrders) List(_ context.Context, _ string, filter domain.OrderFilter) ([]domain.Order, error) {
	out := []domain.Order{}
	for _, o := range f.items {
		if filter.Status != "" && o.Status != filter.Status {
			continue
		}
		if filter.Source != "" && o.Source != filter.Source {
			continue
		}
		out = append(out, o)
	}
	return out, nil
}

func (f *fakeOrders) Get(_ context.Context, _, id string) (*domain.Order, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			o := f.items[i]
			return &o, nil
		}
	}
	return nil, apperr.NotFound("pesanan tidak ditemukan")
}

func (f *fakeOrders) Create(_ context.Context, _ string, o *domain.Order) error {
	f.items = append(f.items, *o)
	return nil
}

func (f *fakeOrders) Update(_ context.Context, _ string, o *domain.Order) error {
	for i := range f.items {
		if f.items[i].ID == o.ID {
			f.items[i] = *o
			return nil
		}
	}
	return apperr.NotFound("pesanan tidak ditemukan")
}

type fakeTransactions struct{ lines []domain.TransactionLine }

func (f *fakeTransactions) Append(_ context.Context, _ string, lines []domain.TransactionLine) error {
	f.lines = append(f.lines, lines...)
	return nil
}

func (f *fakeTransactions) ListLines(_ context.Context, _ string, from, to time.Time) ([]domain.TransactionLine, error) {
	out := []domain.TransactionLine{}
	for _, l := range f.lines {
		if !from.IsZero() && l.Date.Before(from) {
			continue
		}
		if !to.IsZero() && !l.Date.Before(to) {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func newOrderFixture() (*OrderService, *fakeProducts, *fakeOrders, *fakeTransactions) {
	products := &fakeProducts{items: []domain.Product{
		{ID: "P1", Name: "Nasi Goreng", Price: 25000, Stock: 10},
		{ID: "P2", Name: "Es Teh", Price: 6000, Stock: 3},
	}}
	orders := &fakeOrders{}
	transactions := &fakeTransactions{}
	return NewOrderService(products, orders, transactions), products, orders, transactions
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error bukan apperr.Error: %v", err)
	}
	return appErr.Code
}

// --- Pengujian ---

func TestCheckoutMenghitungTotalDanStruk(t *testing.T) {
	svc, products, orders, transactions := newOrderFixture()

	result, err := svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 2}, {ProductID: "P2", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
		AmountPaid:    100000,
		CustomerName:  "Budi",
	})
	if err != nil {
		t.Fatalf("checkout gagal: %v", err)
	}

	if result.Order.Total != 56000 {
		t.Errorf("total pesanan = %v, ingin 56000", result.Order.Total)
	}
	if result.Order.Status != domain.OrderStatusBaru || result.Order.Source != domain.OrderSourceKasir {
		t.Errorf("pesanan kasir harus berstatus baru dan bersumber kasir, dapat %v/%v",
			result.Order.Status, result.Order.Source)
	}
	if result.Receipt == nil || result.Receipt.Change != 44000 {
		t.Errorf("kembalian = %+v, ingin 44000", result.Receipt)
	}

	// Satu baris transaksi per item.
	if len(transactions.lines) != 2 {
		t.Fatalf("jumlah baris transaksi = %d, ingin 2", len(transactions.lines))
	}
	for _, l := range transactions.lines {
		if l.TransactionID != result.Order.TransactionID {
			t.Errorf("baris transaksi memakai ID %q, ingin %q", l.TransactionID, result.Order.TransactionID)
		}
	}

	if len(orders.items) != 1 {
		t.Errorf("pesanan tersimpan = %d, ingin 1", len(orders.items))
	}
	if products.items[0].Stock != 8 || products.items[1].Stock != 2 {
		t.Errorf("stok setelah penjualan = %d/%d, ingin 8/2", products.items[0].Stock, products.items[1].Stock)
	}
}

func TestCheckoutMenggabungkanItemGanda(t *testing.T) {
	svc, _, _, transactions := newOrderFixture()

	result, err := svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}, {ProductID: "P1", Qty: 2, Note: "pedas"}},
		PaymentMethod: domain.PaymentQRIS,
	})
	if err != nil {
		t.Fatalf("checkout gagal: %v", err)
	}
	if len(result.Order.Items) != 1 || result.Order.Items[0].Qty != 3 {
		t.Errorf("item ganda harus digabung, dapat %+v", result.Order.Items)
	}
	if len(transactions.lines) != 1 {
		t.Errorf("baris transaksi = %d, ingin 1", len(transactions.lines))
	}
}

func TestCheckoutMenolakStokKurang(t *testing.T) {
	svc, _, _, transactions := newOrderFixture()

	_, err := svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P2", Qty: 5}}, // stok hanya 3
		PaymentMethod: domain.PaymentTunai,
	})
	if err == nil {
		t.Fatal("checkout dengan stok kurang seharusnya gagal")
	}
	if code := codeOf(t, err); code != "conflict" {
		t.Errorf("kode error = %q, ingin conflict", code)
	}
	if len(transactions.lines) != 0 {
		t.Error("tidak boleh ada transaksi tercatat saat validasi gagal")
	}
}

func TestCheckoutMenolakInputTidakValid(t *testing.T) {
	svc, _, _, _ := newOrderFixture()
	ctx := context.Background()

	cases := []struct {
		nama  string
		input CheckoutInput
		kode  string
	}{
		{"keranjang kosong", CheckoutInput{PaymentMethod: domain.PaymentTunai}, "bad_request"},
		{"metode tidak dikenal", CheckoutInput{Items: []CartItem{{ProductID: "P1", Qty: 1}}, PaymentMethod: "gopay"}, "bad_request"},
		{"qty nol", CheckoutInput{Items: []CartItem{{ProductID: "P1", Qty: 0}}, PaymentMethod: domain.PaymentTunai}, "bad_request"},
		{"produk asing", CheckoutInput{Items: []CartItem{{ProductID: "P9", Qty: 1}}, PaymentMethod: domain.PaymentTunai}, "bad_request"},
		{
			"uang kurang",
			CheckoutInput{Items: []CartItem{{ProductID: "P1", Qty: 1}}, PaymentMethod: domain.PaymentTunai, AmountPaid: 1000},
			"bad_request",
		},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			if _, err := svc.Checkout(ctx, "T1", "Ani", tc.input); err == nil {
				t.Fatal("seharusnya gagal")
			} else if code := codeOf(t, err); code != tc.kode {
				t.Errorf("kode error = %q, ingin %q", code, tc.kode)
			}
		})
	}
}

func TestCheckoutTetapBerhasilSaatSinkronStokGagal(t *testing.T) {
	svc, products, _, transactions := newOrderFixture()
	products.failAdjust = true

	result, err := svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
	})
	if err != nil {
		t.Fatalf("penjualan sudah tercatat, checkout tidak boleh gagal: %v", err)
	}
	if result.Receipt == nil {
		t.Error("struk tetap harus diterbitkan")
	}
	if len(transactions.lines) != 1 {
		t.Error("transaksi tetap harus tercatat")
	}
}

func TestOnlineOrderBelumTerbayar(t *testing.T) {
	svc, products, _, transactions := newOrderFixture()

	order, err := svc.CreateOnlineOrder(context.Background(), "T1", OnlineOrderInput{
		Items:        []CartItem{{ProductID: "P1", Qty: 2}},
		CustomerName: "Siti",
	})
	if err != nil {
		t.Fatalf("pesanan online gagal: %v", err)
	}
	if order.TransactionID != "" {
		t.Error("pesanan online belum boleh punya ID transaksi")
	}
	if order.Source != domain.OrderSourceOnline || order.Status != domain.OrderStatusBaru {
		t.Errorf("sumber/status = %v/%v, ingin online/baru", order.Source, order.Status)
	}
	if len(transactions.lines) != 0 {
		t.Error("pesanan online belum boleh mencatat transaksi")
	}
	if products.items[0].Stock != 8 {
		t.Errorf("stok = %d, ingin 8 (dipotong saat pesanan masuk)", products.items[0].Stock)
	}
}

func TestOnlineOrderWajibNamaPemesan(t *testing.T) {
	svc, _, _, _ := newOrderFixture()
	_, err := svc.CreateOnlineOrder(context.Background(), "T1", OnlineOrderInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}},
	})
	if err == nil {
		t.Fatal("nama pemesan kosong seharusnya ditolak")
	}
	if code := codeOf(t, err); code != "bad_request" {
		t.Errorf("kode error = %q, ingin bad_request", code)
	}
}

func TestUpdateStatusAlurPesanan(t *testing.T) {
	svc, _, _, _ := newOrderFixture()
	ctx := context.Background()

	order, err := svc.CreateOnlineOrder(ctx, "T1", OnlineOrderInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, CustomerName: "Siti",
	})
	if err != nil {
		t.Fatalf("persiapan gagal: %v", err)
	}

	updated, err := svc.UpdateStatus(ctx, "T1", order.ID, "diproses")
	if err != nil {
		t.Fatalf("ubah status gagal: %v", err)
	}
	if updated.Status != domain.OrderStatusDiproses {
		t.Errorf("status = %v, ingin diproses", updated.Status)
	}

	if _, err := svc.UpdateStatus(ctx, "T1", order.ID, "entahlah"); err == nil {
		t.Error("status tidak dikenal seharusnya ditolak")
	}
}

func TestBatalkanPesananMengembalikanStok(t *testing.T) {
	svc, products, _, _ := newOrderFixture()
	ctx := context.Background()

	order, err := svc.CreateOnlineOrder(ctx, "T1", OnlineOrderInput{
		Items: []CartItem{{ProductID: "P1", Qty: 3}}, CustomerName: "Siti",
	})
	if err != nil {
		t.Fatalf("persiapan gagal: %v", err)
	}
	if products.items[0].Stock != 7 {
		t.Fatalf("stok awal setelah pesanan = %d, ingin 7", products.items[0].Stock)
	}

	if _, err := svc.UpdateStatus(ctx, "T1", order.ID, "batal"); err != nil {
		t.Fatalf("pembatalan gagal: %v", err)
	}
	if products.items[0].Stock != 10 {
		t.Errorf("stok setelah pembatalan = %d, ingin kembali 10", products.items[0].Stock)
	}

	if _, err := svc.UpdateStatus(ctx, "T1", order.ID, "diproses"); err == nil {
		t.Error("pesanan batal tidak boleh diubah lagi")
	}
}

func TestPesananTerbayarTidakBisaDibatalkan(t *testing.T) {
	svc, _, _, _ := newOrderFixture()
	ctx := context.Background()

	result, err := svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, PaymentMethod: domain.PaymentTunai,
	})
	if err != nil {
		t.Fatalf("persiapan gagal: %v", err)
	}

	_, err = svc.UpdateStatus(ctx, "T1", result.Order.ID, "batal")
	if err == nil {
		t.Fatal("pesanan lunas seharusnya tidak bisa dibatalkan")
	}
	if code := codeOf(t, err); code != "conflict" {
		t.Errorf("kode error = %q, ingin conflict", code)
	}
}

func TestSettlePesananOnline(t *testing.T) {
	svc, _, _, transactions := newOrderFixture()
	ctx := context.Background()

	order, err := svc.CreateOnlineOrder(ctx, "T1", OnlineOrderInput{
		Items: []CartItem{{ProductID: "P1", Qty: 2}}, CustomerName: "Siti",
	})
	if err != nil {
		t.Fatalf("persiapan gagal: %v", err)
	}

	result, err := svc.Settle(ctx, "T1", order.ID, "Budi", domain.PaymentQRIS, 0)
	if err != nil {
		t.Fatalf("settle gagal: %v", err)
	}
	if result.Order.TransactionID == "" {
		t.Error("pesanan terbayar harus punya ID transaksi")
	}
	if result.Order.Cashier != "Budi" {
		t.Errorf("kasir = %q, ingin Budi", result.Order.Cashier)
	}
	if len(transactions.lines) != 1 || transactions.lines[0].Total != 50000 {
		t.Errorf("baris transaksi = %+v, ingin satu baris total 50000", transactions.lines)
	}

	if _, err := svc.Settle(ctx, "T1", order.ID, "Budi", domain.PaymentTunai, 50000); err == nil {
		t.Error("pesanan yang sudah lunas tidak boleh dibayar dua kali")
	}
}

func TestListMenolakFilterTidakDikenal(t *testing.T) {
	svc, _, _, _ := newOrderFixture()
	ctx := context.Background()

	if _, err := svc.List(ctx, "T1", "ngawur", "", time.Time{}, time.Time{}); err == nil {
		t.Error("status tidak dikenal seharusnya ditolak")
	}
	if _, err := svc.List(ctx, "T1", "", "whatsapp", time.Time{}, time.Time{}); err == nil {
		t.Error("sumber tidak dikenal seharusnya ditolak")
	}
	if _, err := svc.List(ctx, "T1", "baru", "kasir", time.Time{}, time.Time{}); err != nil {
		t.Errorf("filter valid seharusnya lolos: %v", err)
	}
}
