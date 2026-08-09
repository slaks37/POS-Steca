package googleapi

import (
	"context"
	"errors"
	"math/rand"
	"time"

	gapi "google.golang.org/api/googleapi"
)

const (
	maxAttempts  = 4
	baseBackoff  = 300 * time.Millisecond
	maxBackoffMs = 4000
)

// call menjalankan panggilan Google API dengan retry eksponensial untuk
// kegagalan sementara (429 rate limit dan 5xx).
func call[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var (
		zero T
		last error
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := baseBackoff * time.Duration(1<<(attempt-1))
			if delay > maxBackoffMs*time.Millisecond {
				delay = maxBackoffMs * time.Millisecond
			}
			// Jitter mencegah beberapa request menabrak kuota bersamaan.
			delay += time.Duration(rand.Int63n(int64(baseBackoff)))
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
		res, err := fn()
		if err == nil {
			return res, nil
		}
		last = err
		if !retryable(err) {
			return zero, err
		}
	}
	return zero, last
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var gerr *gapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == 429 || gerr.Code >= 500
	}
	// Kegagalan jaringan sementara juga layak dicoba ulang.
	return true
}
