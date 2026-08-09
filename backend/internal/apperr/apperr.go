// Package apperr menyediakan error aplikasi yang bisa dipetakan langsung ke
// status HTTP oleh layer handler.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error adalah error domain dengan kode HTTP dan kode mesin.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	err     error
}

func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.err)
	}
	return e.Message
}

// Unwrap mengembalikan error asal agar kompatibel dengan errors.Is/As.
func (e *Error) Unwrap() error { return e.err }

// WithCause melampirkan error asal tanpa membocorkannya ke klien.
func (e *Error) WithCause(err error) *Error {
	clone := *e
	clone.err = err
	return &clone
}

func newf(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

// BadRequest menandai input klien yang tidak valid.
func BadRequest(format string, args ...any) *Error {
	return newf(http.StatusBadRequest, "bad_request", format, args...)
}

// Unauthorized menandai kredensial yang hilang atau tidak sah.
func Unauthorized(format string, args ...any) *Error {
	return newf(http.StatusUnauthorized, "unauthorized", format, args...)
}

// Forbidden menandai akses yang tidak diizinkan untuk role terkait.
func Forbidden(format string, args ...any) *Error {
	return newf(http.StatusForbidden, "forbidden", format, args...)
}

// NotFound menandai sumber daya yang tidak ditemukan.
func NotFound(format string, args ...any) *Error {
	return newf(http.StatusNotFound, "not_found", format, args...)
}

// Conflict menandai bentrokan state, misalnya SKU ganda.
func Conflict(format string, args ...any) *Error {
	return newf(http.StatusConflict, "conflict", format, args...)
}

// Internal menandai kegagalan tak terduga di sisi server.
func Internal(format string, args ...any) *Error {
	return newf(http.StatusInternalServerError, "internal_error", format, args...)
}

// Upstream menandai kegagalan saat memanggil Google Drive/Sheets API.
func Upstream(format string, args ...any) *Error {
	return newf(http.StatusBadGateway, "upstream_error", format, args...)
}

// As mengekstrak *Error dari rantai error, atau membungkusnya sebagai
// internal error bila bukan error aplikasi.
func As(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal("terjadi kesalahan pada server").WithCause(err)
}
