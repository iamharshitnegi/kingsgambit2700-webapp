package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyWebhook(t *testing.T) {
	client, err := New("key", "secret", "webhook", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	body := []byte(`{"event":"payment.captured"}`)
	mac := hmac.New(sha256.New, []byte("webhook"))
	_, _ = mac.Write(body)
	signature := []byte(hex.EncodeToString(mac.Sum(nil)))
	if err := client.VerifyWebhook(body, signature); err != nil {
		t.Fatalf("VerifyWebhook() error = %v", err)
	}
	if err := client.VerifyWebhook(body, []byte("00")); err == nil {
		t.Fatal("VerifyWebhook() accepted wrong signature")
	}
}
