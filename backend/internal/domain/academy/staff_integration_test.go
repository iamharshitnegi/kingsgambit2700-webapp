//go:build integration

package academy

import (
	"crypto/sha256"
	"errors"
	"slices"
	"testing"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestPostgresStaffDirectoryAndSuspension(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	for _, actor := range []string{f.coach, f.guardian, f.student.UserID} {
		if _, err := f.service.Staff(ctx, actor); !errors.Is(err, ErrDenied) {
			t.Fatalf("non-admin directory access: %v", err)
		}
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,'academic_manager',$3)`, platformid.New(), f.coach, f.now); err != nil {
		t.Fatal(err)
	}
	directory, err := f.service.Staff(ctx, f.root)
	if err != nil || len(directory.Staff) != 2 {
		t.Fatalf("staff directory=%+v error=%v", directory, err)
	}
	var coach StaffRecord
	for _, person := range directory.Staff {
		if person.ID == f.coach {
			coach = person
		}
	}
	if !slices.Equal(coach.Roles, []string{"academic_manager", "coach"}) {
		t.Fatalf("all roles not preserved: %+v", coach)
	}
	device, session := platformid.New(), platformid.New()
	hash := sha256.Sum256([]byte("staff-test-device"))
	if _, err = f.pool.Exec(ctx, `INSERT INTO auth_devices(id,user_id,token_hash,created_at,last_seen_at) VALUES($1::uuid,$2::uuid,$3,$4,$4)`, device, f.coach, hash[:], f.now); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO auth_sessions(id,user_id,device_id,token_hash,csrf_hash,csrf_expires_at,created_at,last_seen_at,idle_expires_at,absolute_expires_at,user_agent_hash,ip_address_hash) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$4,$5,$6,$6,$5,$5,$4,$4)`, session, f.coach, device, hash[:], f.now.Add(time.Hour), f.now); err != nil {
		t.Fatal(err)
	}
	input := StaffUpdate{DisplayName: "Coach Maya updated", Status: "suspended", ExpectedUpdatedAt: coach.UpdatedAt}
	if _, err = f.service.UpdateStaff(ctx, f.coach, f.coach, input); !errors.Is(err, ErrDenied) {
		t.Fatalf("coach edited staff: %v", err)
	}
	suspended, err := f.service.UpdateStaff(ctx, f.root, f.coach, input)
	if err != nil || suspended.Status != "suspended" || !slices.Equal(suspended.Roles, coach.Roles) || !suspended.UpdatedAt.After(coach.UpdatedAt) {
		t.Fatalf("suspend=%+v error=%v", suspended, err)
	}
	if _, err = f.service.UpdateStaff(ctx, f.root, f.coach, input); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update=%v", err)
	}
	input.Status, input.ExpectedUpdatedAt = "active", suspended.UpdatedAt
	reactivated, err := f.service.UpdateStaff(ctx, f.root, f.coach, input)
	if err != nil || reactivated.Status != "active" || !slices.Equal(reactivated.Roles, coach.Roles) {
		t.Fatalf("reactivation=%+v error=%v", reactivated, err)
	}
	var activeDevices, activeSessions, auditCount int
	if err = f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auth_devices WHERE user_id=$1::uuid AND revoked_at IS NULL),(SELECT count(*) FROM auth_sessions WHERE user_id=$1::uuid AND revoked_at IS NULL),(SELECT count(*) FROM audit_events WHERE object_id=$1::text AND action='academy.staff.updated')`, f.coach).Scan(&activeDevices, &activeSessions, &auditCount); err != nil {
		t.Fatal(err)
	}
	if activeDevices != 0 || activeSessions != 0 || auditCount != 2 {
		t.Fatalf("credentials revived or duplicate audit: devices=%d sessions=%d audit=%d", activeDevices, activeSessions, auditCount)
	}
	if _, err = f.service.UpdateStaff(ctx, f.root, f.root, StaffUpdate{DisplayName: "Owner", Status: "suspended", ExpectedUpdatedAt: f.now}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self suspension=%v", err)
	}
	if _, err = f.service.UpdateStaff(ctx, f.root, f.student.UserID, StaffUpdate{DisplayName: "Student", Status: "suspended", ExpectedUpdatedAt: f.now}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-staff target=%v", err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,effect,scope_type,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'academy.staff.manage','deny','global','test explicit deny',$2::uuid,$3)`, platformid.New(), f.root, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Staff(ctx, f.root); !errors.Is(err, ErrDenied) {
		t.Fatalf("explicit deny ignored on directory: %v", err)
	}
	input.ExpectedUpdatedAt = reactivated.UpdatedAt
	if _, err = f.service.UpdateStaff(ctx, f.root, f.coach, input); !errors.Is(err, ErrDenied) {
		t.Fatalf("explicit deny ignored on update: %v", err)
	}
}
