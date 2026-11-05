package academy

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
)

func (s *Service) RescheduleTrial(ctx context.Context, actor, id string, in TrialReschedule) (TrialBooking, error) {
	if validID(id) != nil || validID(in.SlotID) != nil || !validText(in.Reason, 5, 1000) || !validText(in.IdempotencyKey, 1, 200) {
		return TrialBooking{}, invalid("choose a trial slot and provide a rescheduling reason and request key")
	}
	return s.repository.RescheduleTrial(ctx, actor, id, in, s.now().UTC())
}
func (r *PostgresRepository) RescheduleTrial(ctx context.Context, actor, id string, in TrialReschedule, now time.Time) (TrialBooking, error) {
	var out TrialBooking
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		found, err := replay(ctx, tx, actor, "trial-reschedule:"+id, in.IdempotencyKey, in, &out)
		if err != nil || found {
			return err
		}
		out, err = loadTrial(ctx, tx, id)
		if err != nil {
			return err
		}
		if out.Status != "confirmed" || !out.StartsAt.After(now) || out.SlotID == in.SlotID {
			return invalid("only a confirmed upcoming trial can move to a different slot")
		}
		// Match the public booking capacity lock; neither path can overbook a slot.
		rows, err := tx.Query(ctx, `SELECT id FROM trial_slots WHERE id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, []string{out.SlotID, in.SlotID})
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		var capacity, booked int
		var starts, ends time.Time
		var oldCoach string
		if err = tx.QueryRow(ctx, `SELECT COALESCE(assessor_user_id::text,'') FROM trial_slots WHERE id=$1::uuid`, out.SlotID).Scan(&oldCoach); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT starts_at,ends_at,capacity FROM trial_slots WHERE id=$1::uuid AND status='published' AND starts_at>$2 AND (cardinality(age_bands)=0 OR (SELECT student_age_band FROM trial_bookings WHERE id=$3::uuid)=ANY(age_bands))`, in.SlotID, now, id).Scan(&starts, &ends, &capacity); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM trial_bookings WHERE slot_id=$1::uuid AND status IN('confirmed','attended')`, in.SlotID).Scan(&booked); err != nil {
			return err
		}
		if booked >= capacity {
			return ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE trial_bookings SET slot_id=$2::uuid,starts_at=$3,ends_at=$4,updated_at=$5 WHERE id=$1::uuid`, id, in.SlotID, starts, ends, now); err != nil {
			return err
		}
		out.SlotID = in.SlotID
		out.StartsAt = starts
		out.EndsAt = ends
		if err = notification.EnqueueTrial(ctx, tx, id, "Academy trial rescheduled", "trial-rescheduled:"+id+":"+in.IdempotencyKey, now, true, oldCoach); err != nil {
			return err
		}
		if err = audit(ctx, tx, actor, "academy.trial.rescheduled", id, now, in); err != nil {
			return err
		}
		return remember(ctx, tx, actor, "trial-reschedule:"+id, in.IdempotencyKey, in, out, now)
	})
	return out, err
}
