// Package gsheets mengimplementasikan port repository domain di atas Google
// Sheets (data tabular) dan Google Drive (berkas gambar).
package gsheets

import (
	"context"
	"fmt"
	"sync"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/googleapi"
)

// Nama sheet dan susunan kolomnya. Header inilah yang akan terlihat oleh
// pemilik usaha saat membuka spreadsheet di Google Sheets.
const (
	SheetProducts     = "Products"
	SheetTransactions = "Transactions"
	SheetOrders       = "Orders"
	SheetEmployees    = "Employees"
	SheetCustomers    = "Customers"
	SheetTables       = "Tables"
)

var (
	productHeader = []string{"ID", "Nama", "Kategori", "Harga", "Stok", "SKU", "URL Gambar", "ID Gambar"}

	transactionHeader = []string{"Tanggal", "ID Transaksi", "Item", "Qty", "Harga Satuan", "Total", "Metode Pembayaran", "Kasir"}

	// Kolom "ID Pelanggan" dan "ID Meja" ditambahkan di ujung kanan agar
	// spreadsheet tenant lama tetap terbaca tanpa migrasi manual.
	orderHeader = []string{"ID Pesanan", "Kode", "ID Transaksi", "Tanggal", "Diperbarui", "Status", "Sumber",
		"Nama Pelanggan", "No Meja", "Items (JSON)", "Total", "Catatan", "Kasir", "ID Pelanggan", "ID Meja"}

	employeeHeader = []string{"ID", "Nama", "Email", "Role", "Status", "PIN Hash", "Dibuat"}

	customerHeader = []string{"ID", "Nama", "No HP", "Total Belanja", "Poin", "Terakhir Belanja", "Dibuat"}

	tableHeader = []string{"ID", "Nama Meja", "Kapasitas", "Status", "Area", "ID Pesanan Aktif", "Diperbarui"}
)

// SheetSpecs adalah struktur lengkap spreadsheet satu tenant.
func SheetSpecs() []googleapi.SheetSpec {
	return []googleapi.SheetSpec{
		{Title: SheetProducts, Header: productHeader},
		{Title: SheetTransactions, Header: transactionHeader},
		{Title: SheetOrders, Header: orderHeader},
		{Title: SheetEmployees, Header: employeeHeader},
		{Title: SheetCustomers, Header: customerHeader},
		{Title: SheetTables, Header: tableHeader},
	}
}

// Provider membuat (dan menyimpan sementara) klien Drive/Sheets per tenant.
// Setiap tenant memakai refresh token miliknya sendiri, sehingga data satu
// bisnis tidak pernah tercampur dengan bisnis lain.
type Provider struct {
	oauth   *googleapi.OAuthManager
	tenants domain.TenantStore

	mu    sync.Mutex
	cache map[string]*tenantClients
	locks map[string]*sync.Mutex
}

type tenantClients struct {
	drive         *googleapi.DriveClient
	sheets        *googleapi.SheetsClient
	spreadsheetID string
}

// NewProvider membuat provider klien Google per tenant.
func NewProvider(oauth *googleapi.OAuthManager, tenants domain.TenantStore) *Provider {
	return &Provider{
		oauth:   oauth,
		tenants: tenants,
		cache:   map[string]*tenantClients{},
		locks:   map[string]*sync.Mutex{},
	}
}

// Clients mengembalikan klien Drive dan Sheets milik tenant tertentu.
func (p *Provider) Clients(ctx context.Context, tenantID string) (*googleapi.DriveClient, *googleapi.SheetsClient, *domain.Tenant, error) {
	tenant, err := p.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return nil, nil, nil, err
	}
	if tenant.RefreshToken == "" {
		return nil, nil, nil, apperr.Unauthorized("koneksi Google untuk akun ini belum aktif, silakan hubungkan ulang")
	}

	p.mu.Lock()
	cached, ok := p.cache[tenantID]
	p.mu.Unlock()
	if ok && cached.spreadsheetID == tenant.SpreadsheetID {
		return cached.drive, cached.sheets, tenant, nil
	}

	driveSvc, sheetsSvc, err := p.oauth.NewServices(ctx, tenant.RefreshToken)
	if err != nil {
		return nil, nil, nil, apperr.Upstream("gagal menyiapkan koneksi Google").WithCause(err)
	}
	clients := &tenantClients{
		drive:         googleapi.NewDriveClient(driveSvc),
		sheets:        googleapi.NewSheetsClient(sheetsSvc, tenant.SpreadsheetID),
		spreadsheetID: tenant.SpreadsheetID,
	}

	p.mu.Lock()
	p.cache[tenantID] = clients
	p.mu.Unlock()

	return clients.drive, clients.sheets, tenant, nil
}

