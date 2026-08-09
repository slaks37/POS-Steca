package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/config"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/repository/memory"
	"github.com/slaks37/pos-steca/backend/internal/service"
)

// newTestServer merakit router lengkap di atas datastore in-memory, sehingga
// alur HTTP (autentikasi, RBAC, validasi) bisa diuji tanpa Google API.
func newTestServer(t *testing.T) (*gin.Engine, *service.AuthService, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	store := memory.NewStore("http://localhost:8080/api/v1/files")
	employees := memory.NewEmployeeRepo(store)
	products := memory.NewProductRepo(store)
	orders := memory.NewOrderRepo(store)
	transactions := memory.NewTransactionRepo(store)
	storage := memory.NewStorage(store)

	auth := service.NewAuthService(store, employees, store, nil, "rahasia-uji", time.Hour)
	demo := service.NewDemoService(store, employees, products, store, auth)

	cfg := &config.Config{
		Port:        "8080",
		Env:         "test",
		Datastore:   config.DatastoreMemory,
		FrontendURL: "http://localhost:5173",
		CORSOrigins: []string{"http://localhost:5173"},
	}

	router := NewRouter(Deps{
		Config:      cfg,
		Auth:        auth,
		Demo:        demo,
		Products:    service.NewProductService(products, storage),
		Orders:      service.NewOrderService(products, orders, transactions),
		Reports:     service.NewReportService(transactions, products, orders),
		Employees:   service.NewEmployeeService(employees, store),
		Tenants:     store,
		MemoryStore: store,
	})

	// Mode demo menyiapkan tenant beserta produk contoh.
	body := doJSON(t, router, http.MethodPost, "/api/v1/auth/demo/login", "", map[string]any{})
	if body.Code != http.StatusOK {
		t.Fatalf("demo login gagal: %d %s", body.Code, body.Body.String())
	}
	var demoResp struct {
		Token  string `json:"token"`
		Tenant struct {
			Code string `json:"code"`
		} `json:"tenant"`
	}
	decode(t, body, &demoResp)
	return router, auth, demoResp.Token
}

func doJSON(t *testing.T, router *gin.Engine, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("membaca respons %s: %v", rec.Body.String(), err)
	}
}

func TestHealthEndpoint(t *testing.T) {
	router, _, _ := newTestServer(t)
	rec := doJSON(t, router, http.MethodGet, "/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200", rec.Code)
	}
}

func TestEndpointTerlindungiMenolakTanpaToken(t *testing.T) {
	router, _, _ := newTestServer(t)

	paths := []string{"/api/v1/auth/me", "/api/v1/products", "/api/v1/orders", "/api/v1/dashboard"}
	for _, path := range paths {
		rec := doJSON(t, router, http.MethodGet, path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s tanpa token = %d, ingin 401", path, rec.Code)
		}
	}

	rec := doJSON(t, router, http.MethodGet, "/api/v1/products", "token-palsu", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token palsu = %d, ingin 401", rec.Code)
	}
}

func TestOwnerBisaMengaksesSeluruhModul(t *testing.T) {
	router, _, token := newTestServer(t)

	for _, path := range []string{"/api/v1/dashboard", "/api/v1/reports/sales", "/api/v1/employees", "/api/v1/products"} {
		rec := doJSON(t, router, http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("owner GET %s = %d (%s), ingin 200", path, rec.Code, rec.Body.String())
		}
	}
}

