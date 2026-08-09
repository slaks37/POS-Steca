package service

import (
	"testing"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

func at(day int, hour int) time.Time {
	return time.Date(2026, time.March, day, hour, 0, 0, 0, timex.Location())
}

func sampleLines() []domain.TransactionLine {
	return []domain.TransactionLine{
		// TRX-1: dua item pada 2 Maret (Senin).
		{Date: at(2, 10), TransactionID: "TRX-1", Item: "Kopi", Qty: 2, UnitPrice: 18000, Total: 36000, PaymentMethod: "tunai", Cashier: "Ani"},
		{Date: at(2, 10), TransactionID: "TRX-1", Item: "Roti", Qty: 1, UnitPrice: 12000, Total: 12000, PaymentMethod: "tunai", Cashier: "Ani"},
		// TRX-2: satu item pada hari yang sama, kasir berbeda.
		{Date: at(2, 15), TransactionID: "TRX-2", Item: "Kopi", Qty: 1, UnitPrice: 18000, Total: 18000, PaymentMethod: "qris", Cashier: "Budi"},
		// TRX-3: minggu berikutnya.
		{Date: at(10, 9), TransactionID: "TRX-3", Item: "Teh", Qty: 4, UnitPrice: 6000, Total: 24000, PaymentMethod: "qris", Cashier: "Ani"},
	}
}

func TestAggregateSalesHarian(t *testing.T) {
	report := aggregateSales(sampleLines(), GranularityHarian)

	if report.TotalOmzet != 90000 {
		t.Errorf("TotalOmzet = %v, ingin 90000", report.TotalOmzet)
	}
	// Tiga ID transaksi unik, meski ada empat baris.
	if report.TotalTransactions != 3 {
		t.Errorf("TotalTransactions = %d, ingin 3", report.TotalTransactions)
	}
	if report.TotalItems != 8 {
		t.Errorf("TotalItems = %d, ingin 8", report.TotalItems)
	}
	if got := report.AverageBasket; got != 30000 {
		t.Errorf("AverageBasket = %v, ingin 30000", got)
	}
	if len(report.Series) != 2 {
		t.Fatalf("jumlah titik grafik = %d, ingin 2", len(report.Series))
	}
	if report.Series[0].Omzet != 66000 || report.Series[0].Transactions != 2 {
		t.Errorf("titik pertama = %+v, ingin omzet 66000 dan 2 transaksi", report.Series[0])
	}
	if !report.Series[0].Start.Before(report.Series[1].Start) {
		t.Error("grafik harus terurut menaik berdasarkan waktu")
	}
}

func TestAggregateSalesProdukTerlaris(t *testing.T) {
	report := aggregateSales(sampleLines(), GranularityHarian)

	if len(report.TopProducts) != 3 {
		t.Fatalf("jumlah produk = %d, ingin 3", len(report.TopProducts))
	}
	// Teh terjual 4 (terbanyak), Kopi 3, Roti 1.
	if report.TopProducts[0].Name != "Teh" || report.TopProducts[0].Qty != 4 {
		t.Errorf("produk terlaris = %+v, ingin Teh qty 4", report.TopProducts[0])
	}
	if report.TopProducts[1].Name != "Kopi" || report.TopProducts[1].Omzet != 54000 {
		t.Errorf("produk kedua = %+v, ingin Kopi omzet 54000", report.TopProducts[1])
	}
}

func TestAggregateSalesMetodeDanKasir(t *testing.T) {
	report := aggregateSales(sampleLines(), GranularityHarian)

	byMethod := map[string]PaymentShare{}
	for _, p := range report.Payments {
		byMethod[p.Method] = p
	}
	if byMethod["tunai"].Omzet != 48000 || byMethod["tunai"].Transactions != 1 {
		t.Errorf("tunai = %+v, ingin omzet 48000 dan 1 transaksi", byMethod["tunai"])
	}
	if byMethod["qris"].Transactions != 2 {
		t.Errorf("qris transaksi = %d, ingin 2", byMethod["qris"].Transactions)
	}

	if report.Cashiers[0].Cashier != "Ani" || report.Cashiers[0].Omzet != 72000 {
		t.Errorf("kasir teratas = %+v, ingin Ani omzet 72000", report.Cashiers[0])
	}
}

func TestAggregateSalesMingguanDanBulanan(t *testing.T) {
	mingguan := aggregateSales(sampleLines(), GranularityMingguan)
	if len(mingguan.Series) != 2 {
		t.Errorf("mingguan: jumlah titik = %d, ingin 2", len(mingguan.Series))
	}

	bulanan := aggregateSales(sampleLines(), GranularityBulanan)
	if len(bulanan.Series) != 1 {
		t.Fatalf("bulanan: jumlah titik = %d, ingin 1", len(bulanan.Series))
	}
	if bulanan.Series[0].Omzet != 90000 {
		t.Errorf("bulanan: omzet = %v, ingin 90000", bulanan.Series[0].Omzet)
	}
	if bulanan.Series[0].Label != "Mar 2026" {
		t.Errorf("label bulanan = %q, ingin \"Mar 2026\"", bulanan.Series[0].Label)
	}
}

func TestAggregateSalesTanpaData(t *testing.T) {
	report := aggregateSales(nil, GranularityHarian)
	if report.TotalOmzet != 0 || report.TotalTransactions != 0 || report.AverageBasket != 0 {
		t.Errorf("laporan kosong harus bernilai nol, dapat %+v", report)
	}
	// Slice kosong (bukan nil) agar JSON mengirim [] alih-alih null.
	if report.Series == nil || report.TopProducts == nil || report.Payments == nil || report.Cashiers == nil {
		t.Error("slice laporan tidak boleh nil")
	}
}

func TestGroupTransactions(t *testing.T) {
	grouped := groupTransactions(sampleLines(), 0)

	if len(grouped) != 3 {
		t.Fatalf("jumlah transaksi = %d, ingin 3", len(grouped))
	}
	// Terbaru lebih dulu: TRX-3 (10 Maret).
	if grouped[0].ID != "TRX-3" {
		t.Errorf("transaksi pertama = %q, ingin TRX-3", grouped[0].ID)
	}
	var trx1 domain.Transaction
	for _, g := range grouped {
		if g.ID == "TRX-1" {
			trx1 = g
		}
	}
	if len(trx1.Items) != 2 || trx1.Total != 48000 {
		t.Errorf("TRX-1 = %+v, ingin 2 item dengan total 48000", trx1)
	}

	if limited := groupTransactions(sampleLines(), 2); len(limited) != 2 {
		t.Errorf("limit tidak diterapkan, dapat %d transaksi", len(limited))
	}
}

func TestFillDailySeriesMengisiHariKosong(t *testing.T) {
	start := at(2, 0)
	series := []SeriesPoint{{Label: "2 Mar", Start: start, Omzet: 66000, Transactions: 2}}

	filled := fillDailySeries(series, start, 5)
	if len(filled) != 5 {
		t.Fatalf("jumlah titik = %d, ingin 5", len(filled))
	}
	if filled[0].Omzet != 66000 {
		t.Errorf("hari pertama = %v, ingin 66000", filled[0].Omzet)
	}
	for i := 1; i < 5; i++ {
		if filled[i].Omzet != 0 {
			t.Errorf("hari ke-%d harus nol, dapat %v", i, filled[i].Omzet)
		}
		if filled[i].Label == "" {
			t.Errorf("hari ke-%d harus tetap punya label", i)
		}
	}
}

func TestParseGranularity(t *testing.T) {
	cases := map[string]Granularity{
		"":         GranularityHarian,
		"harian":   GranularityHarian,
		"Mingguan": GranularityMingguan,
		"monthly":  GranularityBulanan,
		"ngawur":   GranularityHarian,
	}
	for input, want := range cases {
		if got := ParseGranularity(input); got != want {
			t.Errorf("ParseGranularity(%q) = %q, ingin %q", input, got, want)
		}
	}
}
