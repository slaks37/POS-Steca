package service

import (
	"context"
	"testing"

	"github.com/slaks37/pos-steca/backend/internal/domain"
)

func TestCreateTableValidasiDanDuplikat(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	tb, err := f.tableSvc.Create(ctx, "T1", TableInput{Name: "Meja 7", Capacity: 4, Area: "Indoor"})
	if err != nil {
		t.Fatalf("membuat meja: %v", err)
	}
	if tb.Status != domain.TableStatusKosong {
		t.Errorf("meja baru = %q, ingin kosong", tb.Status)
	}

	if _, err := f.tableSvc.Create(ctx, "T1", TableInput{Name: "meja 7"}); err == nil {
		t.Error("nama meja ganda seharusnya ditolak")
	}
	if _, err := f.tableSvc.Create(ctx, "T1", TableInput{Name: "  "}); err == nil {
		t.Error("nama meja kosong seharusnya ditolak")
	}
	if _, err := f.tableSvc.Create(ctx, "T1", TableInput{Name: "Meja 8", Capacity: -2}); err == nil {
		t.Error("kapasitas negatif seharusnya ditolak")
	}
}

func TestListTableUrutanNatural(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	for _, name := range []string{"Meja 10", "Meja 2"} {
		if _, err := f.tableSvc.Create(ctx, "T1", TableInput{Name: name}); err != nil {
			t.Fatalf("membuat %s: %v", name, err)
		}
	}
	items, err := f.tableSvc.List(ctx, "T1")
	if err != nil {
		t.Fatalf("list gagal: %v", err)
	}
	// "Meja 2" harus muncul sebelum "Meja 10", bukan urutan alfabet.
	var names []string
	for _, tb := range items {
		names = append(names, tb.Name)
	}
	want := []string{"Meja 1", "Meja 2", "Meja 10"}
	for i, w := range want {
		if i >= len(names) || names[i] != w {
			t.Fatalf("urutan meja = %v, ingin %v", names, want)
		}
	}
}

func TestCheckoutMengisiMeja(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	result, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
		TableID:       "T-1",
	})
	if err != nil {
		t.Fatalf("checkout gagal: %v", err)
	}
	if result.Order.TableID != "T-1" || result.Order.TableNo != "Meja 1" {
		t.Errorf("pesanan = meja %q/%q, ingin T-1/Meja 1", result.Order.TableID, result.Order.TableNo)
	}

	table, _ := f.tableSvc.Get(ctx, "T1", "T-1")
	if table.Status != domain.TableStatusTerisi {
		t.Errorf("status meja = %q, ingin terisi", table.Status)
	}
	if table.ActiveOrderID != result.Order.ID {
		t.Errorf("pesanan aktif meja = %q, ingin %q", table.ActiveOrderID, result.Order.ID)
	}
}

func TestCheckoutMenolakMejaTidakDikenal(t *testing.T) {
	f := newOrderFixture()

	_, err := f.svc.Checkout(context.Background(), "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
		TableID:       "T-NGAWUR",
	})
	if err == nil {
		t.Fatal("meja tidak dikenal seharusnya ditolak")
	}
	if code := codeOf(t, err); code != "not_found" {
		t.Errorf("kode error = %q, ingin not_found", code)
	}
	// Validasi terjadi sebelum transaksi ditulis.
	if len(f.transactions.lines) != 0 {
		t.Error("tidak boleh ada transaksi tercatat saat meja tidak valid")
	}
}

