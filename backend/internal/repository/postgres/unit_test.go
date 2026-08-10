package postgres

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// Pengujian pada berkas ini murni unit: tidak butuh instance PostgreSQL.
// Uji integrasi yang benar-benar menyentuh database ada di integration_test.go.

// --- Penjagaan multi-tenant ---

// tenantScopedStatements adalah seluruh perintah SQL yang menyentuh tabel
// milik tenant. Setiap tabel entitas WAJIB disaring tenant_id agar data satu
// bisnis tidak pernah terbaca bisnis lain.
var tenantScopedStatements = map[string][]string{
	"products": {
		`SELECT ` + productColumns + ` FROM products WHERE tenant_id = $1 ORDER BY lower(category), lower(name)`,
		`SELECT ` + productColumns + ` FROM products WHERE tenant_id = $1 AND id = $2`,
		`INSERT INTO products (tenant_id, id, name, category, price, stock, sku, image_url, image_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		`UPDATE products SET name = $3 WHERE tenant_id = $1 AND id = $2`,
		`DELETE FROM products WHERE tenant_id = $1 AND id = $2`,
		`UPDATE products SET stock = GREATEST(stock + $3, 0) WHERE tenant_id = $1 AND id = $2`,
	},
	"employees": {
		`SELECT ` + employeeColumns + ` FROM employees WHERE tenant_id = $1 ORDER BY lower(name)`,
		`SELECT ` + employeeColumns + ` FROM employees WHERE tenant_id = $1 AND id = $2`,
		`SELECT ` + employeeColumns + ` FROM employees WHERE tenant_id = $1 AND lower(email) = lower($2)`,
		`INSERT INTO employees (tenant_id, id, name, email, role, active, pin_hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		`UPDATE employees SET name = $3 WHERE tenant_id = $1 AND id = $2`,
		`DELETE FROM employees WHERE tenant_id = $1 AND id = $2`,
	},
	"customers": {
		`SELECT ` + customerColumns + ` FROM customers WHERE tenant_id = $1 ORDER BY total_spent DESC`,
		`SELECT ` + customerColumns + ` FROM customers WHERE tenant_id = $1 AND id = $2`,
		`SELECT ` + customerColumns + ` FROM customers WHERE tenant_id = $1 AND phone_normalized = $2`,
		`INSERT INTO customers (tenant_id, id, name, phone, phone_normalized,
			total_spent, points, last_purchase, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		`UPDATE customers SET name = $3 WHERE tenant_id = $1 AND id = $2`,
		`DELETE FROM customers WHERE tenant_id = $1 AND id = $2`,
	},
	"tables": {
		`SELECT ` + tableColumns + ` FROM tables WHERE tenant_id = $1 ORDER BY lower(area), lower(name)`,
		`SELECT ` + tableColumns + ` FROM tables WHERE tenant_id = $1 AND id = $2`,
		`INSERT INTO tables (tenant_id, id, name, capacity, status, area, active_order_id, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		`UPDATE tables SET name = $3 WHERE tenant_id = $1 AND id = $2`,
		`DELETE FROM tables WHERE tenant_id = $1 AND id = $2`,
	},
	"orders": {
		`SELECT ` + orderColumns + ` FROM orders WHERE tenant_id = $1 ORDER BY created_at DESC`,
		`SELECT ` + orderColumns + ` FROM orders WHERE tenant_id = $1 AND id = $2`,
		`INSERT INTO orders (tenant_id, id, code) VALUES ($1, $2, $3)`,
		`UPDATE orders SET code = $3 WHERE tenant_id = $1 AND id = $2`,
	},
	"transaction_lines": {
		`SELECT ` + transactionColumns + ` FROM transaction_lines WHERE tenant_id = $1`,
	},
}

// tenantPredicate mendeteksi penyaringan tenant_id pada klausa WHERE.
var tenantPredicate = regexp.MustCompile(`(?i)\btenant_id\s*=\s*\$1\b`)

func TestSeluruhQueryDisaringPerTenant(t *testing.T) {
	for table, statements := range tenantScopedStatements {
		for _, sql := range statements {
			normalized := strings.Join(strings.Fields(sql), " ")

			switch {
			case strings.HasPrefix(strings.ToUpper(normalized), "INSERT"):
				// INSERT tidak punya WHERE; yang penting tenant_id ikut ditulis
				// sebagai kolom pertama.
				if !strings.Contains(normalized, "(tenant_id,") {
					t.Errorf("[%s] INSERT tidak menyertakan tenant_id: %s", table, normalized)
				}
			default:
				if !tenantPredicate.MatchString(normalized) {
					t.Errorf("[%s] query tidak disaring tenant_id = $1: %s", table, normalized)
				}
			}
		}
	}
}

func TestKolomQueryTidakMembocorkanTenantID(t *testing.T) {
	// Daftar kolom hasil SELECT tidak perlu mengembalikan tenant_id: model
	// domain tidak memilikinya, dan pemanggil sudah tahu tenant aktif.
	for name, columns := range map[string]string{
		"products":          productColumns,
		"employees":         employeeColumns,
		"customers":         customerColumns,
		"tables":            tableColumns,
		"orders":            orderColumns,
		"transaction_lines": transactionColumns,
	} {
		if strings.Contains(columns, "tenant_id") {
			t.Errorf("daftar kolom %s tidak boleh menyertakan tenant_id: %s", name, columns)
		}
	}
}

// --- Filter pesanan ---

func TestOrderFilterClauseKosong(t *testing.T) {
	clause, args := orderFilterClause(domain.OrderFilter{})
	if clause != "" {
		t.Errorf("filter kosong seharusnya tidak menambah klausa, dapat %q", clause)
	}
	if len(args) != 0 {
		t.Errorf("filter kosong seharusnya tanpa argumen, dapat %v", args)
	}
}

func TestOrderFilterClausePenomoranArgumen(t *testing.T) {
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, timex.Location())
	to := from.AddDate(0, 1, 0)

	clause, args := orderFilterClause(domain.OrderFilter{
		Status: domain.OrderStatusBaru,
		Source: domain.OrderSourceOnline,
		From:   from,
		To:     to,
	})

	// $1 dicadangkan untuk tenant_id, jadi filter mulai dari $2.
	want := " AND status = $2 AND source = $3 AND created_at >= $4 AND created_at < $5"
	if clause != want {
		t.Errorf("klausa = %q, ingin %q", clause, want)
	}
	if len(args) != 4 {
		t.Fatalf("jumlah argumen = %d, ingin 4", len(args))
	}
	if args[0] != "baru" || args[1] != "online" {
		t.Errorf("argumen status/sumber = %v/%v", args[0], args[1])
	}
	if got, ok := args[2].(time.Time); !ok || !got.Equal(from) {
		t.Errorf("argumen from = %v, ingin %v", args[2], from)
	}
}

func TestOrderFilterClauseSebagianTerisi(t *testing.T) {
	clause, args := orderFilterClause(domain.OrderFilter{Source: domain.OrderSourceKasir})
	if clause != " AND source = $2" {
		t.Errorf("klausa = %q", clause)
	}
	if len(args) != 1 || args[0] != "kasir" {
		t.Errorf("argumen = %v", args)
	}

	from := time.Date(2026, time.January, 2, 0, 0, 0, 0, timex.Location())
	clause, args = orderFilterClause(domain.OrderFilter{From: from})
	if clause != " AND created_at >= $2" {
		t.Errorf("klausa hanya-from = %q", clause)
	}
	if len(args) != 1 {
		t.Errorf("argumen hanya-from = %v", args)
	}
}

// --- Pemetaan baris ---

// fakeRow meniru pgx.Row/pgx.Rows agar fungsi scan bisa diuji tanpa database.
type fakeRow struct {
	values []any
	err    error
}

func (f fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	if len(dest) != len(f.values) {
		return errors.New("jumlah kolom tidak cocok")
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			*target = f.values[i].(string)
		case *int:
			*target = f.values[i].(int)
		case *bool:
			*target = f.values[i].(bool)
		case *float64:
			*target = f.values[i].(float64)
		case *time.Time:
			*target = f.values[i].(time.Time)
		case **time.Time:
			if f.values[i] == nil {
				*target = nil
			} else {
				v := f.values[i].(time.Time)
				*target = &v
			}
		case *[]byte:
			*target = f.values[i].([]byte)
		case *domain.Role:
			*target = domain.Role(f.values[i].(string))
		case *domain.OrderStatus:
			*target = domain.OrderStatus(f.values[i].(string))
		case *domain.OrderSource:
			*target = domain.OrderSource(f.values[i].(string))
		case *domain.TableStatus:
			*target = domain.TableStatus(f.values[i].(string))
		default:
			return errors.New("tipe tujuan tidak didukung dalam pengujian")
		}
	}
	return nil
}

