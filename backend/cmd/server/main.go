// Command server menjalankan REST API POS Steca.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/config"
	"github.com/slaks37/pos-steca/backend/internal/crypto"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/googleapi"
	"github.com/slaks37/pos-steca/backend/internal/handler"
	"github.com/slaks37/pos-steca/backend/internal/repository/gsheets"
	"github.com/slaks37/pos-steca/backend/internal/repository/memory"
	"github.com/slaks37/pos-steca/backend/internal/repository/postgres"
	"github.com/slaks37/pos-steca/backend/internal/repository/tenantstore"
	"github.com/slaks37/pos-steca/backend/internal/service"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if tz := os.Getenv("APP_TIMEZONE"); tz != "" && !timex.SetLocationFromEnvValue(tz) {
		slog.Warn("APP_TIMEZONE tidak dikenali, memakai WIB (UTC+7)", "nilai", tz)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("konfigurasi tidak valid", "error", err)
		os.Exit(1)
	}

	deps, cleanup, err := buildDeps(context.Background(), cfg)
	if err != nil {
		slog.Error("gagal menyiapkan aplikasi", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.NewRouter(*deps),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("POS Steca berjalan",
			"port", cfg.Port,
			"datastore", string(cfg.Datastore),
			"frontend", cfg.FrontendURL,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server berhenti", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("mematikan server...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("gagal mematikan server dengan rapi", "error", err)
	}
}

// buildDeps merakit repository dan service sesuai mode datastore yang dipilih.
// Fungsi cleanup yang dikembalikan menutup sumber daya seperti pool database.
func buildDeps(ctx context.Context, cfg *config.Config) (*handler.Deps, func(), error) {
	var (
		tenants      domain.TenantStore
		provisioner  domain.Provisioner
		products     domain.ProductRepository
		transactions domain.TransactionRepository
		orders       domain.OrderRepository
		employees    domain.EmployeeRepository
		customers    domain.CustomerRepository
		tables       domain.TableRepository
		storage      domain.FileStorage
		oauth        *googleapi.OAuthManager
		memStore     *memory.Store
		cleanup      = func() {}
	)

	switch cfg.Datastore {
	case config.DatastorePostgres:
		sealer, err := crypto.NewSealer(cfg.EncryptionKey)
		if err != nil {
			return nil, cleanup, err
		}
		pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, cleanup, err
		}
		cleanup = pool.Close

		if cfg.RunMigrations {
			if err := postgres.Migrate(ctx, pool); err != nil {
				pool.Close()
				return nil, func() {}, err
			}
		}

		store := postgres.NewTenantStore(pool, sealer)
		tenants = store
		products = postgres.NewProductRepository(pool)
		transactions = postgres.NewTransactionRepository(pool)
		orders = postgres.NewOrderRepository(pool)
		employees = postgres.NewEmployeeRepository(pool)
		customers = postgres.NewCustomerRepository(pool)
		tables = postgres.NewTableRepository(pool)

		// Data sudah pindah ke PostgreSQL, tetapi gambar produk tetap
		// disimpan di folder Drive milik pemilik akun sehingga onboarding
		// hanya perlu menyiapkan foldernya.
		oauth = googleapi.NewOAuthManager(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
		provider := gsheets.NewProvider(oauth, store)
		provisioner = gsheets.NewDriveFolderProvisioner(provider)
		storage = gsheets.NewFileStorage(provider)

	case config.DatastoreGoogle:
		sealer, err := crypto.NewSealer(cfg.EncryptionKey)
		if err != nil {
			return nil, cleanup, err
		}
		store, err := tenantstore.NewFileStore(cfg.DataDir, sealer)
		if err != nil {
			return nil, cleanup, err
		}
		oauth = googleapi.NewOAuthManager(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
		provider := gsheets.NewProvider(oauth, store)

		tenants = store
		provisioner = provider
		products = gsheets.NewProductRepository(provider, cfg.SheetsCacheTTL)
		transactions = gsheets.NewTransactionRepository(provider)
		orders = gsheets.NewOrderRepository(provider)
		employees = gsheets.NewEmployeeRepository(provider)
		customers = gsheets.NewCustomerRepository(provider)
		tables = gsheets.NewTableRepository(provider)
		storage = gsheets.NewFileStorage(provider)

	case config.DatastoreMemory:
		memStore = memory.NewStore("http://localhost:" + cfg.Port + "/api/v1/files")
		tenants = memStore
		provisioner = memStore
		products = memory.NewProductRepo(memStore)
		transactions = memory.NewTransactionRepo(memStore)
		orders = memory.NewOrderRepo(memStore)
		employees = memory.NewEmployeeRepo(memStore)
		customers = memory.NewCustomerRepo(memStore)
		tables = memory.NewTableRepo(memStore)
		storage = memory.NewStorage(memStore)
	}

	var googleAuth service.GoogleAuthenticator
	if oauth != nil {
		googleAuth = oauth
	}

	authService := service.NewAuthService(tenants, employees, provisioner, googleAuth, cfg.JWTSecret, cfg.JWTTTL)
	productService := service.NewProductService(products, storage)
	customerService := service.NewCustomerService(customers, orders)
	tableService := service.NewTableService(tables, orders)
	orderService := service.NewOrderService(products, orders, transactions, customerService, tableService)
	reportService := service.NewReportService(transactions, products, orders)
	employeeService := service.NewEmployeeService(employees, tenants)

	var demoService *service.DemoService
	if cfg.Datastore == config.DatastoreMemory && !cfg.IsProduction() {
		demoService = service.NewDemoService(tenants, employees, products, tables, provisioner, authService)
	}

	return &handler.Deps{
		Config:      cfg,
		Auth:        authService,
		Demo:        demoService,
		Products:    productService,
		Orders:      orderService,
		Reports:     reportService,
		Employees:   employeeService,
		Customers:   customerService,
		Tables:      tableService,
		Tenants:     tenants,
		MemoryStore: memStore,
	}, cleanup, nil
}
