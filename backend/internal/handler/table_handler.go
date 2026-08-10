package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/service"
)

// TableHandler melayani modul denah/nomor meja.
type TableHandler struct {
	tables *service.TableService
}

// NewTableHandler membuat handler meja.
func NewTableHandler(tables *service.TableService) *TableHandler {
	return &TableHandler{tables: tables}
}

// List mengembalikan seluruh meja beserta statusnya.
func (h *TableHandler) List(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	items, err := h.tables.List(c.Request.Context(), p.TenantID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Create menambahkan meja baru.
func (h *TableHandler) Create(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.TableInput
	if !bindJSON(c, &in) {
		return
	}
	table, err := h.tables.Create(c.Request.Context(), p.TenantID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": table})
}

// Update mengubah data meja.
func (h *TableHandler) Update(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.TableInput
	if !bindJSON(c, &in) {
		return
	}
	table, err := h.tables.Update(c.Request.Context(), p.TenantID, c.Param("id"), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": table})
}

type tableStatusRequest struct {
	Status string `json:"status"`
}

// UpdateStatus mengubah status meja (kosong, terisi, dibersihkan). Kasir
// boleh memakainya langsung dari layar transaksi maupun papan pesanan.
func (h *TableHandler) UpdateStatus(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var req tableStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	table, err := h.tables.UpdateStatus(c.Request.Context(), p.TenantID, c.Param("id"), req.Status)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": table})
}

// Delete menghapus meja.
func (h *TableHandler) Delete(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	if err := h.tables.Delete(c.Request.Context(), p.TenantID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
