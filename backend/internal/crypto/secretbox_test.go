package crypto

import (
	"errors"
	"testing"
)

func TestSealOpenBolakBalik(t *testing.T) {
	sealer, err := NewSealer("kunci-rahasia-yang-panjang-dan-acak")
	if err != nil {
		t.Fatalf("membuat sealer: %v", err)
	}

	plaintext := "1//0abcdefghijklmnop-refresh-token"
	sealed, err := sealer.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal gagal: %v", err)
	}
	if sealed == plaintext {
		t.Fatal("ciphertext tidak boleh sama dengan plaintext")
	}

	opened, err := sealer.Open(sealed)
	if err != nil {
		t.Fatalf("Open gagal: %v", err)
	}
	if opened != plaintext {
		t.Errorf("hasil dekripsi = %q, ingin %q", opened, plaintext)
	}
}

func TestSealMenghasilkanNonceBerbeda(t *testing.T) {
	sealer, _ := NewSealer("kunci-uji")
	a, _ := sealer.Seal("data yang sama")
	b, _ := sealer.Seal("data yang sama")
	if a == b {
		t.Error("dua enkripsi plaintext sama harus berbeda (nonce acak)")
	}
}

func TestOpenGagalDenganKunciBerbeda(t *testing.T) {
	asli, _ := NewSealer("kunci-benar")
	salah, _ := NewSealer("kunci-salah")

	sealed, _ := asli.Seal("rahasia")
	if _, err := salah.Open(sealed); err == nil {
		t.Error("dekripsi dengan kunci berbeda seharusnya gagal")
	}
}

func TestNilaiKosongDanKunciKosong(t *testing.T) {
	sealer, _ := NewSealer("kunci-uji")
	if got, err := sealer.Seal(""); err != nil || got != "" {
		t.Errorf("Seal(\"\") = %q, %v", got, err)
	}
	if got, err := sealer.Open(""); err != nil || got != "" {
		t.Errorf("Open(\"\") = %q, %v", got, err)
	}
	if _, err := NewSealer(""); !errors.Is(err, ErrEmptyKey) {
		t.Errorf("NewSealer(\"\") error = %v, ingin ErrEmptyKey", err)
	}
}

func TestOpenMenolakDataRusak(t *testing.T) {
	sealer, _ := NewSealer("kunci-uji")
	for _, input := range []string{"bukan base64!!!", "aGFsbw"} {
		if _, err := sealer.Open(input); err == nil {
			t.Errorf("Open(%q) seharusnya gagal", input)
		}
	}
}