func TestScanProduct(t *testing.T) {
	p, err := scanProduct(fakeRow{values: []any{
		"PRD-1", " Kopi Susu ", "Minuman", 18000.0, 42, "MNM-003", "https://drive/x", "x",
	}})
	if err != nil {
		t.Fatalf("scanProduct: %v", err)
	}
	if p.ID != "PRD-1" || p.Price != 18000 || p.Stock != 42 {
		t.Errorf("produk = %+v", p)
	}
	if p.Name != "Kopi Susu" {
		t.Errorf("nama = %q, ingin dirapikan tanpa spasi berlebih", p.Name)
	}
}

func TestScanProductBarisTidakAda(t *testing.T) {
	_, err := scanProduct(fakeRow{err: pgx.ErrNoRows})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != "not_found" {
		t.Errorf("error = %v, ingin apperr not_found", err)
	}
}

func TestScanCustomerTanpaRiwayatBelanja(t *testing.T) {
	created := time.Date(2026, time.February, 1, 8, 0, 0, 0, time.UTC)

	c, err := scanCustomer(fakeRow{values: []any{
		"CST-1", "Bu Sri", "0812", 0.0, 0, nil, created,
	}})
	if err != nil {
		t.Fatalf("scanCustomer: %v", err)
	}
	if !c.LastPurchase.IsZero() {
		t.Errorf("last_purchase NULL harus menjadi waktu nol, dapat %v", c.LastPurchase)
	}
	if !c.CreatedAt.Equal(created) {
		t.Errorf("created_at = %v, ingin %v", c.CreatedAt, created)
	}
	// Waktu dari database dikonversi ke zona waktu aplikasi.
	if name, _ := c.CreatedAt.Zone(); name != "WIB" {
		t.Errorf("zona waktu = %q, ingin WIB", name)
	}
}

