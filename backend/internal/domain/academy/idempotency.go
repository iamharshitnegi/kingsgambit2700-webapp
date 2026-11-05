package academy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
)

func (r *PostgresRepository) withCreation(ctx context.Context, actor, operation string, input, output any, work func(pgx.Tx) error) error {
	return r.within(ctx, func(tx pgx.Tx) error {
		if err := authorizeCreation(ctx, tx, actor, operation, input); err != nil {
			return err
		}
		found, err := idempotency.Replay(ctx, tx, actor, "academy.create."+operation, input, output)
		if errors.Is(err, idempotency.ErrConflict) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if found {
			if student, ok := output.(*Student); ok {
				return studentPermission(ctx, tx, actor, student.ID, "academy.manage", "manage_profile", false)
			}
			if enrolment, ok := output.(*Enrolment); ok {
				return studentPermission(ctx, tx, actor, enrolment.StudentID, "academy.manage", "", false)
			}
			return nil
		}
		if err = work(tx); err != nil {
			return err
		}
		return idempotency.Save(ctx, tx, actor, "academy.create."+operation, input, output)
	})
}
func authorizeCreation(ctx context.Context, tx pgx.Tx, actor, operation string, input any) error {
	switch operation {
	case "students":
		student := input.(StudentInput)
		if student.GuardianEmail == "" {
			ok, err := role(ctx, tx, actor, "guardian")
			if err != nil {
				return err
			}
			if !ok {
				return ErrDenied
			}
			return nil
		}
	case "staff":
		return requirePermission(ctx, tx, actor, "academy.staff.manage")
	case "policies":
		return requirePermission(ctx, tx, actor, "academy.policy.manage")
	case "availability":
		return canEditAvailability(ctx, tx, actor, input.(Availability).CoachID)
	case "enrolments":
		return studentPermission(ctx, tx, actor, input.(EnrolmentInput).StudentID, "academy.manage", "", false)
	}
	return requirePermission(ctx, tx, actor, "academy.manage")
}
