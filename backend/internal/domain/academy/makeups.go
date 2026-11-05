package academy

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) FulfillMakeUp(ctx context.Context, actor, id string, in MakeUpFulfillment) (MakeUp, error) {
	if validID(id) != nil || validID(in.ReplacementClassID) != nil || !validText(in.IdempotencyKey, 1, 200) {
		return MakeUp{}, invalid("select a scheduled replacement class and request key")
	}
	return s.repository.FulfillMakeUp(ctx, actor, id, in, s.now().UTC())
}
func (r *PostgresRepository) FulfillMakeUp(ctx context.Context, actor, id string, in MakeUpFulfillment, now time.Time) (MakeUp, error) {
	var out MakeUp
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT id::text,class_session_id::text,student_profile_id::text,reason,status,COALESCE(replacement_session_id::text,''),created_at FROM academy_make_up_obligations WHERE id=$1::uuid FOR UPDATE`, id).Scan(&out.ID, &out.ClassID, &out.StudentID, &out.Reason, &out.Status, &out.ReplacementClassID, &out.CreatedAt); err != nil {
			return err
		}
		if err := studentPermission(ctx, tx, actor, out.StudentID, "schedule.manage", "", true); err != nil {
			return err
		}
		found, err := replay(ctx, tx, actor, "make-up:"+id, in.IdempotencyKey, in, &out)
		if err != nil || found {
			return err
		}
		if out.Status != "open" || out.ClassID == in.ReplacementClassID {
			return ErrConflict
		}
		var eligible, alreadyUsed bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM academy_class_participants p JOIN class_sessions c ON c.id=p.class_session_id WHERE p.class_session_id=$1::uuid AND p.student_profile_id=$2::uuid AND p.status='reserved' AND c.starts_at>$3 AND c.status IN('scheduled','join_open')),EXISTS(SELECT 1 FROM academy_make_up_obligations WHERE student_profile_id=$2::uuid AND replacement_session_id=$1::uuid AND status='fulfilled')`, in.ReplacementClassID, out.StudentID, now).Scan(&eligible, &alreadyUsed); err != nil {
			return err
		}
		if !eligible || alreadyUsed {
			return invalid("choose an upcoming reserved class for this learner that is not already linked to a make-up")
		}
		if _, err = tx.Exec(ctx, `UPDATE academy_make_up_obligations SET status='fulfilled',replacement_session_id=$2::uuid WHERE id=$1::uuid`, id, in.ReplacementClassID); err != nil {
			return err
		}
		out.Status = "fulfilled"
		out.ReplacementClassID = in.ReplacementClassID
		if err = audit(ctx, tx, actor, "academy.make_up.fulfilled", id, now, in); err != nil {
			return err
		}
		return remember(ctx, tx, actor, "make-up:"+id, in.IdempotencyKey, in, out, now)
	})
	return out, err
}
