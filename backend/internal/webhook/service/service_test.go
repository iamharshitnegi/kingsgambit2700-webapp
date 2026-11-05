package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
)

func TestService_Ingest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.FixedZone("IST", 5*60*60+30*60))
	verificationErr := errors.New("verification failed")
	tests := []struct {
		name          string
		verifier      *fakeVerifier
		repository    *fakeRepository
		expectedError error
		expectedDup   bool
	}{
		{
			name:       "stores first delivery",
			verifier:   &fakeVerifier{event: VerifiedEvent{ProviderAccount: "acc_1", ProviderEventID: "evt_1"}},
			repository: &fakeRepository{result: model.SaveResult{Inserted: true, PayloadMatches: true}},
		},
		{
			name:        "acknowledges identical replay",
			verifier:    &fakeVerifier{event: VerifiedEvent{ProviderEventID: "evt_1"}},
			repository:  &fakeRepository{result: model.SaveResult{PayloadMatches: true}},
			expectedDup: true,
		},
		{
			name:          "rejects event id reused for another body",
			verifier:      &fakeVerifier{event: VerifiedEvent{ProviderEventID: "evt_1"}},
			repository:    &fakeRepository{result: model.SaveResult{}},
			expectedError: ErrEventConflict,
		},
		{
			name:          "does not store unverified delivery",
			verifier:      &fakeVerifier{err: verificationErr},
			repository:    &fakeRepository{},
			expectedError: verificationErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := mustService(t, test.repository, test.verifier, now)
			result, err := service.Ingest(
				t.Context(),
				model.ProviderRazorpay,
				[]byte("{\"event\":\"payment.captured\"}"),
				model.Headers{EventID: "evt_1"},
			)
			if !errors.Is(err, test.expectedError) {
				t.Fatalf("Ingest() error = %v, want %v", err, test.expectedError)
			}
			if result.Duplicate != test.expectedDup {
				t.Fatalf("Ingest() duplicate = %v, want %v", result.Duplicate, test.expectedDup)
			}
			if test.expectedError == verificationErr && test.repository.calls != 0 {
				t.Fatalf("repository calls = %d, want 0", test.repository.calls)
			}
			if test.repository.calls == 1 {
				if test.repository.receipt.ReceivedAt != now.UTC() {
					t.Fatalf("receivedAt = %v, want %v", test.repository.receipt.ReceivedAt, now.UTC())
				}
				if len(test.repository.receipt.PayloadHash) != 32 {
					t.Fatalf("payload hash length = %d, want 32", len(test.repository.receipt.PayloadHash))
				}
			}
		})
	}
}

func mustService(t *testing.T, repo *fakeRepository, verifier Verifier, now time.Time) *Service {
	t.Helper()
	service, err := New(Dependencies{
		Repository: repo,
		Verifiers: map[model.Provider]Verifier{
			model.ProviderRazorpay: verifier,
			model.ProviderResend:   verifier,
			model.ProviderDaily:    verifier,
		},
		NewID: func() string { return "0198f8c4-0c80-7000-8000-000000000001" },
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}

type fakeVerifier struct {
	event VerifiedEvent
	err   error
}

func (v *fakeVerifier) Verify([]byte, model.Headers, time.Time) (VerifiedEvent, error) {
	return v.event, v.err
}

type fakeRepository struct {
	result  model.SaveResult
	err     error
	receipt model.Receipt
	calls   int
}

func (r *fakeRepository) Save(_ context.Context, receipt model.Receipt) (model.SaveResult, error) {
	r.calls++
	r.receipt = receipt
	return r.result, r.err
}
