package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestSecretCipherRoundTripAndAuthenticatesBinding(t *testing.T) {
	t.Parallel()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	cipher, err := newSecretCipher(key)
	if err != nil {
		t.Fatalf("newSecretCipher() error = %v", err)
	}
	ciphertext, err := cipher.encrypt([]byte("SECRET"), "user:method")
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	if strings.Contains(string(ciphertext), "SECRET") {
		t.Fatal("ciphertext contains the plaintext secret")
	}
	plaintext, err := cipher.decrypt(ciphertext, "user:method")
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}
	if string(plaintext) != "SECRET" {
		t.Fatalf("decrypt() = %q, want SECRET", plaintext)
	}
	if _, err := cipher.decrypt(ciphertext, "other:method"); err == nil {
		t.Fatal("decrypt() accepted ciphertext under different associated data")
	}
}

func TestNewRecoveryCodeHasHighEntropyCanonicalForm(t *testing.T) {
	t.Parallel()
	display, normalized, err := newRecoveryCode()
	if err != nil {
		t.Fatalf("newRecoveryCode() error = %v", err)
	}
	if len(normalized) != 24 {
		t.Fatalf("normalized code length = %d, want 24", len(normalized))
	}
	if got := normalizeRecoveryCode(display); got != normalized {
		t.Fatalf("normalizeRecoveryCode() = %q, want %q", got, normalized)
	}
}
