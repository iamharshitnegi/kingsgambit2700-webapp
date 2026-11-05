package fake

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"io"
	"sync"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/provider"
)

type Email struct {
	mu       sync.Mutex
	Messages []provider.EmailMessage
	Fail     error
}

func (e *Email) Send(_ context.Context, message provider.EmailMessage) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Fail != nil {
		return "", e.Fail
	}
	e.Messages = append(e.Messages, message)
	return platformid.New(), nil
}

type Meeting struct {
	mu     sync.Mutex
	Rooms  map[string]provider.MeetingRoom
	Closed map[string]bool
}

func NewMeeting() *Meeting {
	return &Meeting{Rooms: make(map[string]provider.MeetingRoom), Closed: make(map[string]bool)}
}

func (m *Meeting) CreateRoom(_ context.Context, name string, expiresAt time.Time) (provider.MeetingRoom, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	room := provider.MeetingRoom{ExternalID: platformid.New(), URL: "https://meeting.invalid/" + name, ExpiresAt: expiresAt}
	m.Rooms[room.ExternalID] = room
	return room, nil
}

func (m *Meeting) IssueToken(_ context.Context, request provider.MeetingTokenRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Rooms[request.RoomExternalID]; !ok || m.Closed[request.RoomExternalID] {
		return "", errors.New("fake meeting: room unavailable")
	}
	return "meeting_" + platformid.New(), nil
}

func (m *Meeting) CloseRoom(_ context.Context, roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Closed[roomID] = true
	return nil
}

type Payment struct {
	Secret    []byte
	Checkouts []provider.CheckoutRequest
}

func (p *Payment) CreateCheckout(_ context.Context, request provider.CheckoutRequest) (provider.Checkout, error) {
	p.Checkouts = append(p.Checkouts, request)
	id := platformid.New()
	return provider.Checkout{ProviderID: id, URL: "https://payments.invalid/" + id, Status: "created"}, nil
}

func (p *Payment) VerifyWebhook(body, signature []byte) error {
	mac := hmac.New(sha256.New, p.Secret)
	_, _ = mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), signature) {
		return errors.New("fake payment: invalid signature")
	}
	return nil
}

type Store struct {
	mu      sync.Mutex
	Objects map[string][]byte
}

func NewStore() *Store { return &Store{Objects: make(map[string][]byte)} }

func (s *Store) Put(_ context.Context, metadata provider.ObjectMetadata, reader io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(reader, metadata.Size+1))
	if err != nil {
		return err
	}
	if int64(len(data)) != metadata.Size {
		return errors.New("fake store: size mismatch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Objects[metadata.Key] = bytes.Clone(data)
	return nil
}

func (s *Store) SignedUploadURL(_ context.Context, metadata provider.ObjectMetadata, _ time.Duration) (string, error) {
	return "https://objects.invalid/upload/" + metadata.Key, nil
}

func (s *Store) SignedDownloadURL(_ context.Context, key string, _ time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Objects[key]; !ok {
		return "", errors.New("fake store: not found")
	}
	return "https://objects.invalid/download/" + key, nil
}

func (s *Store) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Objects, key)
	return nil
}

type Captcha struct{ Fail error }

func (c Captcha) Verify(_ context.Context, _, _ string) error { return c.Fail }
