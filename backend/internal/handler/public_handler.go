package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/service"
)

// PublicHandler melayani kanal pemesanan online: menu dan pembuatan pesanan
// tanpa login, diakses lewat kode bisnis tenant.
type PublicHandler struct {
	tenants  domain.TenantStore
	products *service.ProductService
	orders   *service.OrderService
}

// NewPublicHandler membuat handler kanal online.
func NewPublicHandler(tenants domain.TenantStore, products *service.ProductService, orders *service.OrderService) *PublicHandler {
	return &PublicHandler{tenants: tenants, products: products, orders: orders}
}

// Menu mengembalikan katalog produk yang bisa dipesan pelanggan.
func (h *PublicHandler) Menu(c *gin.Context) {
	tenant, err := h.tenants.GetByCode(c.Request.Context(), c.Param("code"))
	if err != nil {
		respondError(c, err)
		return
	}
	items, err := h.products.List(c.Request.Context(), tenant.ID, c.Query("q"), c.Query("category"))
	if err != nil {
		respondError(c, err)
		return
	}

	// Hanya field yang relevan bagi pelanggan yang diekspos.
	menu := make([]gin.H, 0, len(items))
	for _, p := range items {
		if p.Stock <= 0 {
			continue
		}
		menu = append(menu, gin.H{
			"id":        p.ID,
			"name":      p.Name,
			"category":  p.Category,
			"price":     p.Price,
			"image_url": p.ImageURL,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     menu,
		"business": gin.H{"name": tenant.BusinessName, "code": tenant.Code},
	})
}

// CreateOrder mencatat pesanan online yang akan muncul di papan pesanan kasir.
func (h *PublicHandler) CreateOrder(c *gin.Context) {
	tenant, err := h.tenants.GetByCode(c.Request.Context(), c.Param("code"))
	if err != nil {
		respondError(c, err)
		return
	}
	var in service.OnlineOrderInput
	if !bindJSON(c, &in) {
		return
	}
	order, err := h.orders.CreateOnlineOrder(c.Request.Context(), tenant.ID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"id":         order.ID,
		"code":       order.Code,
		"status":     order.Status,
		"total":      order.Total,
		"items":      order.Items,
		"created_at": order.CreatedAt,
	}})
}

// RateLimit membatasi jumlah permintaan per alamat IP pada endpoint publik,
// sebagai perlindungan dasar dari spam pesanan.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type counter struct {
		count int
		reset time.Time
	}
	var (
		mu      sync.Mutex
		buckets = map[string]*counter{}
	)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := buckets[ip]
		if !ok || now.After(b.reset) {
			b = &counter{reset: now.Add(window)}
			buckets[ip] = b
		}
		b.count++
		exceeded := b.count > limit
		// Bersihkan bucket kedaluwarsa agar peta tidak tumbuh tanpa batas.
		if len(buckets) > 1024 {
			for k, v := range buckets {
				if now.After(v.reset) {
					delete(buckets, k)
				}
			}
		}
		mu.Unlock()

		if exceeded {
			respondError(c, &apperr.Error{
				Status:  http.StatusTooManyRequests,
				Code:    "rate_limited",
				Message: "terlalu banyak permintaan, coba lagi beberapa saat lagi",
			})
			return
		}
		c.Next()
	}
}
