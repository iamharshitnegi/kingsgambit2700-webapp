package model

import "time"

type PasswordRegistration struct {
	TokenHash    []byte
	Email        string
	Username     string
	DisplayName  string
	PasswordHash string
	RedirectPath string
	ExpiresAt    time.Time
}

type PasswordIdentity struct {
	UserID       string
	PasswordHash string
}

type GoogleIdentity struct {
	Subject            string
	Email              string
	DisplayName        string
	EmailAuthoritative bool
}
