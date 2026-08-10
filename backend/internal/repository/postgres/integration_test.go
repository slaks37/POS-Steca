package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/crypto"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// Uji integrasi ini menyentuh PostgreSQL sungguhan dan hanya berjalan bila
// POS_TEST_DATABASE_URL disetel, contoh:
//
//	docker compose up -d
//	POS_TEST_DATABASE_URL="postgres://pos:pos@localhost:5432/pos_steca?sslmode=disable" \
//	  go test ./internal/repository/postgres/
//
// Tanpa variabel tersebut seluruh pengujian di berkas ini dilewati sehingga
// `go test ./...` tetap hijau di mesin yang tidak punya database.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("POS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("POS_TEST_DATABASE_URL tidak disetel; melewati uji integrasi PostgreSQL")
	}

	ctx := context.Background()
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("menghubungi database uji: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("menjalankan migrasi: %v", err)
	}
	return pool
}

// seedTenant membuat tenant uji dan membersihkannya setelah tes selesai.
// Penghapusan tenant otomatis menghapus seluruh data turunannya (ON DELETE CASCADE).
func seedTenant(t *testing.T, pool *pgxpool.Pool, suffix string) string {
	t.Helper()
	ctx := context.Background()

	sealer, err := crypto.NewSealer("kunci-uji-integrasi")
	if err != nil {
		t.Fatalf("membuat sealer: %v", err)
	}
	store := NewTenantStore(pool, sealer)

	id := "tnt_" + suffix + "_" + time.Now().Format("150405.000000")
	tenant := &domain.Tenant{
		ID:           id,
		Code:         "STC-" + suffix + time.Now().Format("05.000000"),
		BusinessName: "Warung " + suffix,
		OwnerEmail:   suffix + time.Now().Format("150405.000000") + "@warung.test",
		OwnerName:    "Pemilik " + suffix,
		RefreshToken: "refresh-" + suffix,
	}
	if err := store.Create(ctx, tenant); err != nil {
		t.Fatalf("membuat tenant uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id = $1", id)
	})
	return id
}

func TestIntegrationMigrasiIdempoten(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Migrate dipanggil sekali oleh testPool; panggilan kedua tidak boleh
	// menjalankan ulang skrip yang sama.
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrasi kedua gagal: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations WHERE version = 1").Scan(&count); err != nil {
		t.Fatalf("membaca schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("versi 1 tercatat %d kali, ingin tepat 1", count)
	}
}

func TestIntegrationTenantStoreEnkripsiToken(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	sealer, _ := crypto.NewSealer("kunci-uji-integrasi")
	store := NewTenantStore(pool, sealer)
	id := seedTenant(t, pool, "enc")

	got, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.RefreshToken != "refresh-enc" {
		t.Errorf("refresh token = %q, ingin terdekripsi utuh", got.RefreshToken)
	}

	// Kolom di database harus berisi ciphertext, bukan token mentah.
	var stored string
	if err := pool.QueryRow(ctx, "SELECT refresh_token_enc FROM tenants WHERE id = $1", id).Scan(&stored); err != nil {
		t.Fatalf("membaca kolom terenkripsi: %v", err)
	}
	if stored == "refresh-enc" || stored == "" {
		t.Errorf("kolom refresh_token_enc = %q, seharusnya ciphertext", stored)
	}

	// Update tanpa token tidak boleh menghapus token lama.
	got.RefreshToken = ""
	got.BusinessName = "Warung Ganti Nama"
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID setelah update: %v", err)
	}
	if after.RefreshToken != "refresh-enc" {
		t.Errorf("refresh token hilang setelah update: %q", after.RefreshToken)
	}
	if after.BusinessName != "Warung Ganti Nama" {
		t.Errorf("nama bisnis = %q", after.BusinessName)
	}
}

