package timex

import (
	"testing"
	"time"
)

func TestParseBerbagaiFormat(t *testing.T) {
	want := time.Date(2026, time.March, 2, 14, 30, 0, 0, Location())
	cases := []string{
		"2026-03-02T14:30:00+07:00",
		"2026-03-02T14:30:00",
		"2026-03-02 14:30:00",
		"2026-03-02 14:30",
		"02/03/2026 14:30",
	}
	for _, input := range cases {
		if got := Parse(input); !got.Equal(want) {
			t.Errorf("Parse(%q) = %v, ingin %v", input, got, want)
		}
	}
}

func TestParseTanggalSaja(t *testing.T) {
	got := Parse("2026-03-02")
	want := time.Date(2026, time.March, 2, 0, 0, 0, 0, Location())
	if !got.Equal(want) {
		t.Errorf("Parse tanggal polos = %v, ingin %v", got, want)
	}
}

func TestParseNilaiTidakValid(t *testing.T) {
	// "45000" dulu ditafsirkan sebagai angka serial tanggal Google Sheets.
	// Setelah Sheets tidak lagi dipakai, angka polos bukan tanggal yang sah.
	for _, input := range []string{"", "   ", "bukan tanggal", "45000"} {
		if got := Parse(input); !got.IsZero() {
			t.Errorf("Parse(%q) = %v, ingin waktu nol", input, got)
		}
	}
}

func TestFormatDanParseBolakBalik(t *testing.T) {
	original := time.Date(2026, time.August, 9, 11, 59, 37, 0, Location())
	if got := Parse(Format(original)); !got.Equal(original) {
		t.Errorf("bolak-balik = %v, ingin %v", got, original)
	}
	if Format(time.Time{}) != "" {
		t.Error("waktu nol harus diformat sebagai string kosong")
	}
}

func TestStartOfWeekSelaluSenin(t *testing.T) {
	// 2026-03-02 adalah Senin; seluruh hari pada minggu itu harus memetakan
	// ke tanggal yang sama.
	senin := time.Date(2026, time.March, 2, 0, 0, 0, 0, Location())
	for offset := 0; offset < 7; offset++ {
		day := senin.AddDate(0, 0, offset).Add(13 * time.Hour)
		if got := StartOfWeek(day); !got.Equal(senin) {
			t.Errorf("StartOfWeek(%v) = %v, ingin %v", day, got, senin)
		}
	}
}

func TestStartOfDayDanMonth(t *testing.T) {
	ts := time.Date(2026, time.March, 17, 23, 45, 12, 0, Location())
	if got := StartOfDay(ts); got.Hour() != 0 || got.Day() != 17 {
		t.Errorf("StartOfDay = %v", got)
	}
	if got := StartOfMonth(ts); got.Day() != 1 || got.Month() != time.March {
		t.Errorf("StartOfMonth = %v", got)
	}
}

func TestSetLocationFromEnvValue(t *testing.T) {
	original := Location()
	defer SetLocation(original)

	if !SetLocationFromEnvValue("480") {
		t.Fatal("offset menit seharusnya diterima")
	}
	if _, offset := Now().Zone(); offset != 480*60 {
		t.Errorf("offset = %d detik, ingin %d", offset, 480*60)
	}
	if SetLocationFromEnvValue("Zona/Ngawur") {
		t.Error("zona waktu tidak dikenal seharusnya ditolak")
	}
	if SetLocationFromEnvValue("") {
		t.Error("nilai kosong seharusnya ditolak")
	}
}
