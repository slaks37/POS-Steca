package service

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"sort"
	"strings"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// MaxImageBytes membatasi ukuran gambar produk yang boleh diunggah.
const MaxImageBytes = 5 << 20 // 5 MB

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// ProductInput adalah payload pembuatan/perubahan produk.
type ProductInput struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Stock    int     `json:"stock"`
	SKU      string  `json:"sku"`
	ImageURL string  `json:"image_url"`
	ImageID  string  `json:"image_id"`
}

// ProductService mengelola katalog produk (sinkron dengan sheet "Products").
type ProductService struct {
	products domain.ProductRepository
	storage  domain.FileStorage
}

// NewProductService membuat service produk.
func NewProductService(products domain.ProductRepository, storage domain.FileStorage) *ProductService {
	return &ProductService{products: products, storage: storage}
}

// List mengembalikan katalog produk, opsional disaring kata kunci/kategori.
func (s *ProductService) List(ctx context.Context, tenantID, query, category string) ([]domain.Product, error) {
	items, err := s.products.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	category = strings.TrimSpace(category)

	out := make([]domain.Product, 0, len(items))
	for _, p := range items {
		if category != "" && !strings.EqualFold(p.Category, category) {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(p.Name + " " + p.SKU + " " + p.Category)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Categories mengembalikan daftar kategori unik untuk filter di layar kasir.
func (s *ProductService) Categories(ctx context.Context, tenantID string) ([]string, error) {
	items, err := s.products.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, p := range items {
		c := strings.TrimSpace(p.Category)
		if c == "" || seen[strings.ToLower(c)] {
			continue
		}
		seen[strings.ToLower(c)] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}

// Get mengambil satu produk.
func (s *ProductService) Get(ctx context.Context, tenantID, productID string) (*domain.Product, error) {
	return s.products.Get(ctx, tenantID, productID)
}

// Create menambahkan produk baru ke sheet "Products".
func (s *ProductService) Create(ctx context.Context, tenantID string, in ProductInput) (*domain.Product, error) {
	if err := validateProductInput(in); err != nil {
		return nil, err
	}
	if err := s.ensureSKUUnique(ctx, tenantID, in.SKU, ""); err != nil {
		return nil, err
	}
	p := &domain.Product{
		ID:       NewProductID(),
		Name:     strings.TrimSpace(in.Name),
		Category: strings.TrimSpace(in.Category),
		Price:    in.Price,
		Stock:    in.Stock,
		SKU:      strings.TrimSpace(in.SKU),
		ImageURL: strings.TrimSpace(in.ImageURL),
		ImageID:  strings.TrimSpace(in.ImageID),
	}
	if err := s.products.Create(ctx, tenantID, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update mengubah data produk yang sudah ada.
func (s *ProductService) Update(ctx context.Context, tenantID, productID string, in ProductInput) (*domain.Product, error) {
	if err := validateProductInput(in); err != nil {
		return nil, err
	}
	existing, err := s.products.Get(ctx, tenantID, productID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureSKUUnique(ctx, tenantID, in.SKU, productID); err != nil {
		return nil, err
	}

	oldImageID := existing.ImageID
	existing.Name = strings.TrimSpace(in.Name)
	existing.Category = strings.TrimSpace(in.Category)
	existing.Price = in.Price
	existing.Stock = in.Stock
	existing.SKU = strings.TrimSpace(in.SKU)
	existing.ImageURL = strings.TrimSpace(in.ImageURL)
	existing.ImageID = strings.TrimSpace(in.ImageID)

	if err := s.products.Update(ctx, tenantID, existing); err != nil {
		return nil, err
	}
	if oldImageID != "" && oldImageID != existing.ImageID && s.storage != nil {
		// Gambar lama dibuang setelah data produk berhasil disimpan; kegagalan
		// di sini tidak boleh menggagalkan permintaan.
		_ = s.storage.Delete(ctx, tenantID, oldImageID)
	}
	return existing, nil
}

// Delete menghapus produk beserta gambarnya di Drive.
func (s *ProductService) Delete(ctx context.Context, tenantID, productID string) error {
	existing, err := s.products.Get(ctx, tenantID, productID)
	if err != nil {
		return err
	}
	if err := s.products.Delete(ctx, tenantID, productID); err != nil {
		return err
	}
	if existing.ImageID != "" && s.storage != nil {
		_ = s.storage.Delete(ctx, tenantID, existing.ImageID)
	}
	return nil
}

// UploadResult adalah hasil unggah gambar produk.
type UploadResult struct {
	FileID string `json:"file_id"`
	URL    string `json:"url"`
}

// UploadImage mengunggah gambar produk ke folder Drive milik tenant.
func (s *ProductService) UploadImage(ctx context.Context, tenantID, filename, contentType string, size int64, r io.Reader) (*UploadResult, error) {
	if s.storage == nil {
		return nil, apperr.BadRequest("penyimpanan berkas tidak tersedia")
	}
	if size > MaxImageBytes {
		return nil, apperr.BadRequest("ukuran gambar maksimal 5 MB")
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		// Beberapa browser mengirim content type generik; coba tebak dari nama.
		guessed := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
		if ext, ok = allowedImageTypes[guessed]; !ok {
			return nil, apperr.BadRequest("format gambar harus JPG, PNG, WEBP, atau GIF")
		}
		contentType = guessed
	}

	safeName := fmt.Sprintf("produk-%s-%s%s", timex.Now().Format("20060102-150405"), strings.ToLower(randomString(5)), ext)
	fileID, url, err := s.storage.Upload(ctx, tenantID, safeName, contentType, io.LimitReader(r, MaxImageBytes))
	if err != nil {
		return nil, err
	}
	return &UploadResult{FileID: fileID, URL: url}, nil
}

func (s *ProductService) ensureSKUUnique(ctx context.Context, tenantID, sku, exceptID string) error {
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil
	}
	items, err := s.products.List(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, p := range items {
		if p.ID != exceptID && strings.EqualFold(strings.TrimSpace(p.SKU), sku) {
			return apperr.Conflict("SKU %q sudah dipakai produk lain", sku)
		}
	}
	return nil
}

func validateProductInput(in ProductInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return apperr.BadRequest("nama produk wajib diisi")
	}
	if in.Price < 0 {
		return apperr.BadRequest("harga tidak boleh negatif")
	}
	if in.Stock < 0 {
		return apperr.BadRequest("stok tidak boleh negatif")
	}
	return nil
}
