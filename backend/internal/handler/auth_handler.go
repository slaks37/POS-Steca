package handler

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/service"
)

// AuthHandler melayani endpoint autentikasi dan profil tenant.
type AuthHandler struct {
	auth        *service.AuthService
	demo        *service.DemoService
	frontendURL string
}

// NewAuthHandler membuat handler autentikasi. demo boleh nil pada mode
// produksi.
func NewAuthHandler(auth *service.AuthService, demo *service.DemoService, frontendURL string) *AuthHandler {
	return &AuthHandler{auth: auth, demo: demo, frontendURL: frontendURL}
}

// GoogleURL mengembalikan URL consent Google untuk onboarding pemilik.
func (h *AuthHandler) GoogleURL(c *gin.Context) {
	url, err := h.auth.GoogleLoginURL()
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// GoogleCallback menerima redirect dari Google, lalu mengembalikan pengguna
// ke frontend dengan token aplikasi pada fragment URL.
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if errParam := c.Query("error"); errParam != "" {
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error="+url.QueryEscape(errParam))
		return
	}
	token, _, err := h.auth.HandleGoogleCallback(c.Request.Context(), c.Query("code"), c.Query("state"))
	if err != nil {
		appErr := apperr.As(err)
		c.Redirect(http.StatusFound, h.frontendURL+"/login?error="+url.QueryEscape(appErr.Message))
		return
	}
	// Token dikirim lewat fragment agar tidak tercatat di log server maupun
	// header Referer.
	c.Redirect(http.StatusFound, h.frontendURL+"/auth/callback#token="+url.QueryEscape(token))
}

type staffLoginRequest struct {
	TenantCode string `json:"tenant_code"`
	Email      string `json:"email"`
	PIN        string `json:"pin"`
}

// StaffLogin memproses login karyawan dengan kode bisnis + email + PIN.
func (h *AuthHandler) StaffLogin(c *gin.Context) {
	var req staffLoginRequest
	if !bindJSON(c, &req) {
		return
	}
	token, principal, err := h.auth.StaffLogin(c.Request.Context(), req.TenantCode, req.Email, req.PIN)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": principal})
}

type demoLoginRequest struct {
	BusinessName string `json:"business_name"`
}

// DemoLogin hanya aktif pada mode datastore memory: membuat tenant contoh
// berisi produk dan karyawan demo.
func (h *AuthHandler) DemoLogin(c *gin.Context) {
	if h.demo == nil {
		respondError(c, apperr.NotFound("mode demo tidak aktif"))
		return
	}
	var req demoLoginRequest
	_ = c.ShouldBindJSON(&req)

	token, principal, tenant, err := h.demo.Bootstrap(c.Request.Context(), req.BusinessName)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  principal,
		"tenant": gin.H{
			"id":            tenant.ID,
			"code":          tenant.Code,
			"business_name": tenant.BusinessName,
		},
		"kasir_demo": gin.H{"email": "kasir@demo.local", "pin": "123456"},
	})
}

// Me mengembalikan identitas pengguna beserta profil tenant aktif.
func (h *AuthHandler) Me(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	tenant, err := h.auth.Tenant(c.Request.Context(), p.TenantID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": p, "tenant": tenantView(tenant)})
}

type updateTenantRequest struct {
	BusinessName string `json:"business_name"`
}

// UpdateTenant mengubah nama bisnis (khusus owner).
func (h *AuthHandler) UpdateTenant(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var req updateTenantRequest
	if !bindJSON(c, &req) {
		return
	}
	tenant, err := h.auth.UpdateBusinessProfile(c.Request.Context(), p.TenantID, req.BusinessName)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tenant": tenantView(tenant)})
}

// tenantView menyaring field tenant yang aman dikirim ke frontend.
func tenantView(t *domain.Tenant) gin.H {
	return gin.H{
		"id":              t.ID,
		"code":            t.Code,
		"business_name":   t.BusinessName,
		"owner_email":     t.OwnerEmail,
		"owner_name":      t.OwnerName,
		"folder_url":      t.FolderURL(),
		"spreadsheet_url": t.SpreadsheetURL(),
		"created_at":      t.CreatedAt,
	}
}
