package support

import (
	"context"
	"errors"
	"testing"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestService_ViewEnforcesScopeAndRedactsInternalEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	supportCase := testCase(t, now)
	supportCase.Events = append(supportCase.Events, Event{
		ID:         platformid.New(),
		CaseID:     supportCase.ID,
		Type:       EventTypeMessage,
		Visibility: VisibilityInternal,
		ActorID:    platformid.New(),
		Body:       "private staff note",
		Metadata:   map[string]string{},
		OccurredAt: now.Add(time.Minute),
	})

	tests := []struct {
		name           string
		access         Access
		expectedEvents int
		expectedError  error
	}{
		{name: "family customer sees visible timeline", access: Access{CanViewFamily: true}, expectedEvents: 1},
		{name: "operations manager sees full timeline", access: Access{CanManage: true}, expectedEvents: 2},
		{name: "unrelated actor is denied", access: Access{}, expectedError: ErrPermissionDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repository := &fakeRepository{storedCase: supportCase}
			authorizer := &fakeAuthorizer{access: tt.access}
			service := newTestService(t, repository, authorizer)

			loaded, err := service.View(t.Context(), platformid.New(), supportCase.ID)
			if !errors.Is(err, tt.expectedError) {
				t.Fatalf("View() error = %v, want %v", err, tt.expectedError)
			}
			if tt.expectedError != nil {
				return
			}
			if len(loaded.Events) != tt.expectedEvents {
				t.Fatalf("View() events = %+v, want %d", loaded.Events, tt.expectedEvents)
			}
			if !tt.access.CanManage && loaded.Events[0].Body == "private staff note" {
				t.Fatal("View() leaked internal event")
			}
		})
	}
}

func TestService_AppendEventRestrictsInternalVisibility(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	supportCase := testCase(t, now)
	tests := []struct {
		name          string
		access        Access
		visibility    Visibility
		expectedError error
	}{
		{
			name:          "customer cannot create internal note",
			access:        Access{CanViewFamily: true},
			visibility:    VisibilityInternal,
			expectedError: ErrPermissionDenied,
		},
		{
			name:       "customer can create visible reply",
			access:     Access{CanViewFamily: true},
			visibility: VisibilityCustomerVisible,
		},
		{
			name:       "manager can create internal note",
			access:     Access{CanManage: true},
			visibility: VisibilityInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repository := &fakeRepository{storedCase: supportCase}
			authorizer := &fakeAuthorizer{access: tt.access}
			service := newTestService(t, repository, authorizer)
			_, err := service.AppendEvent(t.Context(), AppendEventCommand{
				CaseID:          supportCase.ID,
				EventID:         platformid.New(),
				ExpectedVersion: supportCase.Version,
				Visibility:      tt.visibility,
				Body:            "timeline reply",
				ActorID:         platformid.New(),
				OccurredAt:      now.Add(time.Minute),
			})
			if !errors.Is(err, tt.expectedError) {
				t.Fatalf("AppendEvent() error = %v, want %v", err, tt.expectedError)
			}
			if tt.expectedError != nil && repository.appendCalls != 0 {
				t.Fatalf("AppendEvent() repository calls = %d, want 0", repository.appendCalls)
			}
		})
	}
}

func TestService_ChangeStatusEnforcesTransitionAndVersion(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	supportCase := testCase(t, now)
	tests := []struct {
		name            string
		expectedVersion uint64
		status          Status
		expectedError   error
	}{
		{name: "valid investigation", expectedVersion: 1, status: StatusInvestigating},
		{name: "stale version", expectedVersion: 2, status: StatusInvestigating, expectedError: ErrConflict},
		{name: "invalid direct close", expectedVersion: 1, status: StatusClosed, expectedError: ErrInvalidCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repository := &fakeRepository{storedCase: supportCase}
			service := newTestService(t, repository, &fakeAuthorizer{access: Access{CanManage: true}})
			_, err := service.ChangeStatus(t.Context(), ChangeStatusCommand{
				CaseID:          supportCase.ID,
				EventID:         platformid.New(),
				ExpectedVersion: tt.expectedVersion,
				Status:          tt.status,
				ActorID:         platformid.New(),
				Reason:          "triage decision",
				OccurredAt:      now.Add(time.Minute),
			})
			if !errors.Is(err, tt.expectedError) {
				t.Fatalf("ChangeStatus() error = %v, want %v", err, tt.expectedError)
			}
		})
	}
}