func TestPesananSelesaiMembebaskanMeja(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	result, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items:         []CartItem{{ProductID: "P1", Qty: 1}},
		PaymentMethod: domain.PaymentTunai,
		TableID:       "T-1",
	})
	if err != nil {
		t.Fatalf("checkout gagal: %v", err)
	}

	if _, err := f.svc.UpdateStatus(ctx, "T1", result.Order.ID, "selesai"); err != nil {
		t.Fatalf("selesaikan pesanan: %v", err)
	}
	table, _ := f.tableSvc.Get(ctx, "T1", "T-1")
	if table.Status != domain.TableStatusDibersihkan {
		t.Errorf("status meja = %q, ingin dibersihkan", table.Status)
	}
	if table.ActiveOrderID != "" {
		t.Errorf("pesanan aktif = %q, ingin kosong", table.ActiveOrderID)
	}

	// Setelah dirapikan, kasir menandai meja kosong kembali.
	if _, err := f.tableSvc.UpdateStatus(ctx, "T1", "T-1", "kosong"); err != nil {
		t.Fatalf("ubah status meja: %v", err)
	}
	table, _ = f.tableSvc.Get(ctx, "T1", "T-1")
	if table.Status != domain.TableStatusKosong {
		t.Errorf("status meja = %q, ingin kosong", table.Status)
	}
}

func TestPesananBatalMembebaskanMeja(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	order, err := f.svc.CreateOnlineOrder(ctx, "T1", OnlineOrderInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, CustomerName: "Siti", TableID: "T-1",
	})
	if err != nil {
		t.Fatalf("pesanan online gagal: %v", err)
	}
	if table, _ := f.tableSvc.Get(ctx, "T1", "T-1"); table.Status != domain.TableStatusTerisi {
		t.Fatalf("meja seharusnya terisi, dapat %q", table.Status)
	}

	if _, err := f.svc.UpdateStatus(ctx, "T1", order.ID, "batal"); err != nil {
		t.Fatalf("batalkan pesanan: %v", err)
	}
	if table, _ := f.tableSvc.Get(ctx, "T1", "T-1"); table.Status != domain.TableStatusDibersihkan {
		t.Errorf("status meja = %q, ingin dibersihkan", table.Status)
	}
}

func TestReleaseTidakMenggangguPesananLain(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	pertama, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, PaymentMethod: domain.PaymentTunai, TableID: "T-1",
	})
	if err != nil {
		t.Fatalf("checkout pertama: %v", err)
	}
	// Tamu berikutnya memakai meja yang sama.
	if _, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, PaymentMethod: domain.PaymentTunai, TableID: "T-1",
	}); err != nil {
		t.Fatalf("checkout kedua: %v", err)
	}

	// Menyelesaikan pesanan lama tidak boleh membebaskan meja yang kini
	// dipakai pesanan baru.
	if _, err := f.svc.UpdateStatus(ctx, "T1", pertama.Order.ID, "selesai"); err != nil {
		t.Fatalf("selesaikan pesanan pertama: %v", err)
	}
	table, _ := f.tableSvc.Get(ctx, "T1", "T-1")
	if table.Status != domain.TableStatusTerisi {
		t.Errorf("status meja = %q, ingin tetap terisi", table.Status)
	}
}

func TestUpdateStatusMejaMenolakNilaiAsing(t *testing.T) {
	f := newOrderFixture()
	if _, err := f.tableSvc.UpdateStatus(context.Background(), "T1", "T-1", "rusak"); err == nil {
		t.Error("status meja tidak dikenal seharusnya ditolak")
	}
}

func TestHapusMejaTerisiDitolak(t *testing.T) {
	f := newOrderFixture()
	ctx := context.Background()

	if _, err := f.svc.Checkout(ctx, "T1", "Ani", CheckoutInput{
		Items: []CartItem{{ProductID: "P1", Qty: 1}}, PaymentMethod: domain.PaymentTunai, TableID: "T-1",
	}); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := f.tableSvc.Delete(ctx, "T1", "T-1")
	if err == nil {
		t.Fatal("meja terisi seharusnya tidak bisa dihapus")
	}
	if code := codeOf(t, err); code != "conflict" {
		t.Errorf("kode error = %q, ingin conflict", code)
	}

	if _, err := f.tableSvc.UpdateStatus(ctx, "T1", "T-1", "kosong"); err != nil {
		t.Fatalf("kosongkan meja: %v", err)
	}
	if err := f.tableSvc.Delete(ctx, "T1", "T-1"); err != nil {
		t.Errorf("meja kosong seharusnya bisa dihapus: %v", err)
	}
}
