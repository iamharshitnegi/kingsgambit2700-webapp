package model

import "time"

type CreateInput struct {
	GuardianName     string   `json:"guardianName"`
	Email            string   `json:"email,omitempty"`
	Phone            string   `json:"phone,omitempty"`
	StudentAgeBand   string   `json:"studentAgeBand"`
	Experience       string   `json:"experience,omitempty"`
	Rating           *int     `json:"rating,omitempty"`
	Country          string   `json:"country"`
	TimeZone         string   `json:"timeZone"`
	PreferredFormat  string   `json:"preferredFormat,omitempty"`
	PreferredTimes   []string `json:"preferredTimes,omitempty"`
	ContactConsent   bool     `json:"contactConsent"`
	MarketingConsent bool     `json:"marketingConsent,omitempty"`
	Source           string   `json:"source,omitempty"`
	Campaign         string   `json:"campaign,omitempty"`
	Medium           string   `json:"medium,omitempty"`
	Term             string   `json:"term,omitempty"`
	Content          string   `json:"content,omitempty"`
	LandingPage      string   `json:"landingPage,omitempty"`
	Referrer         string   `json:"referrer,omitempty"`
}

type Lead struct {
	ID               string    `json:"id"`
	GuardianName     string    `json:"guardianName"`
	Email            string    `json:"email,omitempty"`
	Phone            string    `json:"phone,omitempty"`
	StudentAgeBand   string    `json:"studentAgeBand"`
	Experience       string    `json:"experience,omitempty"`
	Rating           *int      `json:"rating,omitempty"`
	Country          string    `json:"country"`
	TimeZone         string    `json:"timeZone"`
	PreferredFormat  string    `json:"preferredFormat,omitempty"`
	PreferredTimes   []string  `json:"preferredTimes"`
	ContactConsent   bool      `json:"contactConsent"`
	MarketingConsent bool      `json:"marketingConsent"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Attribution struct {
	Source      string
	Campaign    string
	Medium      string
	Term        string
	Content     string
	LandingPage string
	Referrer    string
	OccurredAt  time.Time
}
