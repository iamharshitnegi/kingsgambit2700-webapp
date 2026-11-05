package learning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
)

type Service struct {
	pool      *pgxpool.Pool
	files     FileStore
	scanner   MalwareScanner
	uploadMax int64
	uploads   chan struct{}
}

func NewService(pool *pgxpool.Pool) (*Service, error) {
	if pool == nil {
		return nil, errors.New("learning: postgres pool is required")
	}
	return &Service{pool: pool}, nil
}
func (s *Service) Action(ctx context.Context, actor string, c Command) (Workspace, error) {
	if !platformid.ValidReference(actor) {
		return Workspace{}, ErrDenied
	}
	for _, id := range []string{c.ID, c.StudentID, c.BatchID, c.ClassID, c.RubricID, c.AssessmentID} {
		if id != "" && !platformid.ValidReference(id) {
			return Workspace{}, invalid("record identifiers must be UUIDv4 or UUIDv7")
		}
	}
	if len(c.StudentIDs) > 100 {
		return Workspace{}, invalid("select at most 100 learners")
	}
	for _, id := range c.StudentIDs {
		if !platformid.ValidReference(id) {
			return Workspace{}, invalid("student identifiers must be UUIDv4 or UUIDv7")
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Workspace{}, err
	}
	defer tx.Rollback(ctx)
	var receipt struct {
		Complete bool `json:"complete"`
	}
	found, err := idempotency.Replay(ctx, tx, actor, "learning.action."+c.Action, c, &receipt)
	if errors.Is(err, idempotency.ErrConflict) {
		return Workspace{}, ErrConflict
	}
	if err != nil {
		return Workspace{}, err
	}
	if found {
		if err = tx.Commit(ctx); err != nil {
			return Workspace{}, err
		}
		return s.Workspace(ctx, actor)
	}
	switch c.Action {
	case "placeStudent":
		err = s.placeStudent(ctx, tx, actor, c)
	case "createAssignment", "editAssignment", "assignAssignment", "closeAssignment", "submitAssignment", "reviewAssignment", "returnAssignment", "puzzleMove", "puzzleHint", "puzzleReveal":
		err = s.assignmentAction(ctx, tx, actor, c)
	case "createResource", "editResource", "publishResource", "archiveResource", "reviseResource":
		err = s.resourceAction(ctx, tx, actor, c)
	case "saveGame", "reviewGame":
		err = s.gameAction(ctx, tx, actor, c)
	case "createThread", "sendMessage", "readThread":
		err = s.threadAction(ctx, tx, actor, c)
	case "readNotification", "readAllNotifications":
		if c.Action == "readAllNotifications" {
			_, err = tx.Exec(ctx, `UPDATE learning_notifications SET read_at=COALESCE(read_at,now()) WHERE recipient_id=$1::uuid`, actor)
		} else {
			var tag pgconnTag
			tag, err = execCount(ctx, tx, `UPDATE learning_notifications SET read_at=COALESCE(read_at,now()) WHERE recipient_id=$1::uuid AND id=$2::uuid`, actor, c.ID)
			if err == nil && tag == 0 {
				err = ErrNotFound
			}
		}
	case "createAssessment", "editAssessment", "submitAssessment", "returnAssessment", "publishAssessment", "acknowledgeAssessment", "correctAssessment", "issueCertificate", "revokeCertificate":
		err = s.assessmentAction(ctx, tx, actor, c)
	default:
		err = invalid("unknown learning action")
	}
	if err != nil {
		return Workspace{}, err
	}
	if err = audit(ctx, tx, actor, c.Action, c.ID); err != nil {
		return Workspace{}, err
	}
	receipt.Complete = true
	if err = idempotency.Save(ctx, tx, actor, "learning.action."+c.Action, c, receipt); err != nil {
		return Workspace{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Workspace{}, fmt.Errorf("learning: committing action: %w", err)
	}
	return s.Workspace(ctx, actor)
}

type pgconnTag int64

func execCount(ctx context.Context, tx pgx.Tx, sql string, args ...any) (pgconnTag, error) {
	tag, err := tx.Exec(ctx, sql, args...)
	return pgconnTag(tag.RowsAffected()), err
}
func audit(ctx context.Context, tx pgx.Tx, actor, action, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(id,occurred_at,actor_type,actor_id,action,object_type,object_id,metadata) VALUES($1::uuid,now(),'user',$2::uuid,$3,'learning',$4,'{}')`, platformid.New(), actor, "learning."+action, id)
	return err
}
func notifyStudent(ctx context.Context, tx pgx.Tx, student, title, href string) error {
	rows, err := tx.Query(ctx, `SELECT user_id::text FROM student_profiles WHERE id=$1::uuid UNION SELECT guardian_user_id::text FROM guardian_student_relationships WHERE student_profile_id=$1::uuid AND status='active' AND 'view_progress'=ANY(permissions)`, student)
	if err != nil {
		return err
	}
	recipients := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, recipient := range recipients {
		noticeID := platformid.New()
		if _, err = tx.Exec(ctx, `INSERT INTO learning_notifications(id,recipient_id,student_id,title,href) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5)`, noticeID, recipient, student, title, href); err != nil {
			return err
		}
		if err = notification.EnqueueEmail(ctx, tx, notification.Message{RecipientID: recipient, StudentID: student, Title: "New academy learning update", Section: strings.TrimPrefix(href, "/portal/"), EventKey: noticeID, GuardianPermission: "view_progress"}); err != nil {
			return err
		}
	}
	return nil
}
func encode(value any) ([]byte, error) { return json.Marshal(value) }
func conflictVersion(current int, c Command) error {
	if c.Version != current {
		return ErrConflict
	}
	return nil
}
func nullID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nowUTC() time.Time { return time.Now().UTC() }
