package learning

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func emptyWorkspace() Workspace {
	return Workspace{Placements: []Placement{}, Students: []Student{}, Assignments: []Assignment{}, Resources: []Resource{}, Games: []Game{}, Threads: []Thread{}, Notifications: []Notification{}, Rubrics: []Rubric{}, Assessments: []Assessment{}, Certificates: []Certificate{}}
}

func (s *Service) Workspace(ctx context.Context, actor string) (Workspace, error) {
	w := emptyWorkspace()
	w.Pagination = Pagination{Limit: workspaceLimit(ctx), NextLimit: workspaceLimit(ctx) + 500}
	limitSQL := strconv.Itoa(w.Pagination.Limit + 1)
	if !platformid.ValidReference(actor) {
		return w, ErrDenied
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return w, err
	}
	defer tx.Rollback(ctx)
	var active bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid AND status='active')`, actor).Scan(&active); err != nil {
		return w, err
	}
	if !active {
		return w, ErrDenied
	}
	w.CanPublish, err = permission(ctx, tx, actor, "learning.publish")
	if err != nil {
		return w, err
	}
	w.CanPlace, err = academicPlacementRole(ctx, tx, actor)
	if err != nil {
		return w, err
	}
	w.CanPlace = w.CanPlace && w.CanPublish
	w.CanCreateResources, err = staffResourceAccess(ctx, tx, actor)
	if err != nil {
		return w, err
	}
	readIDs, err := discoverStudents(ctx, tx, actor, "learning.view", "view_progress")
	if err != nil {
		return w, err
	}
	commIDs, err := discoverStudents(ctx, tx, actor, "learning.communicate", "communicate_with_academy")
	if err != nil {
		return w, err
	}
	readList := []string{}
	manageList := []string{}
	commList := []string{}
	for _, item := range readIDs {
		st := item
		readList = append(readList, st.ID)
		if err = authorizeStudent(ctx, tx, actor, st.ID, "learning.manage", "", false); err == nil {
			st.CanManage = true
			manageList = append(manageList, st.ID)
		} else if !errors.Is(err, ErrDenied) {
			return w, err
		}
		if err = ownStudent(ctx, tx, actor, st.ID); err == nil {
			st.CanSubmit = true
		} else if !errors.Is(err, ErrDenied) {
			return w, err
		}
		if err = authorizeStudent(ctx, tx, actor, st.ID, "learning.communicate", "communicate_with_academy", true); err == nil {
			st.CanCommunicate = true
		} else if !errors.Is(err, ErrDenied) {
			return w, err
		}
		w.Students = append(w.Students, st)
	}
	for _, item := range commIDs {
		commList = append(commList, item.ID)
		found := false
		for _, existing := range w.Students {
			if existing.ID == item.ID {
				found = true
				break
			}
		}
		if !found {
			item.CanCommunicate = true
			item.LevelSlug = ""
			w.Students = append(w.Students, item)
		}
	}
	rows, err := tx.Query(ctx, `SELECT `+assignmentColumns+` FROM learning_assignments WHERE student_profile_id=ANY($1::uuid[]) AND (status<>'draft' OR student_profile_id=ANY($2::uuid[])) ORDER BY created_at DESC,id DESC LIMIT `+limitSQL, readList, manageList)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		a, scanErr := scanAssignment(rows)
		if scanErr != nil {
			rows.Close()
			return w, scanErr
		}
		w.Assignments = append(w.Assignments, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	w.Assignments = bounded(w.Assignments, &w.Pagination)
	for i := range w.Assignments {
		a := &w.Assignments[i]
		a.Attempts, err = loadAttempts(ctx, tx, a.ID, len(a.Puzzles))
		if err != nil {
			return w, err
		}
		if err = authorizeStudent(ctx, tx, actor, a.StudentID, "learning.manage", "", false); errors.Is(err, ErrDenied) {
			for j := range a.Puzzles {
				a.Puzzles[j].Solutions = nil
			}
		} else if err != nil {
			return w, err
		}
	}
	if w.CanCreateResources {
		rows, err = tx.Query(ctx, `SELECT r.id::text,r.owner_id::text,r.title,r.kind,r.visibility,r.status,r.level_slug,r.tags,r.content,r.version,COALESCE(f.id::text,''),COALESCE(f.filename,''),COALESCE(f.media_type,'') FROM teaching_resources r LEFT JOIN teaching_files f ON f.id=r.file_id WHERE r.owner_id=$1::uuid OR (r.visibility='shared' AND (r.status='published' OR $2)) ORDER BY r.updated_at DESC,r.id DESC LIMIT `+limitSQL, actor, w.CanPublish)
		if err != nil {
			return w, err
		}
		for rows.Next() {
			var r Resource
			if err = rows.Scan(&r.ID, &r.OwnerID, &r.Title, &r.Kind, &r.Visibility, &r.Status, &r.LevelSlug, &r.Tags, &r.Content, &r.Version, &r.FileID, &r.Filename, &r.MediaType); err != nil {
				rows.Close()
				return w, err
			}
			w.Resources = append(w.Resources, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return w, err
		}
	}
	rows, err = tx.Query(ctx, `SELECT id::text,student_id::text,title,source,pgn,CASE WHEN shared OR student_id=ANY($2::uuid[]) THEN review ELSE '' END,shared,played_at,version FROM student_games WHERE student_id=ANY($1::uuid[]) AND (shared OR owner_id=$3::uuid OR student_id=ANY($2::uuid[])) ORDER BY played_at DESC,id DESC LIMIT `+limitSQL, readList, manageList, actor)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var g Game
		if err = rows.Scan(&g.ID, &g.StudentID, &g.Title, &g.Source, &g.PGN, &g.Review, &g.Shared, &g.PlayedAt, &g.Version); err != nil {
			rows.Close()
			return w, err
		}
		w.Games = append(w.Games, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	rows, err = tx.Query(ctx, `SELECT t.id::text,COALESCE(t.student_id::text,''),COALESCE(t.batch_id::text,''),t.title FROM learning_threads t WHERE student_id=ANY($1::uuid[]) OR batch_id IN(SELECT e.batch_id FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=ANY($1::uuid[]) AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.status='active') ORDER BY created_at DESC,id DESC LIMIT `+limitSQL, commList)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var thread Thread
		thread.Messages = []Message{}
		if err = rows.Scan(&thread.ID, &thread.StudentID, &thread.BatchID, &thread.Title); err != nil {
			rows.Close()
			return w, err
		}
		w.Threads = append(w.Threads, thread)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	w.Threads = bounded(w.Threads, &w.Pagination)
	for i := range w.Threads {
		thread := &w.Threads[i]
		if err = threadAccess(ctx, tx, actor, thread.StudentID, thread.BatchID, true); err == nil {
			thread.CanSend = true
		} else if !errors.Is(err, ErrDenied) {
			return w, err
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM learning_messages m LEFT JOIN learning_thread_reads r ON r.thread_id=m.thread_id AND r.user_id=$2::uuid WHERE m.thread_id=$1::uuid AND m.actor_id<>$2::uuid AND (r.read_at IS NULL OR m.created_at>r.read_at)`, thread.ID, actor).Scan(&thread.Unread); err != nil {
			return w, err
		}
		rows, err = tx.Query(ctx, `SELECT id::text,actor_id::text,display_name,body,created_at FROM(SELECT m.id,m.actor_id,u.display_name,m.body,m.created_at FROM learning_messages m JOIN users u ON u.id=m.actor_id WHERE thread_id=$1::uuid ORDER BY m.created_at DESC,m.id DESC LIMIT `+limitSQL+`) recent ORDER BY created_at,id`, thread.ID)
		if err != nil {
			return w, err
		}
		for rows.Next() {
			var m Message
			if err = rows.Scan(&m.ID, &m.ActorID, &m.ActorName, &m.Body, &m.CreatedAt); err != nil {
				rows.Close()
				return w, err
			}
			thread.Messages = append(thread.Messages, m)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return w, err
		}
		if len(thread.Messages) > w.Pagination.Limit {
			w.Pagination.HasMore = true
			thread.Messages = thread.Messages[1:]
		}
	}
	scheduleIDs, err := discoverStudents(ctx, tx, actor, "schedule.view", "view_schedule")
	if err != nil {
		return w, err
	}
	scheduleList := []string{}
	for _, student := range scheduleIDs {
		scheduleList = append(scheduleList, student.ID)
	}
	rows, err = tx.Query(ctx, `SELECT id::text,title,href,created_at,read_at FROM learning_notifications WHERE recipient_id=$1::uuid AND (student_id IS NULL OR (href LIKE '%/calendar' AND student_id=ANY($3::uuid[])) OR (href LIKE '%/messages' AND student_id=ANY($4::uuid[])) OR (href NOT LIKE '%/calendar' AND href NOT LIKE '%/messages' AND student_id=ANY($2::uuid[]))) ORDER BY created_at DESC,id DESC LIMIT `+limitSQL, actor, readList, scheduleList, commList)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.Title, &n.Href, &n.CreatedAt, &n.ReadAt); err != nil {
			rows.Close()
			return w, err
		}
		w.Notifications = append(w.Notifications, n)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	rows, err = tx.Query(ctx, `SELECT id::text,level_slug,title,version,dimensions FROM learning_rubrics ORDER BY level_slug,version DESC`)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var r Rubric
		if err = rows.Scan(&r.ID, &r.LevelSlug, &r.Title, &r.Version, &r.Dimensions); err != nil {
			rows.Close()
			return w, err
		}
		w.Rubrics = append(w.Rubrics, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	rows, err = tx.Query(ctx, `SELECT v.id::text FROM progress_report_versions v JOIN progress_reports p ON p.id=v.progress_report_id WHERE p.student_profile_id=ANY($1::uuid[]) AND (v.published_at IS NOT NULL OR p.student_profile_id=ANY($2::uuid[])) ORDER BY v.created_at DESC,v.id DESC LIMIT `+limitSQL, readList, manageList)
	if err != nil {
		return w, err
	}
	reportVersions := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return w, err
		}
		reportVersions = append(reportVersions, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	reportVersions = bounded(reportVersions, &w.Pagination)
	for _, id := range reportVersions {
		a, _, _, loadErr := loadAssessment(ctx, tx, id, actor, false)
		if loadErr != nil {
			return w, loadErr
		}
		// Staff can inspect review history; families see only immutable publication content.
		if authorizeErr := authorizeStudent(ctx, tx, actor, a.StudentID, "learning.manage", "", false); authorizeErr == nil {
			history, historyErr := tx.Query(ctx, `SELECT action,actor_id::text,occurred_at,content FROM learning_assessment_history WHERE assessment_id=$1::uuid ORDER BY occurred_at,id`, id)
			if historyErr != nil {
				return w, historyErr
			}
			for history.Next() {
				var event AssessmentEvent
				if historyErr = history.Scan(&event.Action, &event.ActorID, &event.At, &event.Content); historyErr != nil {
					history.Close()
					return w, historyErr
				}
				a.History = append(a.History, event)
			}
			historyErr = history.Err()
			history.Close()
			if historyErr != nil {
				return w, historyErr
			}
		} else if !errors.Is(authorizeErr, ErrDenied) {
			return w, authorizeErr
		}
		w.Assessments = append(w.Assessments, a)
	}
	rows, err = tx.Query(ctx, `SELECT id::text,student_id::text,assessment_id::text,level_slug,status,issued_at,reason,COALESCE(file_id::text,'') FROM learning_certificates WHERE student_id=ANY($1::uuid[]) ORDER BY issued_at DESC,id DESC LIMIT `+limitSQL, readList)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var c Certificate
		if err = rows.Scan(&c.ID, &c.StudentID, &c.AssessmentID, &c.LevelSlug, &c.Status, &c.IssuedAt, &c.Reason, &c.FileID); err != nil {
			rows.Close()
			return w, err
		}
		w.Certificates = append(w.Certificates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	rows, err = tx.Query(ctx, `SELECT p.id::text,p.student_id::text,p.curriculum_version_id::text,c.version,p.level_slug,p.previous_level_slug,p.report_version_id::text,p.reason,p.placed_by::text,p.created_at FROM learning_placements p JOIN curriculum_versions c ON c.id=p.curriculum_version_id WHERE p.student_id=ANY($1::uuid[]) ORDER BY p.created_at DESC,p.id DESC LIMIT `+limitSQL, readList)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var p Placement
		if err = rows.Scan(&p.ID, &p.StudentID, &p.CurriculumVersionID, &p.CurriculumVersion, &p.LevelSlug, &p.PreviousLevelSlug, &p.AssessmentID, &p.Reason, &p.PlacedBy, &p.CreatedAt); err != nil {
			rows.Close()
			return w, err
		}
		w.Placements = append(w.Placements, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return w, err
	}
	w.Students = bounded(w.Students, &w.Pagination)
	w.Resources = bounded(w.Resources, &w.Pagination)
	w.Games = bounded(w.Games, &w.Pagination)
	w.Notifications = bounded(w.Notifications, &w.Pagination)
	w.Certificates = bounded(w.Certificates, &w.Pagination)
	w.Placements = bounded(w.Placements, &w.Pagination)
	return w, tx.Commit(ctx)
}
func discoverStudents(ctx context.Context, tx pgx.Tx, actor, code, guardian string) ([]Student, error) {
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.preferred_name,`+currentLevelSQL+` FROM student_profiles s WHERE `+studentAccessSQL+` ORDER BY s.preferred_name,s.id`, actor, code, guardian, true)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Student{}
	for rows.Next() {
		var st Student
		if err = rows.Scan(&st.ID, &st.Name, &st.LevelSlug); err != nil {
			return nil, err
		}
		result = append(result, st)
	}
	return result, rows.Err()
}