func TestScanCustomerDenganRiwayatBelanja(t *testing.T) {
	last := time.Date(2026, time.March, 2, 3, 0, 0, 0, time.UTC)
	c, err := scanCustomer(fakeRow{values: []any{
		"CST-2", "Budi", "0813", 125000.0, 12, last, last,
	}})
	if err != nil {
		t.Fatalf("scanCustomer: %v", err)
	}
	if c.Points != 12 || c.TotalSpent != 125000 {
		t.Errorf("pelanggan = %+v", c)
	}
	if !c.LastPurchase.Equal(last) {
		t.Errorf("last_purchase = %v, ingin %v", c.LastPurchase, last)
	}
}

func TestScanOrderMembacaItemJSON(t *testing.T) {
	now := time.Date(2026, time.March, 2, 5, 0, 0, 0, time.UTC)
	items := []byte(`[{"product_id":"P1","name":"Nasi Goreng","qty":2,"unit_price":25000}]`)

	o, err := scanOrder(fakeRow{values: []any{
		"ORD-1", "#0302-ABC", "TRX-1", "baru", "kasir", "CST-1", "Bu Sri",
		"TBL-1", "Meja 1", items, 50000.0, "pedas", "Ani", now, now,
	}})
	if err != nil {
		t.Fatalf("scanOrder: %v", err)
	}
	if len(o.Items) != 1 || o.Items[0].Qty != 2 || o.Items[0].Name != "Nasi Goreng" {
		t.Errorf("item pesanan = %+v", o.Items)
	}
	if o.Status != domain.OrderStatusBaru || o.Source != domain.OrderSourceKasir {
		t.Errorf("status/sumber = %v/%v", o.Status, o.Source)
	}
	if o.CustomerID != "CST-1" || o.TableID != "TBL-1" {
		t.Errorf("tautan pelanggan/meja = %q/%q", o.CustomerID, o.TableID)
	}
}

func TestScanOrderItemsKosongTetapSliceBukanNil(t *testing.T) {
	now := time.Now().UTC()
	o, err := scanOrder(fakeRow{values: []any{
		"ORD-2", "", "", "baru", "online", "", "", "", "", []byte(`[]`),
		0.0, "", "", now, now,
	}})
	if err != nil {
		t.Fatalf("scanOrder: %v", err)
	}
	if o.Items == nil {
		t.Error("items harus slice kosong agar JSON mengirim [] bukan null")
	}
}

