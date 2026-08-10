package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/service"
)

// CustomerHandler melayani modul CRM & loyalitas pelanggan.
type CustomerHandler struct {
	customers *service.CustomerService
}

// NewCustomerHandler membuat handler pelanggan.
func NewCustomerHandler(customers *service.CustomerService) *CustomerHandler {
	return &CustomerHandler{customers: customers}
}

// List mengembalikan daftar pelanggan, opsional disaring lewat query "q".
func (h *CustomerHandler) List(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	items, err := h.customers.List(c.Request.Context(), p.TenantID, c.Query("q"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"meta": gin.H{"rupiah_per_poin": service.RupiahPerPoint},
	})
}

// Get mengembalikan satu pelanggan.
func (h *CustomerHandler) Get(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	customer, err := h.customers.Get(c.Request.Context(), p.TenantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": customer})
}

// History mengembalikan pelanggan beserta riwayat pembeliannya.
func (h *CustomerHandler) History(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	history, err := h.customers.History(c.Request.Context(), p.TenantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": history})
}

// Create mendaftarkan pelanggan baru (kasir boleh, agar bisa dilakukan saat
// transaksi berlangsung).
func (h *CustomerHandler) Create(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.CustomerInput
	if !bindJSON(c, &in) {
		return
	}
	customer, err := h.customers.Create(c.Request.Context(), p.TenantID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": customer})
}

// Update mengubah data pelanggan.
func (h *CustomerHandler) Update(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.CustomerInput
	if !bindJSON(c, &in) {
		return
	}
	customer, err := h.customers.Update(c.Request.Context(), p.TenantID, c.Param("id"), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": customer})
}

// Delete menghapus pelanggan.
func (h *CustomerHandler) Delete(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	if err := h.customers.Delete(c.Request.Context(), p.TenantID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
