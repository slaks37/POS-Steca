package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/service"
)

const principalKey = "pos_principal"

// CORS mengizinkan hanya origin frontend yang terdaftar.
func CORS(allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[strings.TrimSuffix(o, "/")] = true
	}
	return func(c *gin.Context) {
		origin := strings.TrimSuffix(c.GetHeader("Origin"), "/")
		if origin != "" && allowedSet[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// RequestLogger mencatat setiap permintaan beserta durasinya.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"durasi_ms", time.Since(start).Milliseconds(),
		)
	}
}

// Recovery mengubah panic menjadi respons 500 tanpa membocorkan stack trace.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		slog.Error("panic pada handler", "path", c.Request.URL.Path, "panic", recovered)
		respondError(c, apperr.Internal("terjadi kesalahan pada server"))
	})
}

// Authenticate memvalidasi bearer token dan menyimpan principal di context.
func Authenticate(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			respondError(c, apperr.Unauthorized("token akses tidak ditemukan"))
			return
		}
		principal, err := auth.ParseToken(strings.TrimSpace(header[len("bearer "):]))
		if err != nil {
			respondError(c, err)
			return
		}
		c.Set(principalKey, principal)
		c.Next()
	}
}

// RequireRole membatasi akses endpoint pada role tertentu.
func RequireRole(roles ...domain.Role) gin.HandlerFunc {
	allowed := make(map[domain.Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		p := principalFrom(c)
		if p == nil {
			respondError(c, apperr.Unauthorized("sesi tidak valid"))
			return
		}
		if !allowed[p.Role] {
			respondError(c, apperr.Forbidden("akses ditolak untuk role %s", p.Role))
			return
		}
		c.Next()
	}
}

func principalFrom(c *gin.Context) *service.Principal {
	v, ok := c.Get(principalKey)
	if !ok {
		return nil
	}
	p, ok := v.(*service.Principal)
	if !ok {
		return nil
	}
	return p
}

// mustPrincipal mengambil principal, atau menghentikan permintaan bila
// middleware autentikasi terlewat.
func mustPrincipal(c *gin.Context) (*service.Principal, bool) {
	p := principalFrom(c)
	if p == nil {
		respondError(c, apperr.Unauthorized("sesi tidak valid"))
		return nil, false
	}
	return p, true
}
