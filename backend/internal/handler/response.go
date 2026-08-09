// Package handler berisi layer HTTP (Gin): routing, middleware, validasi
// payload, dan pemetaan error domain ke status HTTP.
package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// respondError memetakan error aplikasi ke respons JSON yang konsisten.
func respondError(c *gin.Context, err error) {
	appErr := apperr.As(err)
	if appErr.Status >= http.StatusInternalServerError {
		slog.Error("permintaan gagal",
			"path", c.FullPath(),
			"method", c.Request.Method,
			"code", appErr.Code,
			"error", appErr.Error(),
		)
	}
	c.AbortWithStatusJSON(appErr.Status, gin.H{"error": gin.H{
		"code":    appErr.Code,
		"message": appErr.Message,
	}})
}

// bindJSON membaca body JSON dan mengubah kegagalan parsing menjadi 400.
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		respondError(c, apperr.BadRequest("format permintaan tidak valid").WithCause(err))
		return false
	}
	return true
}

// parseDateRange membaca query "from" dan "to" (format YYYY-MM-DD atau
// RFC3339). Batas akhir bersifat eksklusif; tanggal polos otomatis digeser ke
// akhir hari agar rentangnya inklusif bagi pengguna.
func parseDateRange(c *gin.Context) (time.Time, time.Time, error) {
	var from, to time.Time

	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		from = timex.Parse(raw)
		if from.IsZero() {
			return from, to, apperr.BadRequest("parameter from bukan tanggal yang valid")
		}
		from = timex.StartOfDay(from)
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		to = timex.Parse(raw)
		if to.IsZero() {
			return from, to, apperr.BadRequest("parameter to bukan tanggal yang valid")
		}
		to = timex.StartOfDay(to).AddDate(0, 0, 1)
	}
	if !from.IsZero() && !to.IsZero() && !to.After(from) {
		return from, to, apperr.BadRequest("tanggal akhir harus setelah tanggal mulai")
	}
	return from, to, nil
}

func parseLimit(c *gin.Context, fallback, max int) int {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > max {
		return max
	}
	return n
}