func TestScanTableStatusTidakDikenalJadiKosong(t *testing.T) {
	tb, err := scanTable(fakeRow{values: []any{
		"TBL-1", "Meja 1", 4, "entah", "Indoor", "", time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("scanTable: %v", err)
	}
	if tb.Status != domain.TableStatusKosong {
		t.Errorf("status = %q, ingin fallback ke kosong", tb.Status)
	}
}

func TestScanEmployeeMenormalkanEmail(t *testing.T) {
	e, err := scanEmployee(fakeRow{values: []any{
		"EMP-1", "Ani", "Ani@Warung.COM", "kasir", true, "hash", time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("scanEmployee: %v", err)
	}
	if e.Email != "ani@warung.com" {
		t.Errorf("email = %q, ingin huruf kecil", e.Email)
	}
	if e.Role != domain.RoleKasir || !e.Active {
		t.Errorf("karyawan = %+v", e)
	}
}

// --- Pemetaan error driver ---

func TestWrapDBMemetakanKodePostgres(t *testing.T) {
	cases := map[string]string{
		pgUniqueViolation:     "conflict",
		pgForeignKeyViolation: "bad_request",
		pgCheckViolation:      "bad_request",
		"08006":               "internal_error", // koneksi putus
	}
	for code, wantCode := range cases {
		err := wrapDB(&pgconn.PgError{Code: code}, "produk")
		var appErr *apperr.Error
		if !errors.As(err, &appErr) {
			t.Fatalf("kode %s: error bukan apperr.Error", code)
		}
		if appErr.Code != wantCode {
			t.Errorf("kode %s dipetakan ke %q, ingin %q", code, appErr.Code, wantCode)
		}
	}

	if wrapDB(nil, "produk") != nil {
		t.Error("wrapDB(nil) harus nil")
	}
}

func TestWrapDBTidakMembocorkanDetailKeKlien(t *testing.T) {
	err := wrapDB(&pgconn.PgError{
		Code:    pgUniqueViolation,
		Message: "duplicate key value violates unique constraint \"products_tenant_sku_key\"",
		Detail:  "Key (tenant_id, lower(sku))=(tnt_rahasia, mnm-001) already exists.",
	}, "produk")

	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatal("error bukan apperr.Error")
	}
	// Pesan yang dikirim ke klien tidak boleh memuat nama constraint atau
	// nilai kolom milik tenant lain.
	if strings.Contains(appErr.Message, "tnt_rahasia") || strings.Contains(appErr.Message, "constraint") {
		t.Errorf("pesan bocor ke klien: %q", appErr.Message)
	}
	// Detail aslinya tetap tersimpan untuk log server.
	if !strings.Contains(appErr.Error(), "products_tenant_sku_key") {
		t.Errorf("detail asli hilang dari rantai error: %v", appErr.Error())
	}
}

// --- Helper waktu ---

func TestTimePtr(t *testing.T) {
	if timePtr(time.Time{}) != nil {
		t.Error("waktu nol harus menjadi NULL")
	}
	now := time.Now()
	got := timePtr(now)
	if got == nil || !got.Equal(now) {
		t.Errorf("timePtr = %v, ingin %v", got, now)
	}
	if got.Location() != time.UTC {
		t.Errorf("waktu yang ditulis harus UTC, dapat %v", got.Location())
	}
}

func TestNullableTime(t *testing.T) {
	if !nullableTime(nil).IsZero() {
		t.Error("NULL harus menjadi waktu nol")
	}
	now := time.Now().UTC()
	if got := nullableTime(&now); !got.Equal(now) {
		t.Errorf("nullableTime = %v, ingin %v", got, now)
	}
}

// --- Migrasi ---

func TestLoadMigrationsTerurutDanLengkap(t *testing.T) {
	migrations, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("tidak ada migrasi yang terembed")
	}

	for i, m := range migrations {
		if i > 0 && migrations[i-1].Version >= m.Version {
			t.Errorf("migrasi tidak terurut menaik: %d setelah %d", m.Version, migrations[i-1].Version)
		}
		if strings.TrimSpace(m.Up) == "" {
			t.Errorf("migrasi %d tidak punya isi up", m.Version)
		}
		if strings.TrimSpace(m.Down) == "" {
			t.Errorf("migrasi %d sebaiknya menyediakan down untuk rollback", m.Version)
		}
	}

	first := migrations[0]
	if first.Version != 1 || first.Name != "init" {
		t.Errorf("migrasi pertama = %d_%s, ingin 1_init", first.Version, first.Name)
	}
	for _, table := range []string{"tenants", "employees", "products", "customers", "tables", "orders", "transaction_lines"} {
		if !strings.Contains(first.Up, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Errorf("migrasi awal tidak membuat tabel %s", table)
		}
	}
}

func TestParseMigrationName(t *testing.T) {
	version, name, direction, err := parseMigrationName("0012_tambah_diskon.up.sql")
	if err != nil {
		t.Fatalf("parseMigrationName: %v", err)
	}
	if version != 12 || name != "tambah_diskon" || direction != "up" {
		t.Errorf("hasil = %d/%s/%s", version, name, direction)
	}

	for _, bad := range []string{"init.up.sql", "0001_init.sql", "abc_init.up.sql", "0000_init.up.sql"} {
		if _, _, _, err := parseMigrationName(bad); err == nil {
			t.Errorf("nama %q seharusnya ditolak", bad)
		}
	}
}
