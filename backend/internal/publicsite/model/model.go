package model

import "time"

type Organization struct {
	Name         string            `json:"name"`
	LegalName    string            `json:"legalName,omitempty"`
	Email        string            `json:"email,omitempty"`
	Phone        string            `json:"phone,omitempty"`
	WhatsApp     string            `json:"whatsApp,omitempty"`
	Address      string            `json:"address,omitempty"`
	CanonicalURL string            `json:"canonicalUrl"`
	SocialLinks  map[string]string `json:"socialLinks"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

type PolicySummary struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Version     string    `json:"version"`
	EffectiveAt time.Time `json:"effectiveAt"`
	Purpose     string    `json:"purpose"`
}

type Policy struct {
	PolicySummary
	ContentHTML string `json:"contentHtml"`
	ContentHash string `json:"contentHash"`
	Locale      string `json:"locale"`
}

type Money struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	Interval    string `json:"interval,omitempty"`
}

type Program struct {
	ID              string   `json:"id"`
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Summary         string   `json:"summary"`
	Formats         []string `json:"formats"`
	AgeBands        []string `json:"ageBands"`
	LevelSlugs      []string `json:"levelSlugs"`
	Capacity        *int     `json:"capacity,omitempty"`
	DurationMinutes *int     `json:"durationMinutes,omitempty"`
	StartingPrice   *Money   `json:"startingPrice,omitempty"`
}

type Coach struct {
	ID           string     `json:"id"`
	Slug         string     `json:"slug"`
	DisplayName  string     `json:"displayName"`
	Bio          string     `json:"bio"`
	Languages    []string   `json:"languages"`
	Title        string     `json:"title,omitempty"`
	Rating       *int       `json:"rating,omitempty"`
	RatingSource string     `json:"ratingSource,omitempty"`
	PhotoURL     string     `json:"photoUrl,omitempty"`
	VerifiedAt   *time.Time `json:"verifiedAt,omitempty"`
}

type Level struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	AgeGuidance    string   `json:"ageGuidance,omitempty"`
	RatingGuidance string   `json:"ratingGuidance,omitempty"`
	Outcomes       []string `json:"outcomes"`
}

type Track struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Summary string  `json:"summary"`
	Levels  []Level `json:"levels"`
}

type Curriculum struct {
	Version string  `json:"version"`
	Tracks  []Track `json:"tracks"`
}

type Price struct {
	PlanID        string   `json:"planId"`
	ProgramSlug   string   `json:"programSlug"`
	Name          string   `json:"name"`
	Format        string   `json:"format"`
	BillingModel  string   `json:"billingModel"`
	AmountMinor   int64    `json:"amountMinor"`
	Interval      string   `json:"interval,omitempty"`
	ClassQuantity *int     `json:"classQuantity,omitempty"`
	Capacity      *int     `json:"capacity,omitempty"`
	TaxInclusive  bool     `json:"taxInclusive"`
	Features      []string `json:"features"`
}
