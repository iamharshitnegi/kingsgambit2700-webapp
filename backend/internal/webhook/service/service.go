package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
	"github.com/kingsgambit2700/platform/backend/internal/webhook/repository"
)

var (
	ErrInvalidSignature = errors.New("webhook: invalid signature")
	ErrInvalidPayload   = errors.New("webhook: invalid payload")
	ErrEventConflict    = errors.New("webhook: event payload conflict")
)

type VerifiedEvent struct {
	ProviderAccount string
	ProviderEventID string
}

type Verifier interface {
	Verify(body []byte, headers model.Headers, now time.Time) (VerifiedEvent, error)
}

type Dependencies struct {
	Repository repository.Repository
	Verifiers  map[model.Provider]Verifier
	NewID      func() string
	Now        func() time.Time
}

type Service struct {
	repository repository.Repository
	verifiers  map[model.Provider]Verifier
	newID      func() string
	now        func() time.Time
}

func New(dependencies Dependencies) (*Service, error) {
	if dependencies.Repository == nil {
		return nil, errors.New("webhook service: repository is required")
	}
	for _, provider := range []model.Provider{
		model.ProviderRazorpay,
		model.ProviderResend,
		model.ProviderDaily,
	} {
		if dependencies.Verifiers[provider] == nil {
			return nil, fmt.Errorf("webhook service: verifier for %s is required", provider)
		}
	}
	if dependencies.NewID == nil {
		dependencies.NewID = platformid.New
	}
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	return &Service{
		repository: dependencies.Repository,
		verifiers:  dependencies.Verifiers,
		newID:      dependencies.NewID,
		now:        dependencies.Now,
	}, nil
}

func (s *Service) Ingest(
	ctx context.Context,
	provider model.Provider,
	body []byte,
	headers model.Headers,
) (model.IngestResult, error) {
	verifier := s.verifiers[provider]
	if verifier == nil {
		return model.IngestResult{}, ErrInvalidPayload
	}
	receivedAt := s.now().UTC()
	event, err := verifier.Verify(body, headers, receivedAt)
	if err != nil {
		return model.IngestResult{}, err
	}
	if event.ProviderEventID == "" || len(event.ProviderEventID) > 255 || len(event.ProviderAccount) > 255 {
		return model.IngestResult{}, ErrInvalidPayload
	}
	payloadHash := sha256.Sum256(body)
	result, err := s.repository.Save(ctx, model.Receipt{
		ID:              s.newID(),
		Provider:        provider,
		ProviderAccount: event.ProviderAccount,
		ProviderEventID: event.ProviderEventID,
		PayloadHash:     append([]byte(nil), payloadHash[:]...),
		ReceivedAt:      receivedAt,
	})
	if err != nil {
		return model.IngestResult{}, fmt.Errorf("webhook service: saving receipt: %w", err)
	}
	if !result.Inserted && !result.PayloadMatches {
		return model.IngestResult{}, ErrEventConflict
	}
	return model.IngestResult{
		Provider:        provider,
		ProviderEventID: event.ProviderEventID,
		Duplicate:       !result.Inserted,
	}, nil
}
