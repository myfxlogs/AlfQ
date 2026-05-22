package crypto

import (
	"crypto/rand"
	"testing"
)

func TestNewAESCipher(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	c, err := NewAESCipher(key)
	if err != nil {
		t.Fatalf("NewAESCipher: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cipher")
	}
}

func TestNewAESCipherBadKey(t *testing.T) {
	_, err := NewAESCipher([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	c, _ := NewAESCipher(key)

	plain := "sk-this-is-a-test-api-key-12345"
	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain || enc == "" {
		t.Fatal("ciphertext should differ from plaintext")
	}

	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != plain {
		t.Fatalf("round-trip failed: got %q, want %q", dec, plain)
	}
}

func TestDecryptBadInput(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	c, _ := NewAESCipher(key)

	_, err := c.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for bad base64")
	}

	_, err = c.Decrypt("")
	if err == nil {
		t.Fatal("expected error for empty")
	}
}

func TestEncryptDifferentNonces(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	c, _ := NewAESCipher(key)

	plain := "same-plaintext"
	e1, _ := c.Encrypt(plain)
	e2, _ := c.Encrypt(plain)
	if e1 == e2 {
		t.Fatal("same plaintext should produce different ciphertexts (nonce)")
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"sk-abc123xyz", "****"},
		{"sk-proj-abcdefghijklmnop", "****"},
		{"short", "****"},
		{"ab", "****"},
	}
	for _, tt := range tests {
		masked := MaskKey(tt.input)
		if len(masked) == 0 {
			t.Errorf("MaskKey(%q) returned empty", tt.input)
		}
		// Masked key should never equal the original
		if masked == tt.input && len(tt.input) > 4 {
			t.Errorf("MaskKey(%q) should not equal input", tt.input)
		}
	}
}

func TestMaskKeyShort(t *testing.T) {
	masked := MaskKey("abc")
	if len(masked) <= len("abc") {
		// Short keys just get "****" appended
		if masked != "abc****" {
			t.Errorf("short key mask: got %q", masked)
		}
	}
}
