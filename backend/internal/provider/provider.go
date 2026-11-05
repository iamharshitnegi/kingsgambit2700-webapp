package provider

import (
	"context"
	"io"
	"time"
)

type EmailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
	Tags     map[string]string
}

type EmailSender interface {
	Send(context.Context, EmailMessage) (string, error)
}

type MeetingRoom struct {
	ExternalID string
	URL        string
	ExpiresAt  time.Time
}

type MeetingTokenRequest struct {
	RoomExternalID       string
	UserID               string
	DisplayName          string
	CanAdminParticipants bool
	ExpiresAt            time.Time
}

type MeetingProvider interface {
	CreateRoom(context.Context, string, time.Time) (MeetingRoom, error)
	IssueToken(context.Context, MeetingTokenRequest) (string, error)
	CloseRoom(context.Context, string) error
}

type CheckoutRequest struct {
	IdempotencyKey string
	CustomerID     string
	AmountMinor    int64
	Currency       string
	Description    string
	Receipt        string
	ReturnURL      string
}

type Checkout struct {
	ProviderID string
	URL        string
	Status     string
}

type PaymentProvider interface {
	CreateCheckout(context.Context, CheckoutRequest) (Checkout, error)
	VerifyWebhook(body, signature []byte) error
}

type ObjectMetadata struct {
	Key         string
	ContentType string
	Size        int64
	SHA256      string
}

type ObjectStore interface {
	Put(context.Context, ObjectMetadata, io.Reader) error
	SignedUploadURL(context.Context, ObjectMetadata, time.Duration) (string, error)
	SignedDownloadURL(context.Context, string, time.Duration) (string, error)
	Delete(context.Context, string) error
}

type CaptchaVerifier interface {
	Verify(context.Context, string, string) error
}