func TestIntegrationTenantPencarianTanpaMemperhatikanHuruf(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	sealer, _ := crypto.NewSealer("kunci-uji-integrasi")
	store := NewTenantStore(pool, sealer)

	id := seedTenant(t, pool, "cari")
	tenant, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if _, err := store.GetByCode(ctx, lower(tenant.Code)); err != nil {
		t.Errorf("pencarian kode huruf kecil gagal: %v", err)
	}
	if _, err := store.GetByOwnerEmail(ctx, upper(tenant.OwnerEmail)); err != nil {
		t.Errorf("pencarian email huruf besar gagal: %v", err)
	}
	if _, err := store.GetByCode(ctx, "STC-TIDAKADA"); !isNotFoundErr(err) {
		t.Errorf("kode asing = %v, ingin not_found", err)
	}
}

func TestIntegrationProdukCRUDdanStok(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenantID := seedTenant(t, pool, "prod")
	repo := NewProductRepository(pool)

	p := &domain.Product{ID: "PRD-1", Name: "Nasi Goreng", Category: "Makanan",
		Price: 25000, Stock: 10, SKU: "MKN-001"}
	if err := repo.Create(ctx, tenantID, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, tenantID, "PRD-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Price != 25000 || got.Stock != 10 || got.Name != "Nasi Goreng" {
		t.Errorf("produk = %+v", got)
	}

	// Harga desimal harus kembali utuh lewat cast ::float8.
	got.Price = 12500.75
	got.Stock = 7
	if err := repo.Update(ctx, tenantID, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _ := repo.Get(ctx, tenantID, "PRD-1")
	if after.Price != 12500.75 || after.Stock != 7 {
		t.Errorf("setelah update = harga %v / stok %d", after.Price, after.Stock)
	}

	// SKU ganda ditolak indeks unik per tenant.
	dup := &domain.Product{ID: "PRD-2", Name: "Lainnya", SKU: "mkn-001"}
	if err := repo.Create(ctx, tenantID, dup); !isConflict(err) {
		t.Errorf("SKU ganda = %v, ingin conflict", err)
	}

	// AdjustStock mengurangi stok dan tidak pernah membuatnya negatif.
	if err := repo.AdjustStock(ctx, tenantID, map[string]int{"PRD-1": -3}); err != nil {
		t.Fatalf("AdjustStock: %v", err)
	}
	after, _ = repo.Get(ctx, tenantID, "PRD-1")
	if after.Stock != 4 {
		t.Errorf("stok setelah pengurangan = %d, ingin 4", after.Stock)
	}
	if err := repo.AdjustStock(ctx, tenantID, map[string]int{"PRD-1": -99}); err != nil {
		t.Fatalf("AdjustStock berlebih: %v", err)
	}
	after, _ = repo.Get(ctx, tenantID, "PRD-1")
	if after.Stock != 0 {
		t.Errorf("stok = %d, ingin dijaga di 0", after.Stock)
	}

	if err := repo.Delete(ctx, tenantID, "PRD-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, tenantID, "PRD-1"); !isNotFoundErr(err) {
		t.Errorf("produk terhapus = %v, ingin not_found", err)
	}
	if err := repo.Delete(ctx, tenantID, "PRD-1"); !isNotFoundErr(err) {
		t.Errorf("hapus dua kali = %v, ingin not_found", err)
	}
}

// TestIntegrationIsolasiAntarTenant adalah penjaga utama kebocoran data:
// dua tenant memakai ID entitas yang persis sama, dan tidak boleh saling
// melihat maupun saling mengubah.
func TestIntegrationIsolasiAntarTenant(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tenantA := seedTenant(t, pool, "isoa")
	tenantB := seedTenant(t, pool, "isob")

	products := NewProductRepository(pool)
	customers := NewCustomerRepository(pool)
	tables := NewTableRepository(pool)
	orders := NewOrderRepository(pool)
	employees := NewEmployeeRepository(pool)
	transactions := NewTransactionRepository(pool)

	// ID yang sama persis di dua tenant berbeda.
	const sharedID = "SAMA-1"
	if err := products.Create(ctx, tenantA, &domain.Product{ID: sharedID, Name: "Punya A", Price: 1000, Stock: 5}); err != nil {
		t.Fatalf("produk A: %v", err)
	}
	if err := products.Create(ctx, tenantB, &domain.Product{ID: sharedID, Name: "Punya B", Price: 2000, Stock: 9}); err != nil {
		t.Fatalf("produk B: %v", err)
	}

	a, err := products.Get(ctx, tenantA, sharedID)
	if err != nil {
		t.Fatalf("Get A: %v", err)
	}
	if a.Name != "Punya A" || a.Price != 1000 {
		t.Errorf("tenant A membaca data tenant lain: %+v", a)
	}

	// Update dari tenant A tidak boleh menyentuh baris tenant B.
	a.Name = "Diubah A"
	if err := products.Update(ctx, tenantA, a); err != nil {
		t.Fatalf("Update A: %v", err)
	}
	b, _ := products.Get(ctx, tenantB, sharedID)
	if b.Name != "Punya B" {
		t.Errorf("data tenant B ikut berubah: %+v", b)
	}

	// AdjustStock juga harus terkurung per tenant.
	if err := products.AdjustStock(ctx, tenantA, map[string]int{sharedID: -5}); err != nil {
		t.Fatalf("AdjustStock A: %v", err)
	}
	b, _ = products.Get(ctx, tenantB, sharedID)
	if b.Stock != 9 {
		t.Errorf("stok tenant B berubah menjadi %d, ingin tetap 9", b.Stock)
	}

	// Delete lintas tenant harus gagal, bukan menghapus milik orang lain.
	if err := products.Delete(ctx, tenantB, "PRD-TIDAKADA"); !isNotFoundErr(err) {
		t.Errorf("hapus id asing = %v, ingin not_found", err)
	}

	// Entitas lain: cukup pastikan daftar tiap tenant hanya berisi miliknya.
	if err := customers.Create(ctx, tenantA, &domain.Customer{ID: sharedID, Name: "Pelanggan A", Phone: "081211112222"}); err != nil {
		t.Fatalf("pelanggan A: %v", err)
	}
	if err := customers.Create(ctx, tenantB, &domain.Customer{ID: sharedID, Name: "Pelanggan B", Phone: "081211112222"}); err != nil {
		t.Fatalf("pelanggan B (nomor sama beda tenant harus boleh): %v", err)
	}
	listA, _ := customers.List(ctx, tenantA)
	if len(listA) != 1 || listA[0].Name != "Pelanggan A" {
		t.Errorf("daftar pelanggan tenant A = %+v", listA)
	}
	byPhoneB, err := customers.GetByPhone(ctx, tenantB, "+62 812 1111 2222")
	if err != nil {
		t.Fatalf("GetByPhone B: %v", err)
	}
	if byPhoneB.Name != "Pelanggan B" {
		t.Errorf("pencarian nomor HP menyeberang tenant: %+v", byPhoneB)
	}

	now := timex.Now()
	if err := tables.Create(ctx, tenantA, &domain.Table{ID: sharedID, Name: "Meja 1", Capacity: 4, Status: domain.TableStatusKosong, UpdatedAt: now}); err != nil {
		t.Fatalf("meja A: %v", err)
	}
	if err := tables.Create(ctx, tenantB, &domain.Table{ID: sharedID, Name: "Meja 1", Capacity: 2, Status: domain.TableStatusTerisi, UpdatedAt: now}); err != nil {
		t.Fatalf("meja B: %v", err)
	}
	tblA, _ := tables.Get(ctx, tenantA, sharedID)
	if tblA.Capacity != 4 || tblA.Status != domain.TableStatusKosong {
		t.Errorf("meja tenant A = %+v", tblA)
	}

	if err := employees.Create(ctx, tenantA, &domain.Employee{ID: sharedID, Name: "Ani", Email: "ani@a.test", Role: domain.RoleKasir, Active: true, CreatedAt: now}); err != nil {
		t.Fatalf("karyawan A: %v", err)
	}
	if err := employees.Create(ctx, tenantB, &domain.Employee{ID: sharedID, Name: "Budi", Email: "ani@a.test", Role: domain.RoleKasir, Active: true, CreatedAt: now}); err != nil {
		t.Fatalf("karyawan B (email sama beda tenant harus boleh): %v", err)
	}
	empA, _ := employees.GetByEmail(ctx, tenantA, "ANI@A.TEST")
	if empA.Name != "Ani" {
		t.Errorf("pencarian email menyeberang tenant: %+v", empA)
	}

	orderA := &domain.Order{ID: sharedID, Code: "#A", Status: domain.OrderStatusBaru, Source: domain.OrderSourceKasir,
		Items: []domain.OrderItem{{ProductID: "P1", Name: "Item A", Qty: 1, UnitPrice: 1000}},
		Total: 1000, CreatedAt: now, UpdatedAt: now}
	orderB := &domain.Order{ID: sharedID, Code: "#B", Status: domain.OrderStatusSelesai, Source: domain.OrderSourceOnline,
		Items: []domain.OrderItem{{ProductID: "P2", Name: "Item B", Qty: 2, UnitPrice: 2000}},
		Total: 4000, CreatedAt: now, UpdatedAt: now}
	if err := orders.Create(ctx, tenantA, orderA); err != nil {
		t.Fatalf("pesanan A: %v", err)
	}
	if err := orders.Create(ctx, tenantB, orderB); err != nil {
		t.Fatalf("pesanan B: %v", err)
	}
	listOrdersA, _ := orders.List(ctx, tenantA, domain.OrderFilter{})
	if len(listOrdersA) != 1 || listOrdersA[0].Code != "#A" {
		t.Errorf("daftar pesanan tenant A = %+v", listOrdersA)
	}

	if err := transactions.Append(ctx, tenantA, []domain.TransactionLine{
		{Date: now, TransactionID: "TRX-A", Item: "Item A", Qty: 1, UnitPrice: 1000, Total: 1000, PaymentMethod: "tunai", Cashier: "Ani"},
	}); err != nil {
		t.Fatalf("transaksi A: %v", err)
	}
	if err := transactions.Append(ctx, tenantB, []domain.TransactionLine{
		{Date: now, TransactionID: "TRX-B", Item: "Item B", Qty: 2, UnitPrice: 2000, Total: 4000, PaymentMethod: "qris", Cashier: "Budi"},
	}); err != nil {
		t.Fatalf("transaksi B: %v", err)
	}
	linesA, _ := transactions.ListLines(ctx, tenantA, time.Time{}, time.Time{})
	if len(linesA) != 1 || linesA[0].TransactionID != "TRX-A" {
		t.Errorf("baris transaksi tenant A = %+v", linesA)
	}
}

func TestIntegrationOrderFilterDanJSONB(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenantID := seedTenant(t, pool, "ordr")
	repo := NewOrderRepository(pool)

	base := timex.StartOfDay(timex.Now())
	mk := func(id, code string, status domain.OrderStatus, source domain.OrderSource, at time.Time) *domain.Order {
		return &domain.Order{
			ID: id, Code: code, Status: status, Source: source,
			Items:     []domain.OrderItem{{ProductID: "P1", Name: "Kopi", Qty: 2, UnitPrice: 18000, Note: "panas"}},
			Total:     36000,
			CreatedAt: at, UpdatedAt: at,
		}
	}
	if err := repo.Create(ctx, tenantID, mk("O1", "#1", domain.OrderStatusBaru, domain.OrderSourceKasir, base)); err != nil {
		t.Fatalf("O1: %v", err)
	}
	if err := repo.Create(ctx, tenantID, mk("O2", "#2", domain.OrderStatusSelesai, domain.OrderSourceOnline, base.Add(time.Hour))); err != nil {
		t.Fatalf("O2: %v", err)
	}
	if err := repo.Create(ctx, tenantID, mk("O3", "#3", domain.OrderStatusBaru, domain.OrderSourceOnline, base.AddDate(0, 0, -3))); err != nil {
		t.Fatalf("O3: %v", err)
	}

	all, err := repo.List(ctx, tenantID, domain.OrderFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("jumlah pesanan = %d, ingin 3", len(all))
	}
	// Terbaru lebih dulu.
	if all[0].ID != "O2" {
		t.Errorf("urutan pesanan = %s dulu, ingin O2", all[0].ID)
	}
	// Item JSONB kembali utuh termasuk catatan.
	if len(all[0].Items) != 1 || all[0].Items[0].Note != "panas" {
		t.Errorf("item JSONB = %+v", all[0].Items)
	}

	baru, _ := repo.List(ctx, tenantID, domain.OrderFilter{Status: domain.OrderStatusBaru})
	if len(baru) != 2 {
		t.Errorf("filter status baru = %d pesanan, ingin 2", len(baru))
	}
	online, _ := repo.List(ctx, tenantID, domain.OrderFilter{Source: domain.OrderSourceOnline})
	if len(online) != 2 {
		t.Errorf("filter sumber online = %d pesanan, ingin 2", len(online))
	}
	hariIni, _ := repo.List(ctx, tenantID, domain.OrderFilter{From: base, To: base.AddDate(0, 0, 1)})
	if len(hariIni) != 2 {
		t.Errorf("filter rentang hari ini = %d pesanan, ingin 2", len(hariIni))
	}
	gabungan, _ := repo.List(ctx, tenantID, domain.OrderFilter{
		Status: domain.OrderStatusBaru, Source: domain.OrderSourceOnline,
	})
	if len(gabungan) != 1 || gabungan[0].ID != "O3" {
		t.Errorf("filter gabungan = %+v", gabungan)
	}
}

func TestIntegrationTransaksiRentangWaktu(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenantID := seedTenant(t, pool, "trx")
	repo := NewTransactionRepository(pool)

	base := timex.StartOfDay(timex.Now())
	lines := []domain.TransactionLine{
		{Date: base.Add(9 * time.Hour), TransactionID: "TRX-1", Item: "Kopi", Qty: 2, UnitPrice: 18000, Total: 36000, PaymentMethod: "tunai", Cashier: "Ani"},
		{Date: base.Add(10 * time.Hour), TransactionID: "TRX-1", Item: "Roti", Qty: 1, UnitPrice: 12000, Total: 12000, PaymentMethod: "tunai", Cashier: "Ani"},
		{Date: base.AddDate(0, 0, -5), TransactionID: "TRX-LAMA", Item: "Teh", Qty: 1, UnitPrice: 6000, Total: 6000, PaymentMethod: "qris", Cashier: "Budi"},
	}
	if err := repo.Append(ctx, tenantID, lines); err != nil {
		t.Fatalf("Append: %v", err)
	}

	all, err := repo.ListLines(ctx, tenantID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("ListLines: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3", len(all))
	}
	if !all[0].Date.After(all[1].Date) && !all[0].Date.Equal(all[1].Date) {
		t.Error("baris transaksi harus terurut dari yang terbaru")
	}

	hariIni, _ := repo.ListLines(ctx, tenantID, base, base.AddDate(0, 0, 1))
	if len(hariIni) != 2 {
		t.Errorf("rentang hari ini = %d baris, ingin 2", len(hariIni))
	}
	for _, l := range hariIni {
		if l.TransactionID != "TRX-1" {
			t.Errorf("baris di luar rentang ikut terbaca: %+v", l)
		}
	}
	if got := hariIni[0]; got.UnitPrice == 0 || got.Total == 0 {
		t.Errorf("nilai uang tidak terbaca: %+v", got)
	}

	// Batas atas bersifat eksklusif.
	sebelumHariIni, _ := repo.ListLines(ctx, tenantID, time.Time{}, base)
	if len(sebelumHariIni) != 1 || sebelumHariIni[0].TransactionID != "TRX-LAMA" {
		t.Errorf("rentang sebelum hari ini = %+v", sebelumHariIni)
	}

	if err := repo.Append(ctx, tenantID, nil); err != nil {
		t.Errorf("Append kosong seharusnya no-op: %v", err)
	}
}

func TestIntegrationPelangganLoyalitas(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenantID := seedTenant(t, pool, "cust")
	repo := NewCustomerRepository(pool)

	c := &domain.Customer{ID: "CST-1", Name: "Bu Sri", Phone: "0812-3456-7890", CreatedAt: timex.Now()}
	if err := repo.Create(ctx, tenantID, c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Pelanggan baru belum pernah belanja.
	got, err := repo.Get(ctx, tenantID, "CST-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.LastPurchase.IsZero() || got.Points != 0 {
		t.Errorf("pelanggan baru = %+v", got)
	}

	// Nomor HP dicocokkan setelah normalisasi.
	if _, err := repo.GetByPhone(ctx, tenantID, "+62 812 3456 7890"); err != nil {
		t.Errorf("GetByPhone dengan format berbeda gagal: %v", err)
	}
	if err := repo.Create(ctx, tenantID, &domain.Customer{ID: "CST-2", Name: "Kembar", Phone: "62812 34567890"}); !isConflict(err) {
		t.Errorf("nomor HP ganda = %v, ingin conflict", err)
	}
	// Pelanggan tanpa nomor HP boleh lebih dari satu.
	if err := repo.Create(ctx, tenantID, &domain.Customer{ID: "CST-3", Name: "Tanpa Nomor"}); err != nil {
		t.Errorf("pelanggan tanpa nomor pertama: %v", err)
	}
	if err := repo.Create(ctx, tenantID, &domain.Customer{ID: "CST-4", Name: "Tanpa Nomor Lagi"}); err != nil {
		t.Errorf("pelanggan tanpa nomor kedua seharusnya boleh: %v", err)
	}

	// Akumulasi poin tersimpan beserta waktu belanja terakhir.
	last := timex.Now().Truncate(time.Second)
	got.TotalSpent = 125000
	got.Points = 12
	got.LastPurchase = last
	if err := repo.Update(ctx, tenantID, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _ := repo.Get(ctx, tenantID, "CST-1")
	if after.Points != 12 || after.TotalSpent != 125000 {
		t.Errorf("setelah akumulasi = %+v", after)
	}
	if !after.LastPurchase.Equal(last) {
		t.Errorf("last_purchase = %v, ingin %v", after.LastPurchase, last)
	}

	// Daftar diurutkan dari total belanja terbesar.
	list, _ := repo.List(ctx, tenantID)
	if len(list) != 3 || list[0].ID != "CST-1" {
		t.Errorf("urutan daftar pelanggan = %+v", list)
	}
}

func TestIntegrationMejaDanKaryawan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenantID := seedTenant(t, pool, "misc")

	tables := NewTableRepository(pool)
	now := timex.Now()
	if err := tables.Create(ctx, tenantID, &domain.Table{
		ID: "TBL-1", Name: "Meja 1", Capacity: 4, Status: domain.TableStatusKosong, Area: "Indoor", UpdatedAt: now,
	}); err != nil {
		t.Fatalf("membuat meja: %v", err)
	}
	// Nama meja ganda per tenant ditolak.
	if err := tables.Create(ctx, tenantID, &domain.Table{
		ID: "TBL-2", Name: "meja 1", Status: domain.TableStatusKosong, UpdatedAt: now,
	}); !isConflict(err) {
		t.Errorf("nama meja ganda = %v, ingin conflict", err)
	}
	// Status di luar daftar ditolak CHECK constraint di database.
	if err := tables.Create(ctx, tenantID, &domain.Table{
		ID: "TBL-3", Name: "Meja 3", Status: domain.TableStatus("rusak"), UpdatedAt: now,
	}); err == nil {
		t.Error("status meja tidak dikenal seharusnya ditolak database")
	}

	tbl, _ := tables.Get(ctx, tenantID, "TBL-1")
	tbl.Status = domain.TableStatusTerisi
	tbl.ActiveOrderID = "ORD-9"
	tbl.UpdatedAt = timex.Now()
	if err := tables.Update(ctx, tenantID, tbl); err != nil {
		t.Fatalf("update meja: %v", err)
	}
	after, _ := tables.Get(ctx, tenantID, "TBL-1")
	if after.Status != domain.TableStatusTerisi || after.ActiveOrderID != "ORD-9" {
		t.Errorf("meja setelah update = %+v", after)
	}

	employees := NewEmployeeRepository(pool)
	if err := employees.Create(ctx, tenantID, &domain.Employee{
		ID: "EMP-1", Name: "Pemilik", Email: "owner@misc.test", Role: domain.RoleOwner, Active: true, CreatedAt: now,
	}); err != nil {
		t.Fatalf("membuat owner: %v", err)
	}
	if err := employees.Create(ctx, tenantID, &domain.Employee{
		ID: "EMP-2", Name: "Ani", Email: "ani@misc.test", Role: domain.RoleKasir, Active: true, PINHash: "hash", CreatedAt: now,
	}); err != nil {
		t.Fatalf("membuat kasir: %v", err)
	}
	// Email ganda per tenant ditolak.
	if err := employees.Create(ctx, tenantID, &domain.Employee{
		ID: "EMP-3", Name: "Ani Lain", Email: "ANI@misc.test", Role: domain.RoleKasir, Active: true, CreatedAt: now,
	}); !isConflict(err) {
		t.Errorf("email karyawan ganda = %v, ingin conflict", err)
	}
	// Role di luar daftar ditolak database.
	if err := employees.Create(ctx, tenantID, &domain.Employee{
		ID: "EMP-4", Name: "Manajer", Email: "m@misc.test", Role: domain.Role("manajer"), Active: true, CreatedAt: now,
	}); err == nil {
		t.Error("role tidak dikenal seharusnya ditolak database")
	}

	list, _ := employees.List(ctx, tenantID)
	if len(list) != 2 || list[0].Role != domain.RoleOwner {
		t.Errorf("daftar karyawan harus menempatkan owner lebih dulu: %+v", list)
	}
	if err := employees.Delete(ctx, tenantID, "EMP-2"); err != nil {
		t.Fatalf("hapus karyawan: %v", err)
	}
	if _, err := employees.GetByEmail(ctx, tenantID, "ani@misc.test"); !isNotFoundErr(err) {
		t.Errorf("karyawan terhapus = %v, ingin not_found", err)
	}
}

func TestIntegrationHapusTenantMembersihkanSeluruhData(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	sealer, _ := crypto.NewSealer("kunci-uji-integrasi")
	store := NewTenantStore(pool, sealer)
	tenantID := "tnt_cascade_" + time.Now().Format("150405.000000")
	if err := store.Create(ctx, &domain.Tenant{
		ID: tenantID, Code: "STC-CSC" + time.Now().Format("05.000000"),
		BusinessName: "Warung Cascade",
		OwnerEmail:   "cascade" + time.Now().Format("150405.000000") + "@warung.test",
		RefreshToken: "refresh",
	}); err != nil {
		t.Fatalf("membuat tenant: %v", err)
	}

	products := NewProductRepository(pool)
	if err := products.Create(ctx, tenantID, &domain.Product{ID: "P1", Name: "Kopi", Price: 1000, Stock: 1}); err != nil {
		t.Fatalf("membuat produk: %v", err)
	}
	transactions := NewTransactionRepository(pool)
	if err := transactions.Append(ctx, tenantID, []domain.TransactionLine{
		{Date: timex.Now(), TransactionID: "TRX-1", Item: "Kopi", Qty: 1, UnitPrice: 1000, Total: 1000, PaymentMethod: "tunai"},
	}); err != nil {
		t.Fatalf("membuat transaksi: %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenantID); err != nil {
		t.Fatalf("menghapus tenant: %v", err)
	}

	for _, table := range []string{"products", "transaction_lines"} {
		var count int
		if err := pool.QueryRow(ctx,
			"SELECT count(*) FROM "+table+" WHERE tenant_id = $1", tenantID).Scan(&count); err != nil {
			t.Fatalf("menghitung %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s masih menyisakan %d baris setelah tenant dihapus", table, count)
		}
	}
}

// --- helper ---

func isNotFoundErr(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == "not_found"
}

func isConflict(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == "conflict"
}

func lower(s string) string { return strings.ToLower(s) }
func upper(s string) string { return strings.ToUpper(s) }
