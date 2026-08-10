// Package timex menyeragamkan zona waktu dan format tanggal yang dipakai di
// seluruh aplikasi.
package timex

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Layout penulisan tanggal: RFC3339 lengkap dengan offset zona waktu sehingga
// tidak ambigu ketika dibaca ulang atau diekspor.
const Layout = time.RFC3339

var loc atomic.Pointer[time.Location]

func init() {
	// Default WIB (UTC+7). Tidak bergantung pada database zona waktu sistem
	// agar tetap benar di container minimal.
	SetLocation(time.FixedZone("WIB", 7*3600))
}

// SetLocation mengganti zona waktu aplikasi.
func SetLocation(l *time.Location) {
	if l == nil {
		return
	}
	loc.Store(l)
}

// SetLocationFromEnvValue menerima nama IANA ("Asia/Jakarta") atau offset
// menit ("420"). Mengembalikan false bila nilai tidak dikenali.
func SetLocationFromEnvValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if mins, err := strconv.Atoi(v); err == nil {
		SetLocation(time.FixedZone("LOCAL", mins*60))
		return true
	}
	l, err := time.LoadLocation(v)
	if err != nil {
		return false
	}
	SetLocation(l)
	return true
}

// Location mengembalikan zona waktu aplikasi.
func Location() *time.Location {
	return loc.Load()
}

// Now mengembalikan waktu sekarang pada zona waktu aplikasi.
func Now() time.Time {
	return time.Now().In(Location())
}

// Format menuliskan waktu dengan layout standar aplikasi.
func Format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(Location()).Format(Layout)
}

// layouts yang diterima saat membaca tanggal dari parameter permintaan.
// Klien (web maupun Android) bisa mengirim beberapa bentuk umum, jadi parser
// dibuat toleran.
var layouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"02/01/2006 15:04:05",
	"02/01/2006 15:04",
	"02/01/2006",
	"1/2/2006 15:04:05",
	"1/2/2006",
}

// Parse membaca nilai tanggal. Waktu nol dikembalikan bila tidak bisa dibaca.
func Parse(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, Location()); err == nil {
			return t.In(Location())
		}
	}
	return time.Time{}
}

// StartOfDay mengembalikan awal hari pada zona waktu aplikasi.
func StartOfDay(t time.Time) time.Time {
	t = t.In(Location())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Location())
}

// StartOfWeek mengembalikan hari Senin pada minggu yang sama.
func StartOfWeek(t time.Time) time.Time {
	d := StartOfDay(t)
	offset := (int(d.Weekday()) + 6) % 7 // Senin = 0
	return d.AddDate(0, 0, -offset)
}

// StartOfMonth mengembalikan tanggal 1 pada bulan yang sama.
func StartOfMonth(t time.Time) time.Time {
	t = t.In(Location())
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, Location())
}
