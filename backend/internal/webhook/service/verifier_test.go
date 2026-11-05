package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
)

func TestRazorpayVerifier_Verify(t *testing.T) {
	t.Parallel()
	body := []byte(`{"account_id":"acc_123","event":"payment.captured"}`)
	verifier, err := NewRazorpayVerifier("secret")
	if err != nil {
		t.Fatalf("NewRazorpayVerifier() error = %v", err)
	}
	signature := hexHMAC([]byte("secret"), body)
	event, err := verifier.Verify(body, model.Headers{EventID: "evt_123", Signature: signature}, time.Time{})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if event.ProviderAccount != "acc_123" || event.ProviderEventID != "evt_123" {
		t.Fatalf("Verify() event = %+v", event)
	}
	_, err = verifier.Verify(append(body, ' '), model.Headers{EventID: "evt_123", Signature: signature}, time.Time{})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered Verify() error = %v, want invalid signature", err)
	}
}

func TestResendVerifier_Verify(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	secret := []byte("01234567890123456789012345678901")
	verifier, err := NewResendVerifier("whsec_"+base64.RawStdEncoding.EncodeToString(secret), 5*time.Minute)
	if err != nil {
		t.Fatalf("NewResendVerifier() error = %v", err)
	}
	body := []byte(`{"type":"email.delivered"}`)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	signed := []byte("msg_123." + timestamp + "." + string(body))
	signature := "v1," + base64.StdEncoding.EncodeToString(rawHMAC(secret, signed))
	_, err = verifier.Verify(body, model.Headers{
		EventID:   "msg_123",
		Timestamp: timestamp,
		Signature: "v1,AAAA " + signature,
	}, now)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	_, err = verifier.Verify(body, model.Headers{
		EventID:   "msg_123",
		Timestamp: strconv.FormatInt(now.Add(-6*time.Minute).Unix(), 10),
		Signature: signature,
	}, now)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("stale Verify() error = %v, want invalid signature", err)
	}
}

func TestDailyVerifier_Verify(t *testing.T) {
	t.Parallel()
	secret := []byte("daily-webhook-secret")
	verifier, err := NewDailyVerifier(base64.StdEncoding.EncodeToString(secret))
	if err != nil {
		t.Fatalf("NewDailyVerifier() error = %v", err)
	}
	body := []byte(`{"id":"daily-event-1","type":"participant.joined","payload":{"domain_id":"domain-1"}}`)
	timestamp := "1800000000"
	signature := base64.StdEncoding.EncodeToString(rawHMAC(secret, []byte(timestamp+"."+string(body))))
	event, err := verifier.Verify(body, model.Headers{Timestamp: timestamp, Signature: signature}, time.Time{})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if event.ProviderEventID != "daily-event-1" || event.ProviderAccount != "domain-1" {
		t.Fatalf("Verify() event = %+v", event)
	}
}

func rawHMAC(secret, body []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}

func hexHMAC(secret, body []byte) string {
	return hex.EncodeToString(rawHMAC(secret, body))
}
