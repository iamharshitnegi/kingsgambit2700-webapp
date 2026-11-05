package model

import "time"

type Slot struct {
	ID             string    `json:"id"`
	StartsAt       time.Time `json:"startsAt"`
	EndsAt         time.Time `json:"endsAt"`
	TimeZone       string    `json:"timeZone"`
	Format         string    `json:"format"`
	AgeBands       []string  `json:"ageBands"`
	LevelSlugs     []string  `json:"levelSlugs"`
	AvailableSeats int       `json:"availableSeats"`
}

type ListFilter struct {
	From     time.Time
	To       time.Time
	Format   string
	AgeBand  string
	TimeZone string
}

type BookingInput struct {
	SlotID         string `json:"slotId"`
	LeadID         string `json:"leadId"`
	GuardianName   string `json:"guardianName"`
	Email          string `json:"email"`
	Phone          string `json:"phone,omitempty"`
	StudentName    string `json:"studentName,omitempty"`
	StudentAgeBand string `json:"studentAgeBand"`
	TimeZone       string `json:"timeZone"`
	ContactConsent bool   `json:"contactConsent"`
	IdempotencyKey string `json:"-"`
}

type Booking struct {
	ID        string    `json:"id"`
	SlotID    string    `json:"slotId"`
	LeadID    string    `json:"leadId"`
	Status    string    `json:"status"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	TimeZone  string    `json:"timeZone"`
	CreatedAt time.Time `json:"createdAt"`
}
