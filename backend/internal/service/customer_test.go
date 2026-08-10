package service

import (
	"context"
	"testing"

	"github.com/slaks37/pos-steca/backend/internal/domain"
)

func TestPointsFor(t *testing.T) {
	cases := map[float64]int{
		0:      0,
		-5000:  0,
		9999:   0,
		10000:  1,
		25000:  2, // pembulatan ke bawah, poin hanya untuk kelipatan penuh
		150000: 15,
	}
	for amount, want := range cases {
		if got := PointsFor(amount); got != want {
			t.Errorf("PointsFor(%v) = %d, ingin %d", amount, got, want)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"0812-3456-7890":    "081234567890",
		"+62 812 3456 7890": "081234567890",
		"62812 34567890":    "081234567890",
		"812-3456-7890":     "081234567890",
		"bukan nomor":       "",
	}
	for input, want := range cases {
		if got := domain.NormalizePhone(input); got != want {
			t.Errorf("NormalizePhone(%q) = %q, ingin %q", input, got, want)
		}
	}
}

func TestCreateCustomerMenolakDuplikatNomor(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	if _, err := f.customerSvc.Create(ctx, "T1", CustomerInput{Name: "Sri", Phone: "0812-1111"}); err != nil {
		t.Fatalf("membuat pelanggan: %v", err)
	}
	// Nomor sama dengan penulisan berbeda harus terdeteksi duplikat.
	_, err := f.customerSvc.Create(ctx, "T1", CustomerInput{Name: "Sri Lain", Phone: "+62 812 1111"})
	if err == nil {
		t.Fatal("nomor HP ganda seharusnya ditolak")
	}
	if code := codeOf(t, err); code != "conflict" {
		t.Errorf("kode error = %q, ingin conflict", code)
	}

	if _, err := f.customerSvc.Create(ctx, "T1", CustomerInput{Name: "  "}); err == nil {
		t.Error("nama kosong seharusnya ditolak")
	}
}

func TestCheckoutMengakumulasiPoinLoyalitas(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	// Belanja Rp 50.000 (2 x Nasi Goreng) => 5 poin.
	result, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 2}},
		PaymentMethod: domain.PaymentTunai,
		CustomerName:  "Bu Sri",
		CustomerPhone: "081211112222",
	})
	if err != nil {
		t.Fatalf("checkout gagal: %v", err)
	}

	if len(f.customers.items) != 1 {
		t.Fatalf("pelanggan tersimpan = %d, ingin 1 (didaftarkan otomatis)", len(f.customers.items))
	}
	customer := f.customers.items[0]
	if customer.Points != 5 || customer.TotalSpent != 50000 {
		t.Errorf("pelanggan = %d poin / total %v, ingin 5 poin / 50000", customer.Points, customer.TotalSpent)
	}
	if customer.LastPurchase.IsZero() {
		t.Error("terakhir belanja harus terisi")
	}
	if result.Order.CustomerID != customer.ID {
		t.Errorf("pesanan tidak tertaut ke pelanggan: %q", result.Order.CustomerID)
	}
	if result.Receipt.PointsEarned != 5 || result.Receipt.TotalPoints != 5 {
		t.Errorf("struk = %d poin didapat / %d total", result.Receipt.PointsEarned, result.Receipt.TotalPoints)
	}

	// Belanja kedua menambah poin pada pelanggan yang sama, dikenali dari
	// nomor HP meski ditulis berbeda.
	if _, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentQRIS,
		CustomerPhone: "+62 812 1111 2222",
	}); err != nil {
		t.Fatalf("checkout kedua gagal: %v", err)
	}
	if len(f.customers.items) != 1 {
		t.Fatalf("pelanggan tersimpan = %d, ingin tetap 1", len(f.customers.items))
	}
	if got := f.customers.items[0]; got.Points != 7 || got.TotalSpent != 75000 {
		t.Errorf("setelah belanja kedua = %d poin / total %v, ingin 7 poin / 75000", got.Points, got.TotalSpent)
	}
}

func TestCheckoutAnonimTetapBoleh(t *testing.T) {
	f := newOrderFixture()

	result, err := f.svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
	})
	if err != nil {
		t.Fatalf("checkout anonim gagal: %v", err)
	}
	if len(f.customers.items) != 0 {
		t.Errorf("transaksi anonim tidak boleh membuat pelanggan, dapat %d", len(f.customers.items))
	}
	if result.Order.CustomerID != "" || result.Receipt.PointsEarned != 0 {
		t.Errorf("struk anonim = %+v", result.Receipt)
	}
}

