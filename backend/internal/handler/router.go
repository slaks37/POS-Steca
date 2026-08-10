package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/config"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/repository/memory"
	"github.com/slaks37/pos-steca/backend/internal/service"
)

// Deps adalah seluruh dependensi yang dibutuhkan router.
type Deps struct {
	Config    *config.Config
	Auth      *service.AuthService
	Demo      *service.DemoService
	Products  *service.ProductService
	Orders    *service.OrderService
	Reports   *service.ReportService
	Employees *service.EmployeeService
	Customers *service.CustomerService
	Tables    *service.TableService
	Tenants   domain.TenantStore

	// MemoryStore hanya terisi pada mode datastore memory, untuk melayani
	// gambar produk yang tersimpan di RAM.
	MemoryStore *memory.Store
}

// NewRouter merakit seluruh rute REST API POS Steca.
func NewRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(Recovery(), RequestLogger(), CORS(d.Config.CORSOrigins))
	r.MaxMultipartMemory = service.MaxImageBytes

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"datastore": string(d.Config.Datastore),
			"waktu":     time.Now().UTC(),
		})
	})

	authHandler := NewAuthHandler(d.Auth, d.Demo, d.Config.FrontendURL)
	productHandler := NewProductHandler(d.Products)
	orderHandler := NewOrderHandler(d.Orders)
	reportHandler := NewReportHandler(d.Reports, d.Auth)
	employeeHandler := NewEmployeeHandler(d.Employees)
	customerHandler := NewCustomerHandler(d.Customers)
	tableHandler := NewTableHandler(d.Tables)
	publicHandler := NewPublicHandler(d.Tenants, d.Products, d.Orders)

	api := r.Group("/api/v1")

	// --- Autentikasi (tanpa token) ---
	auth := api.Group("/auth")
	{
		auth.GET("/google/url", authHandler.GoogleURL)
		auth.GET("/google/callback", authHandler.GoogleCallback)
		auth.POST("/staff/login", RateLimit(20, time.Minute), authHandler.StaffLogin)
		if d.Demo != nil {
			auth.POST("/demo/login", authHandler.DemoLogin)
		}
	}

	// --- Kanal pemesanan online (tanpa token) ---
	public := api.Group("/public/:code", RateLimit(60, time.Minute))
	{
		public.GET("/menu", publicHandler.Menu)
		public.POST("/orders", publicHandler.CreateOrder)
	}

	if d.MemoryStore != nil {
		// Gambar produk mode demo dilayani langsung oleh backend.
		api.GET("/files/:tenant/:file", func(c *gin.Context) {
			data, contentType, ok := d.MemoryStore.ReadFile(c.Param("tenant"), c.Param("file"))
			if !ok {
				c.Status(http.StatusNotFound)
				return
			}
			c.Data(http.StatusOK, contentType, data)
		})
	}

	// --- Endpoint yang membutuhkan sesi ---
	secured := api.Group("", Authenticate(d.Auth))
	{
		secured.GET("/auth/me", authHandler.Me)
		secured.PATCH("/tenant", RequireRole(domain.RoleOwner), authHandler.UpdateTenant)

		// Katalog produk dibaca kasir, tetapi hanya owner yang boleh mengubah.
		secured.GET("/products", productHandler.List)
		secured.GET("/products/categories", productHandler.Categories)
		secured.GET("/products/:id", productHandler.Get)
		secured.POST("/products", RequireRole(domain.RoleOwner), productHandler.Create)
		secured.PUT("/products/:id", RequireRole(domain.RoleOwner), productHandler.Update)
		secured.DELETE("/products/:id", RequireRole(domain.RoleOwner), productHandler.Delete)
		secured.POST("/product-images", RequireRole(domain.RoleOwner), productHandler.UploadImage)

		// Kasir dan manajemen pesanan: owner maupun kasir boleh mengakses.
		secured.POST("/checkout", orderHandler.Checkout)
		secured.GET("/orders", orderHandler.List)
		secured.GET("/orders/:id", orderHandler.Get)
		secured.PATCH("/orders/:id/status", orderHandler.UpdateStatus)
		secured.POST("/orders/:id/settle", orderHandler.Settle)

		// Pelanggan: kasir perlu mencari dan mendaftarkan pelanggan saat
		// transaksi, tetapi hanya owner yang boleh mengubah dan menghapus.
		secured.GET("/customers", customerHandler.List)
		secured.GET("/customers/:id", customerHandler.Get)
		secured.POST("/customers", customerHandler.Create)

		// Meja: kasir melihat denah dan mengubah status, owner mengelola.
		secured.GET("/tables", tableHandler.List)
		secured.PATCH("/tables/:id/status", tableHandler.UpdateStatus)

		// Laporan, dashboard, karyawan, dan data master: khusus owner.
		owner := secured.Group("", RequireRole(domain.RoleOwner))
		{
			owner.GET("/dashboard", reportHandler.Dashboard)
			owner.GET("/reports/sales", reportHandler.Sales)
			owner.GET("/reports/transactions", reportHandler.Transactions)
			owner.GET("/employees", employeeHandler.List)
			owner.POST("/employees", employeeHandler.Create)
			owner.PUT("/employees/:id", employeeHandler.Update)
			owner.DELETE("/employees/:id", employeeHandler.Delete)
			owner.GET("/customers/:id/history", customerHandler.History)
			owner.PUT("/customers/:id", customerHandler.Update)
			owner.DELETE("/customers/:id", customerHandler.Delete)
			owner.POST("/tables", tableHandler.Create)
			owner.PUT("/tables/:id", tableHandler.Update)
			owner.DELETE("/tables/:id", tableHandler.Delete)
		}
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
			"code":    "not_found",
			"message": "endpoint tidak ditemukan",
		}})
	})

	return r
}
