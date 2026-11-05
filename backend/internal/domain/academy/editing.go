package academy

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (s *Service) UpdateStudent(ctx context.Context, actor, id string, in StudentUpdate) (Student, error) {
	if validID(id) != nil || !validText(in.Name, 2, 100) || !slices.Contains([]string{"4_7", "8_12", "13_17", "18_plus"}, in.AgeBand) || len(in.Country) != 2 || !slices.Contains([]string{"prospect", "active", "paused"}, in.Status) {
		return Student{}, invalid("student profile details or status are invalid")
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Student{}, invalid("choose an IANA time zone")
	}
	return s.repository.UpdateStudent(ctx, actor, id, in, s.now().UTC())
}
func (r *PostgresRepository) UpdateStudent(ctx context.Context, actor, id string, in StudentUpdate, now time.Time) (Student, error) {
	var out Student
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := studentPermission(ctx, tx, actor, id, "academy.manage", "manage_profile", false); err != nil {
			return err
		}
		var current string
		if err := tx.QueryRow(ctx, `SELECT status FROM student_profiles WHERE id=$1::uuid`, id).Scan(&current); err != nil {
			return err
		}
		if in.Status != current {
			if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
				return err
			}
			if in.Status == "active" {
				ok, err := consentCurrent(ctx, tx, id)
				if err != nil {
					return err
				}
				if !ok {
					return ErrConsent
				}
			} else {
				var upcoming bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM academy_class_participants p JOIN class_sessions c ON c.id=p.class_session_id WHERE p.student_profile_id=$1::uuid AND p.status='reserved' AND c.ends_at>$2)`, id, now).Scan(&upcoming); err != nil {
					return err
				}
				if upcoming {
					return invalid("resolve upcoming classes before pausing a student")
				}
			}
		}
		err := tx.QueryRow(ctx, `UPDATE student_profiles SET preferred_name=$2,age_band=$3,time_zone=$4,country=$5,status=$6,updated_at=$7 WHERE id=$1::uuid RETURNING id::text,user_id::text,preferred_name,age_band,time_zone,status`, id, strings.TrimSpace(in.Name), in.AgeBand, in.TimeZone, strings.ToUpper(in.Country), in.Status, now).Scan(&out.ID, &out.UserID, &out.Name, &out.AgeBand, &out.TimeZone, &out.Status)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET display_name=$2,updated_at=$3 WHERE id=$1::uuid`, out.UserID, out.Name, now); err != nil {
			return err
		}
		out.Country = strings.ToUpper(in.Country)
		out.Consent, err = consentCurrent(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, `SELECT family_id::text FROM guardian_student_relationships WHERE student_profile_id=$1::uuid AND status='active' ORDER BY created_at LIMIT 1`, id).Scan(&out.FamilyID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return audit(ctx, tx, actor, "academy.student.updated", id, now, map[string]string{"status": in.Status})
	})
	return out, err
}
func (s *Service) UpdateBatch(ctx context.Context, actor, id string, in BatchInput) (Batch, error) {
	if validID(id) != nil || validID(in.CoachID) != nil || !validText(in.Name, 2, 120) || !validText(in.Level, 1, 80) || !slices.Contains([]string{"group", "one_to_one"}, in.Format) || in.Capacity < 1 || in.Capacity > 6 || (in.Format == "one_to_one" && in.Capacity != 1) {
		return Batch{}, invalid("batch details are invalid")
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if !slices.Contains([]string{"active", "archived"}, in.Status) {
		return Batch{}, invalid("batch status is invalid")
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Batch{}, invalid("choose an IANA time zone")
	}
	return s.repository.UpdateBatch(ctx, actor, id, in, s.now().UTC())
}
func (r *PostgresRepository) UpdateBatch(ctx context.Context, actor, id string, in BatchInput, now time.Time) (Batch, error) {
	out := Batch{ID: id, Name: in.Name, CoachID: in.CoachID, Format: in.Format, Level: in.Level, TimeZone: in.TimeZone, Capacity: in.Capacity, StudentIDs: []string{}}
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		old, _, err := loadBatch(ctx, tx, id)
		if err != nil {
			return err
		}
		if old.Format != in.Format {
			return invalid("create a separate batch to change class format")
		}
		ok, err := role(ctx, tx, in.CoachID, "coach")
		if err != nil {
			return err
		}
		if !ok {
			return invalid("choose an active coach")
		}
		var enrolled int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM enrolments WHERE batch_id=$1::uuid AND status='active' AND (ends_at IS NULL OR ends_at>$2)`, id, now).Scan(&enrolled); err != nil {
			return err
		}
		if enrolled > in.Capacity {
			return invalid("capacity cannot be lower than current enrolments")
		}
		rows, err := tx.Query(ctx, `SELECT id::text FROM class_sessions WHERE batch_id=$1::uuid AND starts_at>$2 AND status IN('scheduled','join_open') ORDER BY starts_at`, id, now)
		if err != nil {
			return err
		}
		classIDs := []string{}
		for rows.Next() {
			var cid string
			if err = rows.Scan(&cid); err != nil {
				rows.Close()
				return err
			}
			classIDs = append(classIDs, cid)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if in.Status == "archived" && (enrolled > 0 || len(classIDs) > 0) {
			return invalid("end enrolments and resolve future classes before archiving a batch")
		}
		if old.CoachID != in.CoachID {
			for _, cid := range classIDs {
				lesson, err := loadClass(ctx, tx, cid)
				if err != nil {
					return err
				}
				if err = conflict(ctx, tx, in.CoachID, nil, lesson.StartsAt, lesson.EndsAt, cid); err != nil {
					return err
				}
				if err = coachAvailability(ctx, tx, in.CoachID, lesson.StartsAt, lesson.EndsAt); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE session_assignments SET user_id=$2::uuid WHERE class_session_id=$1::uuid AND role='coach'`, cid, in.CoachID); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE class_sessions SET revision=revision+1,updated_at=$2 WHERE id=$1::uuid`, cid, now); err != nil {
					return err
				}
				if err = notifyClass(ctx, tx, cid, "Your academy class coach changed", now, true); err != nil {
					return err
				}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE batches SET name=$2,level_slug=$3,coach_user_id=$4::uuid,source_time_zone=$5,hard_capacity=$6,target_capacity=LEAST(target_capacity,$6),status=$7,updated_at=$8 WHERE id=$1::uuid`, id, in.Name, in.Level, in.CoachID, in.TimeZone, in.Capacity, in.Status, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.batch.updated", id, now, map[string]string{"coachId": in.CoachID, "status": in.Status})
	})
	return out, err
}
func (s *Service) UpdateEnrolment(ctx context.Context, actor, id string, in EnrolmentUpdate) (Enrolment, error) {
	if validID(id) != nil || !slices.Contains([]string{"active", "paused", "cancelled"}, in.Status) {
		return Enrolment{}, invalid("enrolment status is invalid")
	}
	return s.repository.UpdateEnrolment(ctx, actor, id, in, s.now().UTC())
}
func (r *PostgresRepository) UpdateEnrolment(ctx context.Context, actor, id string, in EnrolmentUpdate, now time.Time) (Enrolment, error) {
	var out Enrolment
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT e.id::text,e.student_profile_id::text,e.batch_id::text,a.id::text,e.status,e.starts_at,e.ends_at FROM enrolments e JOIN entitlement_accounts a ON a.enrolment_id=e.id WHERE e.id=$1::uuid FOR UPDATE OF e`, id).Scan(&out.ID, &out.StudentID, &out.BatchID, &out.AccountID, &out.Status, &out.StartsAt, &out.EndsAt); err != nil {
			return err
		}
		if in.EndsAt != nil && !in.EndsAt.After(out.StartsAt) {
			return invalid("enrolment end must follow its start")
		}
		var stranded bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM academy_class_participants p JOIN class_sessions c ON c.id=p.class_session_id WHERE p.enrolment_id=$1::uuid AND p.status='reserved' AND ($2<>'active' OR ($3::timestamptz IS NOT NULL AND c.ends_at>$3)))`, id, in.Status, in.EndsAt).Scan(&stranded); err != nil {
			return err
		}
		if stranded {
			return invalid("resolve affected scheduled classes before changing this enrolment")
		}
		if in.Status == "active" {
			ok, err := consentCurrent(ctx, tx, out.StudentID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrConsent
			}
			var capacity, count int
			if err = tx.QueryRow(ctx, `SELECT hard_capacity FROM batches WHERE id=$1::uuid AND status='active'`, out.BatchID).Scan(&capacity); err != nil {
				return err
			}
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM enrolments WHERE batch_id=$1::uuid AND id<>$2::uuid AND status='active' AND (ends_at IS NULL OR ends_at>$3) AND ($4::timestamptz IS NULL OR starts_at<$4)`, out.BatchID, id, out.StartsAt, in.EndsAt).Scan(&count); err != nil {
				return err
			}
			if count >= capacity {
				return invalid("batch capacity is full")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE enrolments SET status=$2,ends_at=$3,updated_at=$4 WHERE id=$1::uuid`, id, in.Status, in.EndsAt, now); err != nil {
			return err
		}
		out.Status = in.Status
		out.EndsAt = in.EndsAt
		if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(available_delta),0),COALESCE(sum(reserved_delta),0),COALESCE(sum(consumed_delta),0) FROM entitlement_entries WHERE account_id=$1::uuid`, out.AccountID).Scan(&out.Available, &out.Reserved, &out.Consumed); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.enrolment.updated", id, now, in)
	})
	return out, err
}
func (s *Service) Availability(ctx context.Context, actor string) ([]Availability, error) {
	return s.repository.Availability(ctx, actor)
}
func (s *Service) SaveAvailability(ctx context.Context, actor string, in Availability) (Availability, error) {
	if validID(in.CoachID) != nil || !in.EndsAt.After(in.StartsAt) || !in.StartsAt.After(s.now()) || !slices.Contains([]string{"available", "blocked"}, in.Status) {
		return Availability{}, invalid("choose coach, future interval and availability status")
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Availability{}, invalid("choose an IANA time zone")
	}
	in.ID = platformid.New()
	return s.repository.SaveAvailability(ctx, actor, in, s.now().UTC())
}
func (s *Service) DeleteAvailability(ctx context.Context, actor, id string) error {
	if err := validID(id); err != nil {
		return err
	}
	return s.repository.DeleteAvailability(ctx, actor, id, s.now().UTC())
}
func canEditAvailability(ctx context.Context, q querier, actor, coach string) error {
	if err := requirePermission(ctx, q, actor, "schedule.manage"); err == nil {
		return nil
	} else if !errors.Is(err, ErrDenied) {
		return err
	}
	_, deny, err := permission(ctx, q, actor, "schedule.manage", "")
	if err != nil {
		return err
	}
	if deny || actor != coach {
		return ErrDenied
	}
	ok, err := role(ctx, q, actor, "coach")
	if err != nil {
		return err
	}
	if !ok {
		return ErrDenied
	}
	return nil
}
func (r *PostgresRepository) Availability(ctx context.Context, actor string) ([]Availability, error) {
	all := requirePermission(ctx, r.pool, actor, "schedule.manage") == nil
	coach, err := role(ctx, r.pool, actor, "coach")
	if err != nil {
		return nil, err
	}
	if !all && !coach {
		return nil, ErrDenied
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text,coach_user_id::text,starts_at,ends_at,source_time_zone,status FROM coach_availability_windows WHERE status<>'cancelled' AND starts_at IS NOT NULL AND ($2 OR coach_user_id=$1::uuid) ORDER BY starts_at LIMIT `+strconv.Itoa(workspaceLimit(ctx)+1), actor, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Availability{}
	for rows.Next() {
		var a Availability
		if err = rows.Scan(&a.ID, &a.CoachID, &a.StartsAt, &a.EndsAt, &a.TimeZone, &a.Status); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r *PostgresRepository) SaveAvailability(ctx context.Context, actor string, in Availability, now time.Time) (Availability, error) {
	request := in
	request.ID = ""
	err := r.withCreation(ctx, actor, "availability", request, &in, func(tx pgx.Tx) error {
		if err := canEditAvailability(ctx, tx, actor, in.CoachID); err != nil {
			return err
		}
		ok, err := role(ctx, tx, in.CoachID, "coach")
		if err != nil {
			return err
		}
		if !ok {
			return ErrDenied
		}
		if in.Status == "blocked" {
			if err = conflict(ctx, tx, in.CoachID, nil, in.StartsAt, in.EndsAt, ""); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO coach_availability_windows(id,coach_user_id,starts_at,ends_at,source_time_zone,status,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7)`, in.ID, in.CoachID, in.StartsAt, in.EndsAt, in.TimeZone, in.Status, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.availability.created", in.ID, now, in)
	})
	return in, err
}
func (r *PostgresRepository) DeleteAvailability(ctx context.Context, actor, id string, now time.Time) error {
	return r.within(ctx, func(tx pgx.Tx) error {
		var coach string
		if err := tx.QueryRow(ctx, `SELECT coach_user_id::text FROM coach_availability_windows WHERE id=$1::uuid`, id).Scan(&coach); err != nil {
			return err
		}
		if err := canEditAvailability(ctx, tx, actor, coach); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE coach_availability_windows SET status='cancelled' WHERE id=$1::uuid`, id); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.availability.cancelled", id, now, struct{}{})
	})
}
