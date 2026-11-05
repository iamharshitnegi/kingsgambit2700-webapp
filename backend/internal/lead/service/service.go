package service

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/domain/ageband"
	"github.com/kingsgambit2700/platform/backend/internal/domain/trialformat"
	"github.com/kingsgambit2700/platform/backend/internal/lead/model"
	"github.com/kingsgambit2700/platform/backend/internal/lead/repository"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var ErrInvalidInput = errors.New("lead: invalid input")

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

type Service struct {
	repo repository.Repository
	now  func() time.Time
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Create(ctx context.Context, input model.CreateInput) (model.Lead, bool, error) {
	input.GuardianName = strings.TrimSpace(input.GuardianName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = normalizePhone(input.Phone)
	input.StudentAgeBand = strings.TrimSpace(input.StudentAgeBand)
	input.Country = strings.ToUpper(strings.TrimSpace(input.Country))
	input.TimeZone = strings.TrimSpace(input.TimeZone)
	if input.GuardianName == "" || !ageband.Valid(input.StudentAgeBand) || len(input.Country) != 2 || !input.ContactConsent {
		return model.Lead{}, false, ErrInvalidInput
	}
	if input.Email == "" && input.Phone == "" {
		return model.Lead{}, false, ErrInvalidInput
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || !strings.EqualFold(address.Address, input.Email) {
			return model.Lead{}, false, ErrInvalidInput
		}
	}
	if input.Phone != "" && !phonePattern.MatchString(input.Phone) {
		return model.Lead{}, false, ErrInvalidInput
	}
	if input.PreferredFormat != "" && !trialformat.Valid(input.PreferredFormat) {
		return model.Lead{}, false, ErrInvalidInput
	}
	if _, err := time.LoadLocation(input.TimeZone); err != nil {
		return model.Lead{}, false, ErrInvalidInput
	}
	if input.Rating != nil && (*input.Rating < 0 || *input.Rating > 4000) {
		return model.Lead{}, false, ErrInvalidInput
	}
	now := s.now().UTC()
	lead := model.Lead{
		ID:               platformid.New(),
		GuardianName:     input.GuardianName,
		Email:            input.Email,
		Phone:            input.Phone,
		StudentAgeBand:   input.StudentAgeBand,
		Experience:       strings.TrimSpace(input.Experience),
		Rating:           input.Rating,
		Country:          input.Country,
		TimeZone:         input.TimeZone,
		PreferredFormat:  strings.TrimSpace(input.PreferredFormat),
		PreferredTimes:   nonNil(input.PreferredTimes),
		ContactConsent:   input.ContactConsent,
		MarketingConsent: input.MarketingConsent,
		Status:           "new",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	attribution := model.Attribution{
		Source:      trim(input.Source, 128),
		Campaign:    trim(input.Campaign, 128),
		Medium:      trim(input.Medium, 128),
		Term:        trim(input.Term, 128),
		Content:     trim(input.Content, 128),
		LandingPage: trim(input.LandingPage, 2048),
		Referrer:    trim(input.Referrer, 2048),
		OccurredAt:  now,
	}
	return s.repo.Upsert(ctx, lead, attribution)
}

func normalizePhone(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for i, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
			continue
		}
		if r == '+' && i == 0 {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func trim(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) > max {
		return value[:max]
	}
	return value
}
