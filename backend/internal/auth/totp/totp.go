package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // TOTP interoperability requires HMAC-SHA1 by default (RFC 6238).
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	period = 30 * time.Second
	digits = 6
)

func NewSecret() (string, error) {
	value := make([]byte, 20)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("totp: generating secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(value), nil
}

// Generate returns the code for a specific instant. Production authentication
// should call MatchCounter so the accepted step can be persisted against replay.
func Generate(secret string, now time.Time) (string, error) {
	return generate(secret, now)
}

func Verify(secret, code string, now time.Time) bool {
	_, ok := MatchCounter(secret, code, now)
	return ok
}

// MatchCounter returns the accepted RFC 6238 time-step. Persisting this value
// lets callers reject replay of a valid code within its time window.
func MatchCounter(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != digits {
		return 0, false
	}
	for offset := -1; offset <= 1; offset++ {
		candidateTime := now.Add(time.Duration(offset) * period)
		candidate, err := generate(secret, candidateTime)
		if err == nil && subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return candidateTime.Unix() / int64(period/time.Second), true
		}
	}
	return 0, false
}

func generate(secret string, now time.Time) (string, error) {
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("totp: decoding secret: %w", err)
	}
	counter := uint64(now.Unix() / int64(period/time.Second))
	message := make([]byte, 8)
	binary.BigEndian.PutUint64(message, counter)
	mac := hmac.New(sha1.New, decoded)
	if _, err := mac.Write(message); err != nil {
		return "", fmt.Errorf("totp: hashing counter: %w", err)
	}
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	value := int(binaryCode % 1_000_000)
	return strings.Repeat("0", digits-len(strconv.Itoa(value))) + strconv.Itoa(value), nil
}
