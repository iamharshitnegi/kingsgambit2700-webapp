package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/domain/ageband"
	"github.com/kingsgambit2700/platform/backend/internal/domain/trialformat"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/trial/model"
	"github.com/kingsgambit2700/platform/backend/internal/trial/repository"
)

var ErrInvalidInput = errors.New("trial: invalid input")

type Service struct {
	repo repository.Repository
	now  func() time.Time
}

func New(repo repository.Repository) *Service { return &Service{repo: repo, now: time.Now} }

func (s *Service) List(ctx context.Context, filter model.ListFilter) ([]model.Slot, error) {
	now := s.now().UTC()
	if filter.From.IsZero() || filter.From.Before(now) {
		filter.From = now
	}
	if filter.To.IsZero() {
		filter.To = filter.From.Add(30 * 24 * time.Hour)
	}
	if !filter.To.After(filter.From) || filter.To.Sub(filter.From) > 90*24*time.Hour {
		return nil, ErrInvalidInput
	}
	if filter.TimeZone == "" {
		filter.TimeZone = "UTC"
	}
	if _, err := time.LoadLocation(filter.TimeZone); err != nil {
		return nil, ErrInvalidInput
	}
	if filter.AgeBand != "" && !ageband.Valid(filter.AgeBand) {
		return nil, ErrInvalidInput
	}
	if filter.Format != "" && !trialformat.Valid(filter.Format) {
		return nil, ErrInvalidInput
	}
	items, err := s.repo.ListAvailable(ctx, filter)
	if items == nil {
		items = []model.Slot{}
	}
	return items, err
}

func (s *Service) Book(ctx context.Context, input model.BookingInput) (model.Booking, error) {
	input.SlotID = strings.TrimSpace(input.SlotID)
	input.LeadID = strings.TrimSpace(input.LeadID)
	input.GuardianName = strings.TrimSpace(input.GuardianName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	input.StudentName = strings.TrimSpace(input.StudentName)
	input.TimeZone = strings.TrimSpace(input.TimeZone)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !platformid.ValidReference(input.SlotID) || !platformid.ValidReference(input.LeadID) || input.GuardianName == "" || !ageband.Valid(input.StudentAgeBand) || !input.ContactConsent || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 255 {
		return model.Booking{}, ErrInvalidInput
	}
	if address, err := mail.ParseAddress(input.Email); err != nil || !strings.EqualFold(address.Address, input.Email) {
		return model.Booking{}, ErrInvalidInput
	}
	if _, err := time.LoadLocation(input.TimeZone); err != nil {
		return model.Booking{}, ErrInvalidInput
	}
	now := s.now().UTC()
	booking := model.Booking{
		ID:        platformid.New(),
		SlotID:    input.SlotID,
		LeadID:    input.LeadID,
		Status:    "confirmed",
		TimeZone:  input.TimeZone,
		CreatedAt: now,
	}
	return s.repo.CreateBooking(ctx, input, booking)
}
