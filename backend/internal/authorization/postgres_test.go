package authorization

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	authmodel "github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	"github.com/kingsgambit2700/platform/backend/internal/domain/support"
)

const (
	testActorID   = "018f1f00-0000-7000-8000-000000000001"
	testFamilyID  = "018f1f00-0000-7000-8000-000000000002"
	testStudentID = "018f1f00-0000-7000-8000-000000000003"
	testTargetID  = "018f1f00-0000-7000-8000-000000000004"
)

func TestAuthorizeMFAResetRequiresDifferentActiveStaffTarget(t *testing.T) {
	t.Parallel()

	actor := authmodel.Principal{User: authmodel.User{ID: testActorID}}
	tests := []struct {
		name      string
		targetID  string
		responses [][]bool
		wantError error
	}{
		{name: "active super admin and staff target", targetID: testTargetID, responses: [][]bool{{true}}},
		{name: "actor lacks current super admin role", targetID: testTargetID, responses: [][]bool{{false}}, wantError: ErrMFAResetDenied},
		{name: "self reset", targetID: testActorID, wantError: ErrMFAResetDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &booleanQuerier{responses: test.responses}
			authorizer := newPostgres(database)
			err := authorizer.AuthorizeMFAReset(t.Context(), actor, test.targetID)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("AuthorizeMFAReset() error = %v, want %v", err, test.wantError)
			}
			if database.remaining() != 0 {
				t.Fatalf("unused database responses = %d", database.remaining())
			}
		})
	}
}

func TestAuthorizeRelationshipManagement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses [][]bool
		wantError error
	}{
		{name: "primary guardian", responses: [][]bool{{true}, {true}, {false, false}, {true}}},
		{name: "staff permission", responses: [][]bool{{true}, {true}, {true, false}}},
		{name: "explicit deny overrides guardian", responses: [][]bool{{true}, {true}, {false, true}}, wantError: family.ErrPermissionDenied},
		{name: "unscoped guardian", responses: [][]bool{{true}, {true}, {false, false}, {false}}, wantError: family.ErrPermissionDenied},
		{name: "suspended actor", responses: [][]bool{{false}}, wantError: family.ErrPermissionDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &booleanQuerier{responses: test.responses}
			authorizer := newPostgres(database)
			err := authorizer.AuthorizeRelationshipManagement(
				t.Context(),
				testActorID,
				testFamilyID,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("AuthorizeRelationshipManagement() error = %v, want %v", err, test.wantError)
			}
			if database.remaining() != 0 {
				t.Fatalf("unused database responses = %d", database.remaining())
			}
		})
	}
}

func TestAuthorizeReportUsesRelationshipAndAssignmentScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		action    report.Action
		responses [][]bool
		wantError error
	}{
		{name: "assigned coach drafts", action: report.ActionCreateDraft, responses: [][]bool{{true}, {false, false}, {true}}},
		{name: "explicit deny overrides coach assignment", action: report.ActionCreateDraft, responses: [][]bool{{true}, {false, true}}, wantError: report.ErrPermissionDenied},
		{name: "coach cannot publish", action: report.ActionPublish, responses: [][]bool{{true}, {false, false}}, wantError: report.ErrPermissionDenied},
		{name: "guardian views", action: report.ActionView, responses: [][]bool{{true}, {false, false}, {false}, {false}, {true}}},
		{name: "student cannot acknowledge", action: report.ActionAcknowledge, responses: [][]bool{{true}, {false, false}, {false}}, wantError: report.ErrPermissionDenied},
		{name: "academic manager publishes", action: report.ActionPublish, responses: [][]bool{{true}, {true, false}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &booleanQuerier{responses: test.responses}
			authorizer := newPostgres(database)
			err := authorizer.AuthorizeReport(
				t.Context(),
				testActorID,
				testStudentID,
				test.action,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("AuthorizeReport() error = %v, want %v", err, test.wantError)
			}
			if database.remaining() != 0 {
				t.Fatalf("unused database responses = %d", database.remaining())
			}
		})
	}
}