func testCase(t *testing.T, now time.Time) Case {
	t.Helper()
	caseID := platformid.New()
	actorID := platformid.New()
	event := Event{
		ID:         platformid.New(),
		CaseID:     caseID,
		Type:       EventTypeOpened,
		Visibility: VisibilityCustomerVisible,
		ActorID:    actorID,
		Body:       "I need help",
		Metadata:   map[string]string{},
		OccurredAt: now,
	}
	supportCase, err := New(NewParams{
		ID:           caseID,
		FamilyID:     platformid.New(),
		OpenedBy:     actorID,
		Category:     CategoryLearning,
		Severity:     SeverityMedium,
		Subject:      "Assignment question",
		OpenedAt:     now,
		SLADueAt:     now.Add(24 * time.Hour),
		InitialEvent: event,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return supportCase
}

func newTestService(
	t *testing.T,
	repository Repository,
	authorizer Authorizer,
) *Service {
	t.Helper()
	service, err := NewService(repository, authorizer, DefaultConfig())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

type fakeAuthorizer struct {
	access      Access
	accessError error
	assignError error
}

func (a *fakeAuthorizer) SupportAccess(
	context.Context,
	string,
	string,
) (Access, error) {
	return a.access, a.accessError
}

func (a *fakeAuthorizer) AuthorizeSupportAssignee(context.Context, string, string) error {
	return a.assignError
}

type fakeRepository struct {
	storedCase  Case
	appendCalls int
}

func (r *fakeRepository) Create(context.Context, CreateMutation) (Case, error) {
	return cloneCase(r.storedCase), nil
}

func (r *fakeRepository) Summary(context.Context, string) (Case, error) {
	summary := cloneCase(r.storedCase)
	summary.Events = []Event{}
	return summary, nil
}

func (r *fakeRepository) Case(_ context.Context, _ string, includeInternal bool) (Case, error) {
	loaded := cloneCase(r.storedCase)
	if !includeInternal {
		loaded = loaded.CustomerView()
	}
	return loaded, nil
}

func (r *fakeRepository) List(context.Context, string, ListOptions) (Page, error) {
	return Page{Items: []Case{cloneCase(r.storedCase)}}, nil
}

func (r *fakeRepository) Assign(_ context.Context, mutation AssignMutation) (Case, error) {
	updated := cloneCase(r.storedCase)
	updated.AssignedTo = mutation.AssigneeID
	updated.Version++
	updated.UpdatedAt = mutation.OccurredAt
	return updated, nil
}

func (r *fakeRepository) ChangeStatus(_ context.Context, mutation StatusMutation) (Case, error) {
	updated := cloneCase(r.storedCase)
	updated.Status = mutation.ToStatus
	updated.Version++
	updated.UpdatedAt = mutation.OccurredAt
	if mutation.ToStatus == StatusResolved || mutation.ToStatus == StatusClosed {
		resolvedAt := mutation.OccurredAt
		updated.ResolvedAt = &resolvedAt
	}
	return updated, nil
}

func (r *fakeRepository) AppendEvent(_ context.Context, mutation EventMutation) (Case, error) {
	r.appendCalls++
	updated := cloneCase(r.storedCase)
	updated.Events = append(updated.Events, cloneEvent(mutation.Event))
	updated.Version++
	updated.UpdatedAt = mutation.Event.OccurredAt
	return updated, nil
}
