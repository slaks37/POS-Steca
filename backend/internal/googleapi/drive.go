package googleapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// MimeFolder adalah tipe MIME folder Google Drive.
const MimeFolder = "application/vnd.google-apps.folder"

// DriveClient membungkus operasi Drive yang dipakai POS Steca.
type DriveClient struct {
	svc *drive.Service
}

// NewDriveClient membuat pembungkus Drive.
func NewDriveClient(svc *drive.Service) *DriveClient {
	return &DriveClient{svc: svc}
}

// EnsureFolder mencari folder milik aplikasi berdasarkan nama, dan membuatnya
// bila belum ada. Mengembalikan ID folder.
func (d *DriveClient) EnsureFolder(ctx context.Context, name string) (string, error) {
	query := fmt.Sprintf("mimeType='%s' and name='%s' and trashed=false and 'root' in parents",
		MimeFolder, escapeQuery(name))
	list, err := call(ctx, func() (*drive.FileList, error) {
		return d.svc.Files.List().Q(query).Spaces("drive").Fields("files(id,name)").PageSize(10).Context(ctx).Do()
	})
	if err != nil {
		return "", fmt.Errorf("mencari folder Drive: %w", err)
	}
	if len(list.Files) > 0 {
		return list.Files[0].Id, nil
	}

	created, err := call(ctx, func() (*drive.File, error) {
		return d.svc.Files.Create(&drive.File{
			Name:        name,
			MimeType:    MimeFolder,
			Description: "Folder gambar produk Steca POS",
		}).Fields("id").Context(ctx).Do()
	})
	if err != nil {
		return "", fmt.Errorf("membuat folder Drive: %w", err)
	}
	return created.Id, nil
}

// UploadImage mengunggah berkas ke folder tenant, memberinya izin baca publik
// (agar bisa ditampilkan di frontend), lalu mengembalikan ID dan URL-nya.
func (d *DriveClient) UploadImage(ctx context.Context, folderID, filename, contentType string, r io.Reader) (string, string, error) {
	file, err := call(ctx, func() (*drive.File, error) {
		return d.svc.Files.Create(&drive.File{
			Name:    filename,
			Parents: []string{folderID},
		}).Media(r, googleapi.ContentType(contentType)).
			Fields("id,webViewLink,thumbnailLink").
			Context(ctx).Do()
	})
	if err != nil {
		return "", "", fmt.Errorf("mengunggah gambar ke Drive: %w", err)
	}

	if _, err := call(ctx, func() (*drive.Permission, error) {
		return d.svc.Permissions.Create(file.Id, &drive.Permission{
			Role: "reader",
			Type: "anyone",
		}).Context(ctx).Do()
	}); err != nil {
		// Gambar tetap tersimpan meski izin publik gagal dipasang; pemilik
		// akun bisa membagikannya manual dari Drive.
		return file.Id, PublicImageURL(file.Id), fmt.Errorf("memasang izin baca publik: %w", err)
	}

	return file.Id, PublicImageURL(file.Id), nil
}

// DeleteFile memindahkan berkas ke sampah Drive.
func (d *DriveClient) DeleteFile(ctx context.Context, fileID string) error {
	_, err := call(ctx, func() (*drive.File, error) {
		return d.svc.Files.Update(fileID, &drive.File{Trashed: true}).Fields("id").Context(ctx).Do()
	})
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("menghapus berkas Drive: %w", err)
	}
	return nil
}

// PublicImageURL membentuk URL tampilan gambar Drive yang bisa dipasang pada
// tag <img>.
func PublicImageURL(fileID string) string {
	return "https://drive.google.com/thumbnail?id=" + fileID + "&sz=w800"
}

// IsNotFound memeriksa apakah error berasal dari Google API dengan status 404.
func IsNotFound(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == 404
}

func escapeQuery(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `'`, `\'`)
}
