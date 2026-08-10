// Package drivestore menyimpan gambar produk di folder Google Drive milik
// masing-masing tenant.
//
// Data terstruktur (produk, transaksi, pesanan, karyawan, pelanggan, meja)
// seluruhnya berada di PostgreSQL. Google Drive hanya berperan untuk berkas
// biner — plus OAuth2 sebagai jalur login pemilik, yang ditangani package
// googleapi.
package drivestore

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/googleapi"
)

// folderNameFormat adalah nama folder Drive yang dibuat untuk tiap tenant.
const folderNameFormat = "Steca POS - %s"

// Provider membuat dan menyimpan sementara klien Drive per tenant. Setiap
// tenant memakai refresh token miliknya sendiri, sehingga berkas satu bisnis
// tidak pernah tercampur dengan bisnis lain.
type Provider struct {
	oauth   *googleapi.OAuthManager
	tenants domain.TenantStore

	mu    sync.Mutex
	cache map[string]*googleapi.DriveClient
}

// NewProvider membuat provider klien Drive per tenant.
func NewProvider(oauth *googleapi.OAuthManager, tenants domain.TenantStore) *Provider {
	return &Provider{
		oauth:   oauth,
		tenants: tenants,
		cache:   map[string]*googleapi.DriveClient{},
	}
}

// Client mengembalikan klien Drive milik tenant beserta data tenantnya.
func (p *Provider) Client(ctx context.Context, tenantID string) (*googleapi.DriveClient, *domain.Tenant, error) {
	tenant, err := p.tenants.GetByID(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if tenant.RefreshToken == "" {
		return nil, nil, apperr.Unauthorized("koneksi Google untuk akun ini belum aktif, silakan hubungkan ulang")
	}

	p.mu.Lock()
	cached, ok := p.cache[tenantID]
	p.mu.Unlock()
	if ok {
		return cached, tenant, nil
	}

	client, err := p.newClient(ctx, tenant.RefreshToken)
	if err != nil {
		return nil, nil, err
	}

	p.mu.Lock()
	p.cache[tenantID] = client
	p.mu.Unlock()

	return client, tenant, nil
}

func (p *Provider) newClient(ctx context.Context, refreshToken string) (*googleapi.DriveClient, error) {
	driveSvc, err := p.oauth.NewDriveService(ctx, refreshToken)
	if err != nil {
		return nil, apperr.Upstream("gagal menyiapkan koneksi Google Drive").WithCause(err)
	}
	return googleapi.NewDriveClient(driveSvc), nil
}

// Invalidate membuang klien tersimpan, dipakai setelah tenant menghubungkan
// ulang akun Google-nya.
func (p *Provider) Invalidate(tenantID string) {
	p.mu.Lock()
	delete(p.cache, tenantID)
	p.mu.Unlock()
}

// FolderProvisioner menyiapkan folder Drive tenant saat onboarding. Hanya
// folder yang dibuat: tidak ada spreadsheet, karena data terstruktur sudah
// sepenuhnya berada di PostgreSQL.
type FolderProvisioner struct {
	provider *Provider
}

// NewFolderProvisioner membuat provisioner folder Drive.
func NewFolderProvisioner(p *Provider) *FolderProvisioner {
	return &FolderProvisioner{provider: p}
}

var _ domain.Provisioner = (*FolderProvisioner)(nil)

// Provision memastikan folder Drive tenant tersedia dan menyimpan ID-nya pada
// data tenant. Operasi ini idempoten: folder yang sudah ada dipakai ulang.
func (f *FolderProvisioner) Provision(ctx context.Context, t *domain.Tenant) error {
	if t.RefreshToken == "" {
		return apperr.Unauthorized("refresh token Google tidak tersedia")
	}

	client, err := f.provider.newClient(ctx, t.RefreshToken)
	if err != nil {
		return err
	}

	folderID, err := client.EnsureFolder(ctx, fmt.Sprintf(folderNameFormat, t.BusinessName))
	if err != nil {
		return apperr.Upstream("gagal menyiapkan folder Drive").WithCause(err)
	}
	t.FolderID = folderID

	f.provider.Invalidate(t.ID)
	return nil
}

// FileStorage menyimpan gambar produk di folder Drive milik tenant.
type FileStorage struct {
	provider *Provider
}

// NewFileStorage membuat penyimpanan berkas berbasis Google Drive.
func NewFileStorage(p *Provider) *FileStorage {
	return &FileStorage{provider: p}
}

var _ domain.FileStorage = (*FileStorage)(nil)

// Upload mengunggah gambar ke folder tenant dan mengembalikan ID serta URL-nya.
func (s *FileStorage) Upload(
	ctx context.Context, tenantID, filename, contentType string, r io.Reader,
) (string, string, error) {
	client, tenant, err := s.provider.Client(ctx, tenantID)
	if err != nil {
		return "", "", err
	}
	if tenant.FolderID == "" {
		return "", "", apperr.Internal("folder Drive tenant belum tersedia")
	}

	id, url, err := client.UploadImage(ctx, tenant.FolderID, filename, contentType, r)
	if err != nil {
		if id != "" {
			// Berkas terunggah namun izin publik gagal dipasang; tetap
			// kembalikan hasilnya agar produk punya referensi gambar.
			return id, url, nil
		}
		return "", "", apperr.Upstream("gagal mengunggah gambar").WithCause(err)
	}
	return id, url, nil
}

// Delete memindahkan gambar ke sampah Drive.
func (s *FileStorage) Delete(ctx context.Context, tenantID, fileID string) error {
	if fileID == "" {
		return nil
	}
	client, _, err := s.provider.Client(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := client.DeleteFile(ctx, fileID); err != nil {
		return apperr.Upstream("gagal menghapus gambar").WithCause(err)
	}
	return nil
}
