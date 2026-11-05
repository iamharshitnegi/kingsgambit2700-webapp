package academy

import (
	"context"
	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
	"slices"
	"strconv"
	"time"
)

func (s *Service) Operations(ctx context.Context, actor string) (Operations, error) {
	return s.repository.Operations(ctx, actor)
}
func (s *Service) UpdateLead(ctx context.Context, actor, id string, in LeadUpdate) (Lead, error) {
	if validID(id) != nil || !slices.Contains([]string{"new", "contacted", "trial_booked", "trial_attended", "assessed", "offered", "unqualified", "lost", "duplicate", "do_not_contact"}, in.Status) || len(in.Notes) > 5000 {
		return Lead{}, invalid("lead stage or notes are invalid; use conversion for an enrolled family")
	}
	return s.repository.UpdateLead(ctx, actor, id, in, s.now().UTC())
}
func (s *Service) ConvertLead(ctx context.Context, actor, id string, in ConversionInput) (Lead, error) {
	if validID(id) != nil || validID(in.StudentID) != nil {
		return Lead{}, invalid("choose the enrolled student")
	}
	return s.repository.ConvertLead(ctx, actor, id, in, s.now().UTC())
}
func (s *Service) CreateTrialSlot(ctx context.Context, actor string, in TrialSlot) (TrialSlot, error) {
	if validID(in.CoachID) != nil || !in.EndsAt.After(in.StartsAt) || !in.StartsAt.After(s.now()) || in.EndsAt.Sub(in.StartsAt) > 4*time.Hour || !slices.Contains([]string{"group", "one_to_one"}, in.Format) || in.Capacity < 1 || in.Capacity > 6 || (in.Format == "one_to_one" && in.Capacity != 1) || len(in.AgeBands) == 0 {
		return TrialSlot{}, invalid("choose an active coach, future trial interval, format, capacity and age bands")
	}
	for _, age := range in.AgeBands {
		if !slices.Contains([]string{"4_7", "8_12", "13_17", "18_plus"}, age) {
			return TrialSlot{}, invalid("trial age band is invalid")
		}
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return TrialSlot{}, invalid("choose an IANA time zone")
	}
	in.ID = platformid.New()
	in.Status = "published"
	in.AvailableSeats = in.Capacity
	return s.repository.CreateTrialSlot(ctx, actor, in, s.now().UTC())
}
func (s *Service) UpdateTrial(ctx context.Context, actor, id string, in TrialUpdate) (TrialBooking, error) {
	if validID(id) != nil || !slices.Contains([]string{"confirmed", "attended", "no_show", "cancelled"}, in.Status) || len(in.Notes) > 5000 {
		return TrialBooking{}, invalid("trial status or notes are invalid")
	}
	return s.repository.UpdateTrial(ctx, actor, id, in, s.now().UTC())
}

