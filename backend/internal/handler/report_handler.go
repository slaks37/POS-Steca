package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/service"
)

// ReportHandler melayani modul laporan penjualan dan dashboard owner.
type ReportHandler struct {
	reports *service.ReportService
	auth    *service.AuthService
}

// NewReportHandler membuat handler laporan.
func NewReportHandler(reports *service.ReportService, auth *service.AuthService) *ReportHandler {
	return &ReportHandler{reports: reports, auth: auth}
}

// Sales mengembalikan laporan penjualan harian/mingguan/bulanan.
func (h *ReportHandler) Sales(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	from, to, err := parseDateRange(c)
	if err != nil {
		respondError(c, err)
		return
	}
	report, err := h.reports.Sales(c.Request.Context(), p.TenantID, service.ParseGranularity(c.Query("granularity")), from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

// Transactions mengembalikan riwayat transaksi tergabung per struk.
func (h *ReportHandler) Transactions(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	from, to, err := parseDateRange(c)
	if err != nil {
		respondError(c, err)
		return
	}
	items, err := h.reports.TransactionHistory(c.Request.Context(), p.TenantID, from, to, parseLimit(c, 100, 500))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Dashboard mengembalikan ringkasan performa toko untuk pemilik.
func (h *ReportHandler) Dashboard(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	tenant, err := h.auth.Tenant(c.Request.Context(), p.TenantID)
	if err != nil {
		respondError(c, err)
		return
	}
	summary, err := h.reports.Dashboard(c.Request.Context(), p.TenantID, tenant.BusinessName)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}
