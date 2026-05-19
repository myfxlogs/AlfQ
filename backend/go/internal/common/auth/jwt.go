// Package auth provides JWT creation and validation using Ed25519.
package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims holds JWT claims with tenant and role information.
type Claims struct {
	Sub      string   `json:"sub"`
	TenantID string   `json:"tenant_id"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	Iat      int64    `json:"iat"`
	Exp      int64    `json:"exp"`
}

// KeyPair holds an Ed25519 key pair with a kid identifier.
type KeyPair struct {
	Kid       string
	PublicKey ed25519.PublicKey
	secretKey ed25519.PrivateKey
}

// GenerateKeyPair creates a new Ed25519 key pair with a random kid.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("auth: generate key: %w", err)
	}
	kidBytes := make([]byte, 8)
	if _, err := rand.Read(kidBytes); err != nil {
		return nil, fmt.Errorf("auth: generate kid: %w", err)
	}
	return &KeyPair{
		Kid:       fmt.Sprintf("k-%x", kidBytes),
		PublicKey: pub,
		secretKey: priv,
	}, nil
}

// Sign creates a signed JWT string with the given claims and TTL.
func (kp *KeyPair) Sign(claims Claims, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims.Iat = now.Unix()
	claims.Exp = now.Add(ttl).Unix()

	header := jwtHeader{Alg: "EdDSA", Typ: "JWT", Kid: kp.Kid}
	return signEdDSA(header, claims, kp.secretKey)
}

// Verify parses and validates a JWT token using the given key set.
// Returns the claims if the token is valid.
func Verify(tokenString string, keys map[string]ed25519.PublicKey) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("auth: invalid token format")
	}

	// Decode header
	headerJSON, err := b64Decode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("auth: header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("auth: header json: %w", err)
	}
	if header.Alg != "EdDSA" {
		return nil, fmt.Errorf("auth: unsupported alg: %s", header.Alg)
	}

	// Decode claims
	claimsJSON, err := b64Decode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("auth: claims: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("auth: claims json: %w", err)
	}

	// Find the right key
	signingInput := parts[0] + "." + parts[1]
	sig, err := b64Decode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("auth: signature: %w", err)
	}

	// Try the key by kid, then fall back to iterating
	if header.Kid != "" {
		if pk, ok := keys[header.Kid]; ok {
			if ed25519.Verify(pk, []byte(signingInput), sig) {
				return &claims, nil
			}
		}
	}
	// Fallback: try all keys
	for _, pk := range keys {
		if ed25519.Verify(pk, []byte(signingInput), sig) {
			return &claims, nil
		}
	}
	return nil, errors.New("auth: signature verification failed")
}

// IsExpired checks if the claims have expired.
func (c *Claims) IsExpired() bool {
	return time.Now().UTC().Unix() > c.Exp
}

// jwtHeader is the standard JWT header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

func signEdDSA(header jwtHeader, claims Claims, priv ed25519.PrivateKey) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	h := b64Encode(headerJSON)
	c := b64Encode(claimsJSON)
	signingInput := h + "." + c
	sig := ed25519.Sign(priv, []byte(signingInput))
	return signingInput + "." + b64Encode(sig), nil
}

func b64Encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func b64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
