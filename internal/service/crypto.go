// Package service — AES-GCM crypto helper for at-rest secrets.
//
// Sensitive fields (third-party API keys, service passwords, …) are
// stored in SQLite. We encrypt them with AES-256-GCM keyed off the JWT
// secret so a stolen DB file alone is not enough to recover the
// plaintext credentials.
//
// Format on disk:  "enc:v1:" + base64(nonce || ciphertext || tag)
//
// Legacy plaintext rows (no prefix) round-trip unchanged so an upgraded
// install does not need a migration step.
package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"go.uber.org/zap"
)

// encPrefix tags ciphertext rows so we can tell them apart from legacy
// plaintext values.
const encPrefix = "enc:v1:"

// CryptoService wraps an AES-GCM cipher derived from a stable per-install
// secret (the JWT secret).
type CryptoService struct {
	log  *zap.Logger
	aead cipher.AEAD
}

// NewCryptoService derives a 256-bit key from the given secret via
// SHA-256 and constructs an AES-GCM AEAD. Empty secrets yield a service
// whose Encrypt/Decrypt methods are pass-throughs (used in unit tests).
func NewCryptoService(secret string, log *zap.Logger) *CryptoService {
	c := &CryptoService{log: log}
	if strings.TrimSpace(secret) == "" {
		return c
	}
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		log.Error("crypto: aes.NewCipher", zap.Error(err))
		return c
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		log.Error("crypto: cipher.NewGCM", zap.Error(err))
		return c
	}
	c.aead = aead
	return c
}

// Encrypt returns the base64-encoded ciphertext (with prefix) for plain.
// Empty inputs round-trip unchanged.
func (c *CryptoService) Encrypt(plain string) string {
	if plain == "" || c.aead == nil {
		return plain
	}
	if strings.HasPrefix(plain, encPrefix) {
		return plain
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return plain
	}
	cipherBytes := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(cipherBytes)
}

// Decrypt returns the plaintext for an encrypted value. Plaintext rows
// (no prefix) are returned unchanged.
func (c *CryptoService) Decrypt(value string) string {
	if c == nil || value == "" || c.aead == nil {
		return value
	}
	if !strings.HasPrefix(value, encPrefix) {
		return value
	}
	raw := strings.TrimPrefix(value, encPrefix)
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return value
	}
	if len(data) < c.aead.NonceSize() {
		return value
	}
	nonce, cipherBytes := data[:c.aead.NonceSize()], data[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return value
	}
	return string(plain)
}

// IsEncrypted returns true if value carries the encrypted prefix.
func (c *CryptoService) IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encPrefix)
}

// MaskAPIKey returns "abcd****wxyz" so the key can be displayed in the
// admin UI without leaking it. Inputs shorter than 8 chars become "****".
func MaskAPIKey(plain string) string {
	plain = strings.TrimSpace(plain)
	if len(plain) < 8 {
		return "****"
	}
	return plain[:4] + "****" + plain[len(plain)-4:]
}

// ErrCryptoUnavailable is returned when callers expect crypto and the
// service is degraded (empty secret, init failure).
var ErrCryptoUnavailable = errors.New("crypto unavailable")
