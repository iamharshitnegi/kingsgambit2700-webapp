package daily

import (
	"bytes"
	"context"
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
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(apiKey, baseURL string, client *http.Client) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("daily: api key is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("daily: invalid base url: %w", err)
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), http: client}, nil
}

func (c *Client) CreateRoom(ctx context.Context, name string, expiresAt time.Time) (provider.MeetingRoom, error) {
	payload := map[string]any{
		"name":    name,
		"privacy": "private",
		"properties": map[string]any{
			"exp":                          expiresAt.Unix(),
			"enable_recording":             "off",
			"enable_recording_ui":          false,
			"enable_live_captions_ui":      false,
			"enable_transcription_storage": false,
			"auto_start_transcription":     false,
			"enable_chat":                  false,
			"enable_knocking":              true,
			"enable_prejoin_ui":            true,
			"start_video_off":              false,
		},
	}
	var response struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/rooms", payload, &response); err != nil {
		return provider.MeetingRoom{}, err
	}
	return provider.MeetingRoom{ExternalID: response.Name, URL: response.URL, ExpiresAt: expiresAt}, nil
}

func (c *Client) IssueToken(ctx context.Context, request provider.MeetingTokenRequest) (string, error) {
	var canAdmin any = false
	if request.CanAdminParticipants {
		canAdmin = []string{"participants"}
	}
	payload := map[string]any{
		"properties": map[string]any{
			"room_name":                request.RoomExternalID,
			"user_id":                  request.UserID,
			"user_name":                request.DisplayName,
			"exp":                      request.ExpiresAt.Unix(),
			"enable_recording":         "off",
			"enable_recording_ui":      false,
			"enable_live_captions_ui":  false,
			"auto_start_transcription": false,
			"permissions": map[string]any{
				"canAdmin": canAdmin,
			},
		},
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/meeting-tokens", payload, &response); err != nil {
		return "", err
	}
	return response.Token, nil
}

func (c *Client) CloseRoom(ctx context.Context, roomID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/rooms/"+url.PathEscape(roomID), nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, payload, target any) error {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return fmt.Errorf("daily: encoding request: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, &body)
	if err != nil {
		return fmt.Errorf("daily: creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("daily: sending request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("daily: unexpected status %d", resp.StatusCode)
	}
	if target == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("daily: decoding response: %w", err)
	}
	return nil
}
