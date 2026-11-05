package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/provider"
)

type Client struct {
	apiKey string
	from   string
	http   *http.Client
}

func New(apiKey, from string, client *http.Client) (*Client, error) {
	if apiKey == "" || from == "" {
		return nil, errors.New("resend: api key and from address are required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{apiKey: apiKey, from: from, http: client}, nil
}

func (c *Client) Send(ctx context.Context, message provider.EmailMessage) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"from":    c.from,
		"to":      []string{message.To},
		"subject": message.Subject,
		"text":    message.TextBody,
		"html":    message.HTMLBody,
	})
	if err != nil {
		return "", fmt.Errorf("resend: encoding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("resend: creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if key := message.Tags["notification_id"]; key != "" {
		req.Header.Set("Idempotency-Key", "academy-notification-"+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("resend: sending request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("resend: unexpected status %d", resp.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("resend: decoding response: %w", err)
	}
	return result.ID, nil
}