// Invalidate membuang klien tersimpan, dipakai setelah re-provisioning.
func (p *Provider) Invalidate(tenantID string) {
	p.mu.Lock()
	delete(p.cache, tenantID)
	p.mu.Unlock()
}

// Lock mengembalikan mutex per tenant. Google Sheets tidak punya transaksi,
// jadi operasi baca-ubah-tulis diserialkan per tenant di dalam satu proses.
func (p *Provider) Lock(tenantID string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l, ok := p.locks[tenantID]; ok {
		return l
	}
	l := &sync.Mutex{}
	p.locks[tenantID] = l
	return l
}

// DriveFolderProvisioner hanya menyiapkan folder Drive tenant, tanpa membuat
// spreadsheet. Dipakai pada mode datastore postgres: sumber kebenaran data
// sudah pindah ke database, tetapi gambar produk tetap disimpan di Drive
// milik pemilik akun. Spreadsheet baru dibuat lagi nanti ketika fitur
// sinkronisasi opsional diaktifkan.
type DriveFolderProvisioner struct {
	provider *Provider
}

// NewDriveFolderProvisioner membungkus provider menjadi Provisioner ringan.
func NewDriveFolderProvisioner(p *Provider) *DriveFolderProvisioner {
	return &DriveFolderProvisioner{provider: p}
}

var _ domain.Provisioner = (*DriveFolderProvisioner)(nil)

// Provision memastikan folder Drive tenant tersedia.
func (d *DriveFolderProvisioner) Provision(ctx context.Context, t *domain.Tenant) error {
	return d.provider.ProvisionDriveFolder(ctx, t)
}

// ProvisionDriveFolder membuat (atau memakai ulang) folder Drive tenant.
func (p *Provider) ProvisionDriveFolder(ctx context.Context, t *domain.Tenant) error {
	if t.RefreshToken == "" {
		return apperr.Unauthorized("refresh token Google tidak tersedia")
	}
	driveSvc, _, err := p.oauth.NewServices(ctx, t.RefreshToken)
	if err != nil {
		return apperr.Upstream("gagal menyiapkan koneksi Google").WithCause(err)
	}

	folderID, err := googleapi.NewDriveClient(driveSvc).
		EnsureFolder(ctx, fmt.Sprintf("POS Steca - %s", t.BusinessName))
	if err != nil {
		return apperr.Upstream("gagal menyiapkan folder Drive").WithCause(err)
	}
	t.FolderID = folderID

	p.Invalidate(t.ID)
	return nil
}

// Provision menyiapkan folder Drive dan spreadsheet untuk tenant baru, lalu
// memastikan seluruh sheet dan headernya tersedia.
func (p *Provider) Provision(ctx context.Context, t *domain.Tenant) error {
	if t.RefreshToken == "" {
		return apperr.Unauthorized("refresh token Google tidak tersedia")
	}
	driveSvc, sheetsSvc, err := p.oauth.NewServices(ctx, t.RefreshToken)
	if err != nil {
		return apperr.Upstream("gagal menyiapkan koneksi Google").WithCause(err)
	}
	driveClient := googleapi.NewDriveClient(driveSvc)

	folderName := fmt.Sprintf("POS Steca - %s", t.BusinessName)
	folderID, err := driveClient.EnsureFolder(ctx, folderName)
	if err != nil {
		return apperr.Upstream("gagal menyiapkan folder Drive").WithCause(err)
	}
	t.FolderID = folderID

	sheetName := fmt.Sprintf("POS Steca Data - %s", t.BusinessName)
	spreadsheetID, err := driveClient.EnsureSpreadsheet(ctx, folderID, sheetName)
	if err != nil {
		return apperr.Upstream("gagal menyiapkan spreadsheet").WithCause(err)
	}
	t.SpreadsheetID = spreadsheetID

	sheetsClient := googleapi.NewSheetsClient(sheetsSvc, spreadsheetID)
	if err := sheetsClient.EnsureSheets(ctx, SheetSpecs()); err != nil {
		return apperr.Upstream("gagal menyiapkan struktur sheet").WithCause(err)
	}

	p.Invalidate(t.ID)
	return nil
}
