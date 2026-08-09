package gsheets

import (
	"context"
	"io"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// FileStorage menyimpan gambar produk di folder Drive milik tenant.
type FileStorage struct {
	p *Provider
}

// NewFileStorage membuat penyimpanan berkas berbasis Google Drive.
func NewFileStorage(p *Provider) *FileStorage {
	return &FileStorage{p: p}
}

var _ domain.FileStorage = (*FileStorage)(nil)

// Upload mengunggah gambar ke folder tenant dan mengembalikan ID serta URL-nya.
func (s *FileStorage) Upload(ctx context.Context, tenantID, filename, contentType string, r io.Reader) (string, string, error) {
	dr, _, tenant, err := s.p.Clients(ctx, tenantID)
	if err != nil {
		return "", "", err
	}
	if tenant.FolderID == "" {
		return "", "", apperr.Internal("folder Drive tenant belum tersedia")
	}
	id, url, err := dr.UploadImage(ctx, tenant.FolderID, filename, contentType, r)
	if err != nil {
		if id != "" {
			// Berkas terunggah namun izin publik gagal; tetap kembalikan hasil
			// agar produk punya referensi gambar.
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
	dr, _, _, err := s.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	if err := dr.DeleteFile(ctx, fileID); err != nil {
		return apperr.Upstream("gagal menghapus gambar").WithCause(err)
	}
	return nil
}