func (r *PostgresRepository) Operations(ctx context.Context, actor string) (Operations, error) {
	out := Operations{Leads: []Lead{}, TrialSlots: []TrialSlot{}, TrialBookings: []TrialBooking{}, Pagination: Pagination{Limit: workspaceLimit(ctx), NextLimit: workspaceLimit(ctx) + 500}}
	limitSQL := strconv.Itoa(out.Pagination.Limit + 1)
	if err := requirePermission(ctx, r.pool, actor, "academy.manage"); err != nil {
		out.Leads = bounded(out.Leads, &out.Pagination)
		out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
		out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
		return out, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text,guardian_name,COALESCE(normalized_email::text,''),COALESCE(normalized_phone,''),student_age_band,status,academy_notes,COALESCE(converted_student_id::text,''),created_at FROM leads ORDER BY created_at DESC LIMIT `+limitSQL)
	if err != nil {
		out.Leads = bounded(out.Leads, &out.Pagination)
		out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
		out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
		return out, err
	}
	for rows.Next() {
		var lead Lead
		if err = rows.Scan(&lead.ID, &lead.GuardianName, &lead.Email, &lead.Phone, &lead.AgeBand, &lead.Status, &lead.Notes, &lead.StudentID, &lead.CreatedAt); err != nil {
			rows.Close()
			out.Leads = bounded(out.Leads, &out.Pagination)
			out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
			out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
			return out, err
		}
		out.Leads = append(out.Leads, lead)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		out.Leads = bounded(out.Leads, &out.Pagination)
		out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
		out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
		return out, err
	}
	rows, err = r.pool.Query(ctx, `SELECT s.id::text,COALESCE(s.assessor_user_id::text,''),s.starts_at,s.ends_at,s.source_time_zone,s.format,s.capacity,s.age_bands,s.status,s.capacity-count(b.id) FILTER(WHERE b.status IN('confirmed','attended')) FROM trial_slots s LEFT JOIN trial_bookings b ON b.slot_id=s.id GROUP BY s.id ORDER BY s.starts_at DESC LIMIT `+limitSQL)
	if err != nil {
		out.Leads = bounded(out.Leads, &out.Pagination)
		out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
		out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
		return out, err
	}
	for rows.Next() {
		var slot TrialSlot
		if err = rows.Scan(&slot.ID, &slot.CoachID, &slot.StartsAt, &slot.EndsAt, &slot.TimeZone, &slot.Format, &slot.Capacity, &slot.AgeBands, &slot.Status, &slot.AvailableSeats); err != nil {
			rows.Close()
			out.Leads = bounded(out.Leads, &out.Pagination)
			out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
			out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
			return out, err
		}
		out.TrialSlots = append(out.TrialSlots, slot)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		out.Leads = bounded(out.Leads, &out.Pagination)
		out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
		out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
		return out, err
	}
	rows, err = r.pool.Query(ctx, `SELECT id::text,slot_id::text,lead_id::text,guardian_name,COALESCE(student_name,''),status,starts_at,ends_at,academy_notes FROM trial_bookings ORDER BY starts_at DESC LIMIT `+limitSQL)
	if err != nil {
		out.Leads = bounded(out.Leads, &out.Pagination)
		out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
		out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
		return out, err
	}
	for rows.Next() {
		var booking TrialBooking
		if err = rows.Scan(&booking.ID, &booking.SlotID, &booking.LeadID, &booking.GuardianName, &booking.StudentName, &booking.Status, &booking.StartsAt, &booking.EndsAt, &booking.Notes); err != nil {
			rows.Close()
			out.Leads = bounded(out.Leads, &out.Pagination)
			out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
			out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
			return out, err
		}
		out.TrialBookings = append(out.TrialBookings, booking)
	}
	err = rows.Err()
	rows.Close()
	out.Leads = bounded(out.Leads, &out.Pagination)
	out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
	out.TrialBookings = bounded(out.TrialBookings, &out.Pagination)
	return out, err
}
func loadLead(ctx context.Context, q querier, id string) (Lead, error) {
	var lead Lead
	err := q.QueryRow(ctx, `SELECT id::text,guardian_name,COALESCE(normalized_email::text,''),COALESCE(normalized_phone,''),student_age_band,status,academy_notes,COALESCE(converted_student_id::text,''),created_at FROM leads WHERE id=$1::uuid`, id).Scan(&lead.ID, &lead.GuardianName, &lead.Email, &lead.Phone, &lead.AgeBand, &lead.Status, &lead.Notes, &lead.StudentID, &lead.CreatedAt)
	return lead, err
}
func (r *PostgresRepository) UpdateLead(ctx context.Context, actor, id string, in LeadUpdate, now time.Time) (Lead, error) {
	var out Lead
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		var err error
		out, err = loadLead(ctx, tx, id)
		if err != nil {
			return err
		}
		if out.StudentID != "" {
			return invalid("converted leads retain their enrolled status")
		}
		if _, err = tx.Exec(ctx, `UPDATE leads SET status=$2,academy_notes=$3,updated_at=$4 WHERE id=$1::uuid`, id, in.Status, in.Notes, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO lead_stage_events(id,lead_id,stage,source,actor_user_id,occurred_at) VALUES($1::uuid,$2::uuid,$3,'academy_portal',$4::uuid,$5)`, platformid.New(), id, in.Status, actor, now); err != nil {
			return err
		}
		out.Status = in.Status
		out.Notes = in.Notes
		return audit(ctx, tx, actor, "academy.lead.updated", id, now, map[string]string{"status": in.Status})
	})
	return out, err
}
func (r *PostgresRepository) ConvertLead(ctx context.Context, actor, id string, in ConversionInput, now time.Time) (Lead, error) {
	var out Lead
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		var err error
		out, err = loadLead(ctx, tx, id)
		if err != nil {
			return err
		}
		if out.StudentID == in.StudentID {
			return nil
		}
		if out.StudentID != "" {
			return ErrConflict
		}
		var eligible bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM student_profiles s JOIN guardian_student_relationships g ON g.student_profile_id=s.id JOIN users u ON u.id=g.guardian_user_id WHERE s.id=$1::uuid AND g.status='active' AND u.email=$2 AND EXISTS(SELECT 1 FROM enrolments e WHERE e.student_profile_id=s.id AND e.status='active'))`, in.StudentID, out.Email).Scan(&eligible)
		if err != nil {
			return err
		}
		if !eligible {
			return invalid("choose an enrolled student linked to this lead's guardian email")
		}
		if _, err = tx.Exec(ctx, `UPDATE leads SET status='won',converted_student_id=$2::uuid,updated_at=$3 WHERE id=$1::uuid`, id, in.StudentID, now); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO lead_stage_events(id,lead_id,stage,source,actor_user_id,occurred_at) VALUES($1::uuid,$2::uuid,'won','academy_conversion',$3::uuid,$4)`, platformid.New(), id, actor, now); err != nil {
			return err
		}
		out.Status = "won"
		out.StudentID = in.StudentID
		return audit(ctx, tx, actor, "academy.lead.converted", id, now, in)
	})
	return out, err
}
func (r *PostgresRepository) CreateTrialSlot(ctx context.Context, actor string, in TrialSlot, now time.Time) (TrialSlot, error) {
	request := in
	request.ID = ""
	err := r.withCreation(ctx, actor, "trial-slots", request, &in, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		ok, err := role(ctx, tx, in.CoachID, "coach")
		if err != nil {
			return err
		}
		if !ok {
			return invalid("choose an active coach")
		}
		if err = conflict(ctx, tx, in.CoachID, nil, in.StartsAt, in.EndsAt, ""); err != nil {
			return err
		}
		if err = coachAvailability(ctx, tx, in.CoachID, in.StartsAt, in.EndsAt); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO trial_slots(id,starts_at,ends_at,source_time_zone,format,age_bands,capacity,assessor_user_id,status,created_at,updated_at) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8::uuid,'published',$9,$9)`, in.ID, in.StartsAt, in.EndsAt, in.TimeZone, in.Format, in.AgeBands, in.Capacity, in.CoachID, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.trial_slot.created", in.ID, now, map[string]string{"coachId": in.CoachID})
	})
	return in, err
}
func loadTrial(ctx context.Context, q querier, id string) (TrialBooking, error) {
	var out TrialBooking
	err := q.QueryRow(ctx, `SELECT id::text,slot_id::text,lead_id::text,guardian_name,COALESCE(student_name,''),status,starts_at,ends_at,academy_notes FROM trial_bookings WHERE id=$1::uuid`, id).Scan(&out.ID, &out.SlotID, &out.LeadID, &out.GuardianName, &out.StudentName, &out.Status, &out.StartsAt, &out.EndsAt, &out.Notes)
	return out, err
}
func (r *PostgresRepository) UpdateTrial(ctx context.Context, actor, id string, in TrialUpdate, now time.Time) (TrialBooking, error) {
	var out TrialBooking
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		var err error
		out, err = loadTrial(ctx, tx, id)
		if err != nil {
			return err
		}
		if (in.Status == "attended" || in.Status == "no_show") && now.Before(out.StartsAt) {
			return invalid("record trial attendance after it starts")
		}
		if in.Status == "confirmed" && !now.Before(out.StartsAt) {
			return invalid("a past trial cannot be confirmed")
		}
		var capacity, count int
		if err = tx.QueryRow(ctx, `SELECT capacity FROM trial_slots WHERE id=$1::uuid AND status='published' FOR UPDATE`, out.SlotID).Scan(&capacity); err != nil {
			return err
		}
		if in.Status == "confirmed" || in.Status == "attended" {
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM trial_bookings WHERE slot_id=$1::uuid AND id<>$2::uuid AND status IN('confirmed','attended')`, out.SlotID, id).Scan(&count); err != nil {
				return err
			}
			if count >= capacity {
				return ErrConflict
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE trial_bookings SET status=$2,academy_notes=$3,updated_at=$4 WHERE id=$1::uuid`, id, in.Status, in.Notes, now); err != nil {
			return err
		}
		if in.Status == "attended" {
			if _, err = tx.Exec(ctx, `UPDATE leads SET status='trial_attended',updated_at=$2 WHERE id=$1::uuid AND converted_student_id IS NULL`, out.LeadID, now); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO lead_stage_events(id,lead_id,stage,source,actor_user_id,occurred_at) VALUES($1::uuid,$2::uuid,'trial_attended','trial_outcome',$3::uuid,$4)`, platformid.New(), out.LeadID, actor, now); err != nil {
				return err
			}
		}
		if out.Status != in.Status {
			if err = notification.EnqueueTrial(ctx, tx, id, "Academy trial status updated", "trial-status:"+id+":"+now.Format(time.RFC3339Nano), now, in.Status == "confirmed", ""); err != nil {
				return err
			}
		}
		out.Status = in.Status
		out.Notes = in.Notes
		return audit(ctx, tx, actor, "academy.trial.updated", id, now, map[string]string{"status": in.Status})
	})
	return out, err
}
func coachAvailability(ctx context.Context, q querier, coach string, start, end time.Time) error {
	var blocked, configured, available bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM coach_availability_windows WHERE coach_user_id=$1::uuid AND status='blocked' AND starts_at<$3 AND ends_at>$2),EXISTS(SELECT 1 FROM coach_availability_windows WHERE coach_user_id=$1::uuid AND status='available' AND ends_at>now()),EXISTS(SELECT 1 FROM coach_availability_windows WHERE coach_user_id=$1::uuid AND status='available' AND starts_at<=$2 AND ends_at>=$3)`, coach, start, end).Scan(&blocked, &configured, &available)
	if err != nil {
		return err
	}
	if blocked || (configured && !available) {
		return invalid("the coach is unavailable during this interval")
	}
	return nil
}
