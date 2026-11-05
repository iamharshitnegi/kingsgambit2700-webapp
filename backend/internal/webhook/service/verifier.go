package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
)

const maxSignatureHeaderBytes = 4096

type razorpayVerifier struct {
	secret []byte
}

func NewRazorpayVerifier(secret string) (Verifier, error) {
	if secret == "" {
		return nil, errors.New("webhook verifier: razorpay secret is required")
	}
	return razorpayVerifier{secret: []byte(secret)}, nil
}

func (v razorpayVerifier) Verify(
	body []byte,
	headers model.Headers,
	_ time.Time,
) (VerifiedEvent, error) {
	if headers.EventID == "" || headers.Signature == "" || len(headers.Signature) > maxSignatureHeaderBytes {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	received, err := hex.DecodeString(headers.Signature)
	if err != nil || len(received) == 0 {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(body)
	if !hmac.Equal(received, mac.Sum(nil)) {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	var envelope struct {
		AccountID string `json:"account_id"`
	}
	if err := decodeJSONObject(body, &envelope); err != nil {
		return VerifiedEvent{}, ErrInvalidPayload
	}
	return VerifiedEvent{
		ProviderAccount: envelope.AccountID,
		ProviderEventID: headers.EventID,
	}, nil
}

type resendVerifier struct {
	secret       []byte
	maxClockSkew time.Duration
}

func NewResendVerifier(secret string, maxClockSkew time.Duration) (Verifier, error) {
	if maxClockSkew <= 0 {
		return nil, errors.New("webhook verifier: resend clock skew must be positive")
	}
	decoded, err := decodePrefixedSecret(secret, "whsec_")
	if err != nil || len(decoded) == 0 {
		return nil, errors.New("webhook verifier: invalid resend secret")
	}
	return resendVerifier{secret: decoded, maxClockSkew: maxClockSkew}, nil
}

func (v resendVerifier) Verify(
	body []byte,
	headers model.Headers,
	now time.Time,
) (VerifiedEvent, error) {
	if headers.EventID == "" || headers.Timestamp == "" || headers.Signature == "" ||
		len(headers.Signature) > maxSignatureHeaderBytes {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	timestampSeconds, err := strconv.ParseInt(headers.Timestamp, 10, 64)
	if err != nil || timestampSeconds <= 0 {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	signedAt := time.Unix(timestampSeconds, 0)
	skew := now.Sub(signedAt)
	if skew < -v.maxClockSkew || skew > v.maxClockSkew {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	signedContent := make([]byte, 0, len(headers.EventID)+len(headers.Timestamp)+len(body)+2)
	signedContent = append(signedContent, headers.EventID...)
	signedContent = append(signedContent, '.')
	signedContent = append(signedContent, headers.Timestamp...)
	signedContent = append(signedContent, '.')
	signedContent = append(signedContent, body...)
	if !verifyVersionedBase64HMAC(v.secret, signedContent, headers.Signature) {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	if err := decodeJSONObject(body, &map[string]json.RawMessage{}); err != nil {
		return VerifiedEvent{}, ErrInvalidPayload
	}
	return VerifiedEvent{ProviderEventID: headers.EventID}, nil
}

type dailyVerifier struct {
	secret []byte
}

func NewDailyVerifier(secret string) (Verifier, error) {
	decoded, err := decodeBase64(secret)
	if err != nil || len(decoded) == 0 {
		return nil, errors.New("webhook verifier: invalid daily secret")
	}
	return dailyVerifier{secret: decoded}, nil
}

func (v dailyVerifier) Verify(
	body []byte,
	headers model.Headers,
	_ time.Time,
) (VerifiedEvent, error) {
	if headers.Timestamp == "" || headers.Signature == "" || len(headers.Signature) > maxSignatureHeaderBytes {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	received, err := decodeBase64(headers.Signature)
	if err != nil {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	signedContent := make([]byte, 0, len(headers.Timestamp)+len(body)+1)
	signedContent = append(signedContent, headers.Timestamp...)
	signedContent = append(signedContent, '.')
	signedContent = append(signedContent, body...)
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write(signedContent)
	if !hmac.Equal(received, mac.Sum(nil)) {
		return VerifiedEvent{}, ErrInvalidSignature
	}
	var envelope struct {
		ID      string `json:"id"`
		Payload struct {
			DomainID string `json:"domain_id"`
		} `json:"payload"`
	}
	if err := decodeJSONObject(body, &envelope); err != nil || envelope.ID == "" {
		return VerifiedEvent{}, ErrInvalidPayload
	}
	return VerifiedEvent{
		ProviderAccount: envelope.Payload.DomainID,
		ProviderEventID: envelope.ID,
	}, nil
}

func decodeJSONObject(body []byte, target any) error {
	if len(body) == 0 {
		return ErrInvalidPayload
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return ErrInvalidPayload
	}
	if err := json.Unmarshal(body, target); err != nil {
		return ErrInvalidPayload
	}
	return nil
}

func decodePrefixedSecret(value, prefix string) ([]byte, error) {
	if !strings.HasPrefix(value, prefix) {
		return nil, ErrInvalidSignature
	}
	return decodeBase64(strings.TrimPrefix(value, prefix))
}

func decodeBase64(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}

func verifyVersionedBase64HMAC(secret, content []byte, header string) bool {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(content)
	expected := mac.Sum(nil)
	for _, candidate := range strings.Fields(header) {
		version, encoded, found := strings.Cut(candidate, ",")
		if !found || version != "v1" {
			continue
		}
		received, err := decodeBase64(encoded)
		if err == nil && hmac.Equal(received, expected) {
			return true
		}
	}
	return false
}
