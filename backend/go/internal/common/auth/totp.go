package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

const (
	totpDigits   = 6
	totpStepSecs = 30
)

// GenerateTOTPSecret creates a new base32-encoded TOTP secret.
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: totp secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// VerifyTOTP checks if the given code is valid for the secret at the current time step.
// Accepts codes from the current and immediately preceding time step (window=1).
func VerifyTOTP(secret, code string) (bool, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return false, fmt.Errorf("auth: totp decode secret: %w", err)
	}
	now := time.Now().UTC().Unix()
	// Check current and previous step
	for _, step := range []int64{now / totpStepSecs, now/totpStepSecs - 1} {
		if computeTOTP(key, step) == code {
			return true, nil
		}
	}
	return false, nil
}

// computeTOTP generates a TOTP code for the given secret and time step.
func computeTOTP(key []byte, step int64) string {
	mac := hmac.New(sha1.New, key)
	binary.Write(mac, binary.BigEndian, step)
	hash := mac.Sum(nil)
	offset := hash[len(hash)-1] & 0x0f
	binary := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	code := binary % uint32(math.Pow10(totpDigits))
	return fmt.Sprintf("%0*d", totpDigits, code)
}
