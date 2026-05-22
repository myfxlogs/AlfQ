// Package crypto provides AES-256-GCM encryption for API keys at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// AESCipher encrypts and decrypts API keys using AES-256-GCM.
// The encryption key is derived from a 32-byte master key (set via env ALFQ_ENC_KEY
// or generated on first use and persisted to system_settings).
type AESCipher struct {
	gcm cipher.AEAD
}

// NewAESCipher creates an AES-256-GCM cipher from a 32-byte key.
func NewAESCipher(key []byte) (*AESCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm: %w", err)
	}
	return &AESCipher{gcm: gcm}, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext (nonce + data).
func (c *AESCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	ciphertext := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext produced by Encrypt.
func (c *AESCipher) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: decode: %w", err)
	}
	nonceSize := c.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}
	return string(plain), nil
}

// MaskKey returns a masked version of an API key showing only prefix and suffix.
// e.g. "sk-abc123...xyz" -> "sk-abc...****xyz"
func MaskKey(key string) string {
	if len(key) <= 12 {
		return key[:min(len(key), 4)] + "****"
	}
	return key[:7] + "..." + "****" + key[len(key)-4:]
}
