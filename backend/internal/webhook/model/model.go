package model

import "time"

type Provider string

const (
	ProviderRazorpay Provider = "razorpay"
	ProviderResend   Provider = "resend"
	ProviderDaily    Provider = "daily"
)

type Headers struct {
	EventID   string
	Signature string
	Timestamp string
}

type Receipt struct {
	ID              string
	Provider        Provider
	ProviderAccount string
	ProviderEventID string
	PayloadHash     []byte
	ReceivedAt      time.Time
}

type SaveResult struct {
	Inserted       bool
	PayloadMatches bool
}

type IngestResult struct {
	Provider        Provider
	ProviderEventID string
	Duplicate       bool
}
