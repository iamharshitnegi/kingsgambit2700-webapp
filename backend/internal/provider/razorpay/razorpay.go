package razorpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/provider"
)

type Client struct {
	keyID         string
	keySecret     string
	webhookSecret []byte
	http          *http.Client
}

func New(keyID, keySecret, webhookSecret string, client *http.Client) (*Client, error) {
	if keyID == "" || keySecret == "" || webhookSecret == "" {
		return nil, errors.New("razorpay: credentials and webhook secret are required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{keyID: keyID, keySecret: keySecret, webhookSecret: []byte(webhookSecret), http: client}, nil
}

func (c *Client) CreateCheckout(ctx context.Context, request provider.CheckoutRequest) (provider.Checkout, error) {
	if request.AmountMinor <= 0 || len(request.Currency) != 3 {
		return provider.Checkout{}, errors.New("razorpay: invalid money")
	}
	payload, err := json.Marshal(map[string]any{
		"amount":   request.AmountMinor,
		"currency": strings.ToUpper(request.Currency),
		"receipt":  request.Receipt,
		"notes": map[string]string{
			"customer_id": request.CustomerID,
			"description": request.Description,
		},
	})
	if err != nil {
		return provider.Checkout{}, fmt.Errorf("razorpay: encoding order: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(payload))
	if err != nil {
		return provider.Checkout{}, fmt.Errorf("razorpay: creating order request: %w", err)
	}
	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Razorpay-Idempotency-Key", request.IdempotencyKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return provider.Checkout{}, fmt.Errorf("razorpay: creating order: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return provider.Checkout{}, fmt.Errorf("razorpay: unexpected status %d", resp.StatusCode)
	}
	var order struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return provider.Checkout{}, fmt.Errorf("razorpay: decoding order: %w", err)
	}
	checkoutURL := "https://api.razorpay.com/v1/checkout/embedded?order_id=" + url.QueryEscape(order.ID)
	return provider.Checkout{ProviderID: order.ID, URL: checkoutURL, Status: order.Status}, nil
}

func (c *Client) VerifyWebhook(body, signature []byte) error {
	decoded := make([]byte, hex.DecodedLen(len(signature)))
	n, err := hex.Decode(decoded, signature)
	if err != nil {
		return errors.New("razorpay: invalid webhook signature encoding")
	}
	mac := hmac.New(sha256.New, c.webhookSecret)
	_, _ = mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), decoded[:n]) {
		return errors.New("razorpay: invalid webhook signature")
	}
	return nil
}
