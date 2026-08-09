package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/service"
)

// OrderHandler melayani modul kasir dan manajemen pesanan.
type OrderHandler struct {
	orders *service.OrderService
}

// NewOrderHandler membuat handler pesanan.
func NewOrderHandler(orders *service.OrderService) *OrderHandler {
	return &OrderHandler{orders: orders}
}

// Checkout mencatat penjualan di kasir dan mengembalikan struk digital.
func (h *OrderHandler) Checkout(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.CheckoutInput
	if !bindJSON(c, &in) {
		return
	}
	result, err := h.orders.Checkout(c.Request.Context(), p.TenantID, p.Name, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

// List mengembalikan pesanan sesuai filter status/sumber/tanggal.
func (h *OrderHandler) List(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	from, to, err := parseDateRange(c)
	if err != nil {
		respondError(c, err)
		return
	}
	items, err := h.orders.List(c.Request.Context(), p.TenantID, c.Query("status"), c.Query("source"), from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Get mengembalikan satu pesanan.
func (h *OrderHandler) Get(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	order, err := h.orders.Get(c.Request.Context(), p.TenantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

// UpdateStatus memindahkan pesanan pada alur baru -> diproses -> selesai.
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var req updateStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	order, err := h.orders.UpdateStatus(c.Request.Context(), p.TenantID, c.Param("id"), req.Status)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

type settleRequest struct {
	PaymentMethod string  `json:"payment_method"`
	AmountPaid    float64 `json:"amount_paid"`
}

// Settle menyelesaikan pembayaran pesanan online di kasir.
func (h *OrderHandler) Settle(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var req settleRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.orders.Settle(c.Request.Context(), p.TenantID, c.Param("id"), p.Name, req.PaymentMethod, req.AmountPaid)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
