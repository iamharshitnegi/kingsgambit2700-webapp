// Package password hashes passwords with salted Argon2id.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const prefix = "$argon2id$v=19$m=19456,t=2,p=1$"

func Valid(value string) bool {
	length := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && length >= 12 && length <= 128 && len(value) <= 512
}

func Hash(value string) (string, error) {
	if !Valid(value) {
		return "", errors.New("password must contain between 12 and 128 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(value), salt, 2, 19456, 1, 32)
	return prefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func Verify(encoded, value string) bool {
	if len(value) > 512 || !strings.HasPrefix(encoded, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, prefix), "$")
	if len(parts) != 2 {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[0])
	want, keyErr := base64.RawStdEncoding.DecodeString(parts[1])
	if saltErr != nil || keyErr != nil || len(salt) != 16 || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(value), salt, 2, 19456, 1, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// DummyHash performs the same work for unknown identities as for known users.
const DummyHash = prefix + "AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