func TestAuthorizeEntitlementUsesStaffAndGuardianScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		action    entitlement.Action
		responses [][]bool
		wantError error
	}{
		{name: "finance views", action: entitlement.ActionView, responses: [][]bool{{true}, {true, false}}},
		{name: "guardian with billing permission views", action: entitlement.ActionView, responses: [][]bool{{true}, {false, false}, {true}}},
		{name: "explicit deny overrides guardian scope", action: entitlement.ActionView, responses: [][]bool{{true}, {false, true}}, wantError: entitlement.ErrPermissionDenied},
		{name: "guardian cannot grant", action: entitlement.ActionGrant, responses: [][]bool{{true}, {false, false}}, wantError: entitlement.ErrPermissionDenied},
		{name: "suspended actor", action: entitlement.ActionView, responses: [][]bool{{false}}, wantError: entitlement.ErrPermissionDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &booleanQuerier{responses: test.responses}
			authorizer := newPostgres(database)
			err := authorizer.AuthorizeEntitlement(
				t.Context(),
				testActorID,
				testStudentID,
				test.action,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("AuthorizeEntitlement() error = %v, want %v", err, test.wantError)
			}
			if database.remaining() != 0 {
				t.Fatalf("unused database responses = %d", database.remaining())
			}
		})
	}
}

func TestSupportAccessSeparatesFamilyVisibilityFromManagement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		responses  [][]bool
		wantAccess support.Access
		wantError  error
	}{
		{name: "operations manager", responses: [][]bool{{true}, {true}, {true, false}, {false}}, wantAccess: support.Access{CanViewFamily: true, CanManage: true}},
		{name: "guardian communicates", responses: [][]bool{{true}, {true}, {false, false}, {true}}, wantAccess: support.Access{CanViewFamily: true}},
		{name: "management deny does not erase guardian relationship", responses: [][]bool{{true}, {true}, {false, true}, {true}}, wantAccess: support.Access{CanViewFamily: true}},
		{name: "unscoped actor", responses: [][]bool{{true}, {true}, {false, false}, {false}}, wantError: support.ErrPermissionDenied},
		{name: "suspended actor", responses: [][]bool{{false}}, wantError: support.ErrPermissionDenied},
		{name: "archived family", responses: [][]bool{{true}, {false}}, wantError: support.ErrPermissionDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &booleanQuerier{responses: test.responses}
			authorizer := newPostgres(database)
			access, err := authorizer.SupportAccess(t.Context(), testActorID, testFamilyID)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("SupportAccess() error = %v, want %v", err, test.wantError)
			}
			if access != test.wantAccess {
				t.Fatalf("SupportAccess() = %+v, want %+v", access, test.wantAccess)
			}
			if database.remaining() != 0 {
				t.Fatalf("unused database responses = %d", database.remaining())
			}
		})
	}
}

func TestAuthorizeSupportAssigneeRequiresActiveManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses [][]bool
		wantError error
	}{
		{name: "active manager", responses: [][]bool{{true}, {true}, {true, false}}},
		{name: "explicitly denied target", responses: [][]bool{{true}, {true}, {true, true}}, wantError: support.ErrPermissionDenied},
		{name: "inactive target", responses: [][]bool{{true}, {false}}, wantError: support.ErrPermissionDenied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			database := &booleanQuerier{responses: test.responses}
			authorizer := newPostgres(database)
			err := authorizer.AuthorizeSupportAssignee(
				t.Context(),
				testActorID,
				testTargetID,
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("AuthorizeSupportAssignee() error = %v, want %v", err, test.wantError)
			}
			if database.remaining() != 0 {
				t.Fatalf("unused database responses = %d", database.remaining())
			}
		})
	}
}

type booleanQuerier struct {
	responses [][]bool
	index     int
}

func (q *booleanQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	if q.index >= len(q.responses) {
		return booleanRow{err: errors.New("unexpected query")}
	}
	values := q.responses[q.index]
	q.index++
	return booleanRow{values: values}
}

func (q *booleanQuerier) remaining() int {
	return len(q.responses) - q.index
}

type booleanRow struct {
	values []bool
	err    error
}

func (r booleanRow) Scan(destinations ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(destinations) != len(r.values) {
		return fmt.Errorf("destination count = %d, want %d", len(destinations), len(r.values))
	}
	for index, rawDestination := range destinations {
		destination, ok := rawDestination.(*bool)
		if !ok {
			return fmt.Errorf("destination type = %T, want *bool", rawDestination)
		}
		*destination = r.values[index]
	}
	return nil
}
