package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const secretCipherVersion byte = 1

type secretCipher struct {
	aead cipher.AEAD
}

func newSecretCipher(encodedKey string) (*secretCipher, error) {
	if strings.TrimSpace(encodedKey) == "" {
		return nil, nil
	}
	key, err := decodeEncryptionKey(strings.TrimSpace(encodedKey))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("auth mfa: creating secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("auth mfa: creating secret aead: %w", err)
	}
	return &secretCipher{aead: aead}, nil
}

func decodeEncryptionKey(raw string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		key, err := encoding.DecodeString(raw)
		if err == nil && len(key) == 32 {
			return key, nil
		}
	}
	return nil, errors.New("auth mfa: encryption key must be base64-encoded 32-byte key material")
}

func (c *secretCipher) encrypt(plaintext []byte, aad string) ([]byte, error) {
	if c == nil {
		return nil, ErrMFAUnavailable
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("auth mfa: generating encryption nonce: %w", err)
	}
	result := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	result[0] = secretCipherVersion
	result = append(result, nonce...)
	result = c.aead.Seal(result, nonce, plaintext, []byte(aad))
	return result, nil
}

func (c *secretCipher) decrypt(ciphertext []byte, aad string) ([]byte, error) {
	if c == nil {
		return nil, ErrMFAUnavailable
	}
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < 1+nonceSize+c.aead.Overhead() || ciphertext[0] != secretCipherVersion {
		return nil, errors.New("auth mfa: invalid encrypted secret")
	}
	nonce := ciphertext[1 : 1+nonceSize]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext[1+nonceSize:], []byte(aad))
	if err != nil {
		return nil, errors.New("auth mfa: decrypting secret")
	}
	return plaintext, nil
}

func newRecoveryCode() (display, normalized string, err error) {
	value := make([]byte, 15)
	if _, err := rand.Read(value); err != nil {
		return "", "", fmt.Errorf("auth mfa: generating recovery code: %w", err)
	}
	normalized = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value)
	groups := make([]string, 0, len(normalized)/4)
	for start := 0; start < len(normalized); start += 4 {
		end := min(start+4, len(normalized))
		groups = append(groups, normalized[start:end])
	}
	return strings.Join(groups, "-"), normalized, nil
}

func normalizeRecoveryCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func secretAAD(userID, methodID string) string {
	return "kg2700:totp:" + userID + ":" + methodID
}