func TestNamaTanpaNomorTidakMembuatKartuLoyalitas(t *testing.T) {
	f := newOrderFixture()

	result, err := f.svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
		CustomerName:  "Pak Budi",
	})
	if err != nil {
		t.Fatalf("checkout gagal: %v", err)
	}
	if len(f.customers.items) != 0 {
		t.Error("tanpa nomor HP tidak boleh ada kartu loyalitas baru")
	}
	if result.Order.CustomerName != "Pak Budi" {
		t.Errorf("nama pelanggan pada pesanan = %q", result.Order.CustomerName)
	}
}

func TestSettlePesananOnlineMemberiPoin(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	order, err := f.svc.CreateOnlineOrder(ctx, "T1", OnlineOrderInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 2}},
		CustomerName:  "Siti",
		CustomerPhone: "081233334444",
	})
	if err != nil {
		t.Fatalf("pesanan online gagal: %v", err)
	}
	// Poin belum diberikan sebelum pembayaran diterima.
	if f.customers.items[0].Points != 0 {
		t.Errorf("poin sebelum bayar = %d, ingin 0", f.customers.items[0].Points)
	}

	result, err := f.svc.Settle(ctx, "T1", order.ID, "Ani", domain.PaymentQRIS, 0, "")
	if err != nil {
		t.Fatalf("settle gagal: %v", err)
	}
	if f.customers.items[0].Points != 5 {
		t.Errorf("poin setelah bayar = %d, ingin 5", f.customers.items[0].Points)
	}
	if result.Receipt.PointsEarned != 5 {
		t.Errorf("struk poin = %d, ingin 5", result.Receipt.PointsEarned)
	}
}

func TestSettleBisaMenautkanPelangganLewatNomorHP(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	order, err := f.svc.CreateOnlineOrder(ctx, "T1", OnlineOrderInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, CustomerName: "Tamu",
	})
	if err != nil {
		t.Fatalf("pesanan online gagal: %v", err)
	}
	if order.CustomerID != "" {
		t.Fatal("pesanan tanpa nomor HP seharusnya belum tertaut pelanggan")
	}

	result, err := f.svc.Settle(ctx, "T1", order.ID, "Ani", domain.PaymentTunai, 25000, "0812-5555-6666")
	if err != nil {
		t.Fatalf("settle gagal: %v", err)
	}
	if result.Order.CustomerID == "" {
		t.Error("pesanan seharusnya tertaut pelanggan setelah nomor HP diisi kasir")
	}
	if len(f.customers.items) != 1 || f.customers.items[0].Points != 2 {
		t.Errorf("pelanggan = %+v, ingin 1 pelanggan dengan 2 poin", f.customers.items)
	}
}

func TestCustomerHistoryHanyaPesananMiliknya(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	if _, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
		CustomerPhone: "0812111",
		CustomerName:  "Sri",
	}); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	// Transaksi anonim tidak boleh ikut ke riwayat siapa pun.
	if _, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P2", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
	}); err != nil {
		t.Fatalf("checkout anonim: %v", err)
	}

	customerID := f.customers.items[0].ID
	history, err := f.customerSvc.History(ctx, "T1", customerID)
	if err != nil {
		t.Fatalf("history gagal: %v", err)
	}
	if len(history.Orders) != 1 {
		t.Fatalf("riwayat = %d pesanan, ingin 1", len(history.Orders))
	}
	if history.Customer.ID != customerID {
		t.Errorf("riwayat milik pelanggan %q", history.Customer.ID)
	}
}

func TestListCustomerPencarianDanUrutan(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	besar, _ := f.customerSvc.Create(ctx, "T1", CustomerInput{Name: "Pelanggan Besar", Phone: "0811000111"})
	kecil, _ := f.customerSvc.Create(ctx, "T1", CustomerInput{Name: "Pelanggan Kecil", Phone: "0822000222"})
	besar.TotalSpent = 500000
	kecil.TotalSpent = 10000
	_ = f.customers.Update(ctx, "T1", besar)
	_ = f.customers.Update(ctx, "T1", kecil)

	all, err := f.customerSvc.List(ctx, "T1", "")
	if err != nil {
		t.Fatalf("list gagal: %v", err)
	}
	if len(all) != 2 || all[0].ID != besar.ID {
		t.Errorf("urutan harus dari total belanja terbesar, dapat %+v", all)
	}

	byName, _ := f.customerSvc.List(ctx, "T1", "kecil")
	if len(byName) != 1 || byName[0].ID != kecil.ID {
		t.Errorf("pencarian nama = %+v", byName)
	}
	byPhone, _ := f.customerSvc.List(ctx, "T1", "0822")
	if len(byPhone) != 1 || byPhone[0].ID != kecil.ID {
		t.Errorf("pencarian nomor HP = %+v", byPhone)
	}
}
