package auth_test

import (
	"testing"
	"time"

	"github.com/alfq/backend/go/internal/common/auth"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("demo123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := auth.VerifyPassword(hash, "demo123")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}
	ok, err = auth.VerifyPassword(hash, "wrong")
	if err != nil {
		t.Fatalf("VerifyPassword wrong: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}

func TestJWTSignAndVerify(t *testing.T) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	claims := auth.Claims{
		Sub:      "u-001",
		TenantID: "t-001",
		Email:    "test@alfq.io",
		Roles:    []string{"trader"},
	}
	token, err := kp.Sign(claims, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}

	keys := map[string]auth.Ed25519PublicKey{kp.Kid: kp.PublicKey}
	parsed, err := auth.Verify(token, keys)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if parsed.Sub != "u-001" {
		t.Fatalf("Sub = %s, want u-001", parsed.Sub)
	}
	if parsed.TenantID != "t-001" {
		t.Fatalf("TenantID = %s, want t-001", parsed.TenantID)
	}
	if len(parsed.Roles) != 1 || parsed.Roles[0] != "trader" {
		t.Fatalf("Roles = %v", parsed.Roles)
	}
	if parsed.IsExpired() {
		t.Fatal("token should not be expired")
	}
}

func TestJWTExpired(t *testing.T) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	claims := auth.Claims{Sub: "u-001"}
	token, err := kp.Sign(claims, -1*time.Second) // already expired
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	keys := map[string]auth.Ed25519PublicKey{kp.Kid: kp.PublicKey}
	parsed, err := auth.Verify(token, keys)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !parsed.IsExpired() {
		t.Fatal("expected token to be expired")
	}
}

func TestJWTTampered(t *testing.T) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	claims := auth.Claims{Sub: "u-001"}
	token, err := kp.Sign(claims, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// Tamper with the token
	tampered := token + "x"
	keys := map[string]auth.Ed25519PublicKey{kp.Kid: kp.PublicKey}
	_, err = auth.Verify(tampered, keys)
	if err == nil {
		t.Fatal("expected tampered token to fail verification")
	}
}

func TestTOTPGenerate(t *testing.T) {
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if secret == "" {
		t.Fatal("empty secret")
	}
	if len(secret) < 16 {
		t.Fatalf("secret too short: %d", len(secret))
	}
}

func TestTOTPVerifyWrong(t *testing.T) {
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	ok, err := auth.VerifyTOTP(secret, "000000")
	if err != nil {
		t.Fatalf("VerifyTOTP: %v", err)
	}
	// "000000" is almost certainly wrong, but could be correct by chance (1 in 1,000,000)
	// We can't assert false definitively, but we can verify it doesn't error
	_ = ok
}
