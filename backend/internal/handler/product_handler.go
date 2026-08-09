package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/service"
)

// ProductHandler melayani modul inventori/produk.
type ProductHandler struct {
	products *service.ProductService
}

// NewProductHandler membuat handler produk.
func NewProductHandler(products *service.ProductService) *ProductHandler {
	return &ProductHandler{products: products}
}

// List mengembalikan katalog produk tenant aktif.
func (h *ProductHandler) List(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	items, err := h.products.List(c.Request.Context(), p.TenantID, c.Query("q"), c.Query("category"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Categories mengembalikan daftar kategori unik.
func (h *ProductHandler) Categories(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	items, err := h.products.Categories(c.Request.Context(), p.TenantID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Get mengembalikan satu produk.
func (h *ProductHandler) Get(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	item, err := h.products.Get(c.Request.Context(), p.TenantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Create menambahkan produk baru.
func (h *ProductHandler) Create(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.ProductInput
	if !bindJSON(c, &in) {
		return
	}
	item, err := h.products.Create(c.Request.Context(), p.TenantID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// Update mengubah produk.
func (h *ProductHandler) Update(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.ProductInput
	if !bindJSON(c, &in) {
		return
	}
	item, err := h.products.Update(c.Request.Context(), p.TenantID, c.Param("id"), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Delete menghapus produk beserta gambarnya.
func (h *ProductHandler) Delete(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	if err := h.products.Delete(c.Request.Context(), p.TenantID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UploadImage menerima multipart form berisi field "file" dan mengunggahnya
// ke folder Drive milik tenant.
func (h *ProductHandler) UploadImage(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondError(c, apperr.BadRequest("berkas gambar tidak ditemukan pada field \"file\""))
		return
	}
	if fileHeader.Size > service.MaxImageBytes {
		respondError(c, apperr.BadRequest("ukuran gambar maksimal 5 MB"))
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		respondError(c, apperr.BadRequest("gagal membaca berkas gambar").WithCause(err))
		return
	}
	defer src.Close()

	result, err := h.products.UploadImage(
		c.Request.Context(), p.TenantID,
		fileHeader.Filename, fileHeader.Header.Get("Content-Type"), fileHeader.Size, src,
	)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}
