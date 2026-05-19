package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/alfq/backend/go/internal/common/auth"
)

func TestContextExtractorsEmpty(t *testing.T) {
	ctx := context.Background()
	if v := auth.TenantFromContext(ctx); v != "" {
		t.Fatalf("TenantFromContext empty: got %q", v)
	}
	if v := auth.UserFromContext(ctx); v != "" {
		t.Fatalf("UserFromContext empty: got %q", v)
	}
	if v := auth.RolesFromContext(ctx); v != nil {
		t.Fatalf("RolesFromContext empty: got %v", v)
	}
	if v := auth.EmailFromContext(ctx); v != "" {
		t.Fatalf("EmailFromContext empty: got %q", v)
	}
}

func TestContextInjectExtractRoundtrip(t *testing.T) {
	ctx := context.Background()
	ctx = auth.WithTenant(ctx, "t-42")
	ctx = auth.WithUser(ctx, "u-7")
	ctx = auth.WithRoles(ctx, []string{"trader", "analyst"})
	ctx = auth.WithEmail(ctx, "alice@alfq.io")

	if v := auth.TenantFromContext(ctx); v != "t-42" {
		t.Fatalf("TenantFromContext: got %q, want t-42", v)
	}
	if v := auth.UserFromContext(ctx); v != "u-7" {
		t.Fatalf("UserFromContext: got %q, want u-7", v)
	}
	roles := auth.RolesFromContext(ctx)
	if len(roles) != 2 || roles[0] != "trader" || roles[1] != "analyst" {
		t.Fatalf("RolesFromContext: got %v", roles)
	}
	if v := auth.EmailFromContext(ctx); v != "alice@alfq.io" {
		t.Fatalf("EmailFromContext: got %q", v)
	}
}

func TestValidateTokenPassthrough(t *testing.T) {
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
	keys := map[string]auth.Ed25519PublicKey{kp.Kid: kp.PublicKey}
	parsed, err := auth.ValidateToken(context.Background(), token, keys)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if parsed.Sub != "u-001" {
		t.Fatalf("Sub = %s", parsed.Sub)
	}
}

func TestValidateTokenTampered(t *testing.T) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	claims := auth.Claims{Sub: "u-001"}
	token, err := kp.Sign(claims, 15*time.Minute)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	keys := map[string]auth.Ed25519PublicKey{kp.Kid: kp.PublicKey}
	_, err = auth.ValidateToken(context.Background(), token+"x", keys)
	if err == nil {
		t.Fatal("expected tampered token to fail")
	}
}
