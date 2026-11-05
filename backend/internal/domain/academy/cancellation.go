package academy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	"github.com/kingsgambit2700/platform/backend/internal/domain/scheduling"
)

func (r *PostgresRepository) Cancel(ctx context.Context, actor, id string, in CancellationInput, now time.Time, preview bool) (CancellationPreview, error) {
	out := CancellationPreview{ClassID: id, Outcomes: []CancellationOutcome{}}
	err := r.within(ctx, func(tx pgx.Tx) error {
		lesson, err := loadClass(ctx, tx, id)
		if err != nil {
			return err
		}
		staffErr := batchPermission(ctx, tx, actor, lesson.BatchID, "schedule.manage")
		if staffErr != nil && !errors.Is(staffErr, ErrDenied) {
			return staffErr
		}
		if in.StudentID == "" {
			if staffErr != nil {
				return invalid("choose the student whose class you want to cancel")
			}
		} else {
			if err = studentPermission(ctx, tx, actor, in.StudentID, "schedule.manage", "manage_schedule", true); err != nil {
				return err
			}
		}
		if !preview {
			found, err := replay(ctx, tx, actor, "cancel:"+id, in.IdempotencyKey, in, &out)
			if err != nil || found {
				return err
			}
		}
		if err = mutableClass(lesson, in.Revision); err != nil {
			return err
		}
		out.Revision = lesson.Revision
		people, err := classParticipants(ctx, tx, id)
		if err != nil {
			return err
		}
		policy, err := scheduling.DefaultPolicy("academy-v1")
		if err != nil {
			return err
		}
		selected := []participant{}
		for _, person := range people {
			if in.StudentID != "" && person.studentID != in.StudentID {
				continue
			}
			if err = studentPermission(ctx, tx, actor, person.studentID, "schedule.manage", "manage_schedule", true); err != nil {
				return err
			}
			if person.status != "reserved" && !(in.StudentID == "" && person.status == "consumed") {
				if in.StudentID != "" {
					return ErrConflict
				}
				continue
			}
			owner := scheduling.CancellationActorAcademy
			if in.StudentID != "" {
				owner = scheduling.CancellationActorFamily
			}
			decision, err := policy.EvaluateCancellation(scheduling.CancellationInput{SessionID: id, StudentID: person.studentID, Format: scheduling.ClassFormat(lesson.Format), Actor: owner, Kind: scheduling.CancellationKindRequest, StartsAt: lesson.StartsAt, RequestedAt: now})
			if err != nil {
				return err
			}
			out.Outcomes = append(out.Outcomes, CancellationOutcome{StudentID: person.studentID, Action: string(decision.EntitlementAction), MakeUp: decision.MakeUpAction == scheduling.MakeUpActionCreate, Reason: string(decision.Reason)})
			selected = append(selected, person)
		}
		if len(selected) == 0 {
			return ErrNotFound
		}
		if preview {
			return nil
		}
		for i, person := range selected {
			outcome := out.Outcomes[i]
			next := "consumed"
			if outcome.Action == "restore" {
				next = "released"
				if person.status == "consumed" {
					err = ledger(ctx, tx, actor, id, person, "academy-reversal", entitlement.EntryKindReversed, entitlement.Delta{Available: 1, Consumed: -1}, in.Reason, now)
				} else {
					err = ledger(ctx, tx, actor, id, person, "resolve", entitlement.EntryKindReleased, entitlement.Delta{Available: 1, Reserved: -1}, in.Reason, now)
				}
			} else {
				err = ledger(ctx, tx, actor, id, person, "resolve", entitlement.EntryKindConsumed, entitlement.Delta{Reserved: -1, Consumed: 1}, in.Reason, now)
			}
			if err != nil {
				return err
			}
			if outcome.MakeUp {
				if err = makeUp(ctx, tx, id, person.studentID, in.Reason, now); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `UPDATE academy_class_participants SET status=$3,cancellation_reason=$4 WHERE class_session_id=$1::uuid AND student_profile_id=$2::uuid`, id, person.studentID, next, in.Reason); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE session_assignments SET ends_at=$3 WHERE class_session_id=$1::uuid AND user_id=$2::uuid`, id, person.userID, now); err != nil {
				return err
			}
		}
		status := lesson.Status
		if in.StudentID == "" {
			status = "cancelled_academy"
		} else if lesson.Format == "one_to_one" {
			status = "cancelled_family"
		}
		if _, err = tx.Exec(ctx, `UPDATE class_sessions SET status=$2,revision=revision+1,updated_at=$3 WHERE id=$1::uuid`, id, status, now); err != nil {
			return err
		}
		if err = audit(ctx, tx, actor, "academy.class.cancelled", id, now, out); err != nil {
			return err
		}
		out.Revision++
		notice := "Academy class cancelled"
		if in.StudentID != "" && lesson.Format == "group" {
			notice = "Class participation updated"
		}
		if err = notifyClass(ctx, tx, id, notice, now, in.StudentID != "" && lesson.Format == "group"); err != nil {
			return err
		}
		return remember(ctx, tx, actor, "cancel:"+id, in.IdempotencyKey, in, out, now)
	})
	return out, err
}
