package academy

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type StaffRecord struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Status      string    `json:"status"`
	Roles       []string  `json:"roles"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type StaffDirectory struct {
	Staff []StaffRecord `json:"staff"`
}

type StaffUpdate struct {
	DisplayName       string    `json:"displayName"`
	Status            string    `json:"status"`
	ExpectedUpdatedAt time.Time `json:"expectedUpdatedAt"`
}

func (s *Service) Staff(ctx context.Context, actor string) (StaffDirectory, error) {
	return s.repository.Staff(ctx, actor)
}

func (s *Service) UpdateStaff(ctx context.Context, actor, target string, in StaffUpdate) (StaffRecord, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if validID(target) != nil || !validText(in.DisplayName, 2, 100) ||
		!slices.Contains([]string{"active", "suspended"}, in.Status) || in.ExpectedUpdatedAt.IsZero() {
		return StaffRecord{}, invalid("staff ID, display name, active or suspended status and current update time are required")
	}
	return s.repository.UpdateStaff(ctx, actor, target, in, s.now().UTC())
}

func requireStaffManager(ctx context.Context, q querier, actor string) error {
	if err := requirePermission(ctx, q, actor, "academy.staff.manage"); err != nil {
		return err
	}
	admin, err := role(ctx, q, actor, "super_admin")
	if err != nil {
		return err
	}
	if !admin {
		return ErrDenied
	}
	return nil
}

const staffColumns = `u.id::text,u.display_name,COALESCE(u.email::text,''),u.status,
 ARRAY(SELECT g.role FROM role_grants g WHERE g.user_id=u.id AND g.revoked_at IS NULL ORDER BY g.role),u.created_at,u.updated_at`

const isStaff = `EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=u.id AND g.revoked_at IS NULL AND g.role IN('coach','academic_manager','operations_admin','finance_admin','super_admin'))`

func scanStaff(row pgx.Row) (StaffRecord, error) {
	var out StaffRecord
	err := row.Scan(&out.ID, &out.DisplayName, &out.Email, &out.Status, &out.Roles, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *PostgresRepository) Staff(ctx context.Context, actor string) (StaffDirectory, error) {
	out := StaffDirectory{Staff: []StaffRecord{}}
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requireStaffManager(ctx, tx, actor); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+staffColumns+` FROM users u WHERE `+isStaff+` ORDER BY lower(u.display_name),u.id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			staff, err := scanStaff(rows)
			if err != nil {
				return err
			}
			out.Staff = append(out.Staff, staff)
		}
		return rows.Err()
	})
	return out, err
}

func (r *PostgresRepository) UpdateStaff(ctx context.Context, actor, target string, in StaffUpdate, now time.Time) (StaffRecord, error) {
	var out StaffRecord
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requireStaffManager(ctx, tx, actor); err != nil {
			return err
		}
		if actor == target && in.Status == "suspended" {
			return invalid("ask another academy owner to suspend your account")
		}
		old, err := scanStaff(tx.QueryRow(ctx, `SELECT `+staffColumns+` FROM users u WHERE u.id=$1::uuid AND `+isStaff+` FOR UPDATE OF u`, target))
		if err != nil {
			return err
		}
		if !old.UpdatedAt.Equal(in.ExpectedUpdatedAt) {
			return ErrConflict
		}
		if old.Status != "active" && old.Status != "suspended" {
			return invalid("only active or suspended staff accounts can be edited here")
		}
		// Maintain a distinct version even when an injected clock or rapid edits
		// share PostgreSQL's microsecond precision.
		now = now.Truncate(time.Microsecond)
		if !now.After(old.UpdatedAt) {
			now = old.UpdatedAt.Add(time.Microsecond)
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET display_name=$2,status=$3,updated_at=$4 WHERE id=$1::uuid`, target, in.DisplayName, in.Status, now); err != nil {
			return err
		}
		if in.Status == "suspended" {
			// Share the user row lock used by remember/restore. Reactivation must
			// never reactivate credentials belonging to a suspended account.
			if _, err = tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=$2 WHERE user_id=$1::uuid AND revoked_at IS NULL`, target, now); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=$2 WHERE user_id=$1::uuid AND revoked_at IS NULL`, target, now); err != nil {
				return err
			}
		}
		out = old
		out.DisplayName, out.Status, out.UpdatedAt = in.DisplayName, in.Status, now
		return audit(ctx, tx, actor, "academy.staff.updated", target, now, map[string]any{
			"previousStatus": old.Status, "status": in.Status, "displayNameChanged": old.DisplayName != in.DisplayName,
		})
	})
	return out, err
}