func TestKasirTerbatasPadaModulOperasional(t *testing.T) {
	router, auth, ownerToken := newTestServer(t)

	// Owner membuat akun kasir, lalu kasir login memakai PIN tersebut.
	var meResp struct {
		Tenant struct {
			Code string `json:"code"`
		} `json:"tenant"`
	}
	decode(t, doJSON(t, router, http.MethodGet, "/api/v1/auth/me", ownerToken, nil), &meResp)

	rec := doJSON(t, router, http.MethodPost, "/api/v1/employees", ownerToken, map[string]any{
		"name": "Ani", "email": "ani@warung.com", "role": "kasir", "pin": "998877",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("membuat kasir = %d (%s)", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/v1/auth/staff/login", "", map[string]any{
		"tenant_code": meResp.Tenant.Code, "email": "ani@warung.com", "pin": "998877",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login kasir = %d (%s)", rec.Code, rec.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
		User  struct {
			Role domain.Role `json:"role"`
		} `json:"user"`
	}
	decode(t, rec, &loginResp)
	if loginResp.User.Role != domain.RoleKasir {
		t.Fatalf("role = %q, ingin kasir", loginResp.User.Role)
	}
	if _, err := auth.ParseToken(loginResp.Token); err != nil {
		t.Fatalf("token kasir tidak valid: %v", err)
	}

	// Boleh: kasir, pesanan, katalog produk.
	for _, path := range []string{"/api/v1/products", "/api/v1/orders"} {
		if got := doJSON(t, router, http.MethodGet, path, loginResp.Token, nil).Code; got != http.StatusOK {
			t.Errorf("kasir GET %s = %d, ingin 200", path, got)
		}
	}
	// Ditolak: laporan, dashboard, karyawan, ubah produk.
	for _, path := range []string{"/api/v1/dashboard", "/api/v1/reports/sales", "/api/v1/employees"} {
		if got := doJSON(t, router, http.MethodGet, path, loginResp.Token, nil).Code; got != http.StatusForbidden {
			t.Errorf("kasir GET %s = %d, ingin 403", path, got)
		}
	}
	created := doJSON(t, router, http.MethodPost, "/api/v1/products", loginResp.Token, map[string]any{
		"name": "Produk Nakal", "price": 1000,
	})
	if created.Code != http.StatusForbidden {
		t.Errorf("kasir membuat produk = %d, ingin 403", created.Code)
	}
}

func TestAlurCheckoutDanPesananLewatHTTP(t *testing.T) {
	router, _, token := newTestServer(t)

	var productsResp struct {
		Data []domain.Product `json:"data"`
	}
	decode(t, doJSON(t, router, http.MethodGet, "/api/v1/products", token, nil), &productsResp)
	if len(productsResp.Data) == 0 {
		t.Fatal("katalog demo kosong")
	}
	item := productsResp.Data[0]

	rec := doJSON(t, router, http.MethodPost, "/api/v1/checkout", token, map[string]any{
		"items":          []map[string]any{{"product_id": item.ID, "qty": 2}},
		"payment_method": "tunai",
		"amount_paid":    item.Price * 3,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("checkout = %d (%s)", rec.Code, rec.Body.String())
	}
	var checkout struct {
		Data service.CheckoutResult `json:"data"`
	}
	decode(t, rec, &checkout)
	if checkout.Data.Receipt == nil || checkout.Data.Receipt.Total != item.Price*2 {
		t.Fatalf("struk = %+v", checkout.Data.Receipt)
	}

	// Pesanan hasil checkout muncul pada papan dengan status "baru".
	var orders struct {
		Data []domain.Order `json:"data"`
	}
	decode(t, doJSON(t, router, http.MethodGet, "/api/v1/orders?status=baru", token, nil), &orders)
	if len(orders.Data) != 1 {
		t.Fatalf("pesanan status baru = %d, ingin 1", len(orders.Data))
	}

	// Pindahkan ke "diproses".
	rec = doJSON(t, router, http.MethodPatch, "/api/v1/orders/"+orders.Data[0].ID+"/status", token,
		map[string]any{"status": "diproses"})
	if rec.Code != http.StatusOK {
		t.Fatalf("ubah status = %d (%s)", rec.Code, rec.Body.String())
	}

	// Laporan harus mencatat omzetnya.
	var report struct {
		Data service.SalesReport `json:"data"`
	}
	decode(t, doJSON(t, router, http.MethodGet, "/api/v1/reports/sales", token, nil), &report)
	if report.Data.TotalOmzet != item.Price*2 || report.Data.TotalTransactions != 1 {
		t.Errorf("laporan = omzet %v / %d transaksi, ingin %v / 1",
			report.Data.TotalOmzet, report.Data.TotalTransactions, item.Price*2)
	}
}

func TestKanalPesananOnlinePublik(t *testing.T) {
	router, _, token := newTestServer(t)

	var me struct {
		Tenant struct {
			Code string `json:"code"`
		} `json:"tenant"`
	}
	decode(t, doJSON(t, router, http.MethodGet, "/api/v1/auth/me", token, nil), &me)

	var menu struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Business struct {
			Name string `json:"name"`
		} `json:"business"`
	}
	rec := doJSON(t, router, http.MethodGet, "/api/v1/public/"+me.Tenant.Code+"/menu", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("menu publik = %d (%s)", rec.Code, rec.Body.String())
	}
	decode(t, rec, &menu)
	if len(menu.Data) == 0 || menu.Business.Name == "" {
		t.Fatalf("menu publik kosong: %+v", menu)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/v1/public/"+me.Tenant.Code+"/orders", "", map[string]any{
		"items":         []map[string]any{{"product_id": menu.Data[0].ID, "qty": 1}},
		"customer_name": "Siti",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("pesanan online = %d (%s)", rec.Code, rec.Body.String())
	}

	// Pesanan online muncul di papan pesanan kasir.
	var orders struct {
		Data []domain.Order `json:"data"`
	}
	decode(t, doJSON(t, router, http.MethodGet, "/api/v1/orders?source=online", token, nil), &orders)
	if len(orders.Data) != 1 || orders.Data[0].TransactionID != "" {
		t.Errorf("pesanan online = %+v, ingin satu pesanan belum terbayar", orders.Data)
	}

	// Kode bisnis yang tidak dikenal harus 404.
	if got := doJSON(t, router, http.MethodGet, "/api/v1/public/STC-NGAWUR/menu", "", nil).Code; got != http.StatusNotFound {
		t.Errorf("kode bisnis asing = %d, ingin 404", got)
	}
}

func TestPayloadTidakValidMenghasilkan400(t *testing.T) {
	router, _, token := newTestServer(t)

	rec := doJSON(t, router, http.MethodPost, "/api/v1/checkout", token, map[string]any{
		"items": []map[string]any{}, "payment_method": "tunai",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("keranjang kosong = %d, ingin 400", rec.Code)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/v1/reports/sales?from=bukan-tanggal", token, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("tanggal tidak valid = %d, ingin 400", rec.Code)
	}

	if got := doJSON(t, router, http.MethodGet, "/api/v1/tidak-ada", token, nil).Code; got != http.StatusNotFound {
		t.Errorf("endpoint asing = %d, ingin 404", got)
	}
}

func TestCORSHanyaUntukOriginTerdaftar(t *testing.T) {
	router, _, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/products", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("origin terdaftar = %q, ingin dizinkan", got)
	}

	req = httptest.NewRequest(http.MethodOptions, "/api/v1/products", nil)
	req.Header.Set("Origin", "https://situs-jahat.example")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("origin asing = %q, ingin kosong", got)
	}
}
