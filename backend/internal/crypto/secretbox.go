// Package crypto membungkus enkripsi simetris untuk data sensitif yang
// disimpan di disk, khususnya refresh token OAuth2 milik tiap tenant.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrEmptyKey dikembalikan bila kunci enkripsi tidak diisi.
var ErrEmptyKey = errors.New("kunci enkripsi kosong")

// Sealer mengenkripsi dan mendekripsi string dengan AES-256-GCM.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer menurunkan kunci 256-bit dari passphrase menggunakan SHA-256.
// Passphrase sebaiknya berupa nilai acak panjang yang disimpan di secret
// manager, bukan kata sandi yang mudah ditebak.
func NewSealer(passphrase string) (*Sealer, error) {
	if passphrase == "" {
		return nil, ErrEmptyKey
	}
	sum := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("membuat cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("membuat GCM: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Seal mengenkripsi plaintext dan mengembalikannya dalam base64 URL-safe.
func (s *Sealer) Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("membuat nonce: %w", err)
	}
	sealed := s.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open mendekripsi hasil Seal.
func (s *Sealer) Open(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("dekode base64: %w", err)
	}
	if len(raw) < s.aead.NonceSize() {
		return "", errors.New("ciphertext terlalu pendek")
	}
	nonce, body := raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():]
	plain, err := s.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("dekripsi gagal: %w", err)
	}
	return string(plain), nil
}
