package learning

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type assignmentContent struct {
	Puzzles   []Puzzle           `json:"puzzles"`
	Resources []AssignedResource `json:"resources"`
}
type attemptPayload struct {
	Answer  string           `json:"answer"`
	Puzzles []PuzzleProgress `json:"puzzles"`
}

const assignmentColumns = `id::text,student_profile_id::text,COALESCE(class_session_id::text,''),title,instructions,assignment_type,status,revision,due_at,content`

func scanAssignment(row pgx.Row) (Assignment, error) {
	a := Assignment{Puzzles: []Puzzle{}, Attempts: []Attempt{}, Resources: []AssignedResource{}}
	var content []byte
	err := row.Scan(&a.ID, &a.StudentID, &a.ClassID, &a.Title, &a.Description, &a.Kind, &a.Status, &a.Version, &a.DueAt, &content)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	var parsed assignmentContent
	if err = json.Unmarshal(content, &parsed); err != nil {
		return a, err
	}
	if parsed.Puzzles != nil {
		a.Puzzles = parsed.Puzzles
	}
	if parsed.Resources != nil {
		a.Resources = parsed.Resources
	}
	return a, nil
}
func (s *Service) assignmentAction(ctx context.Context, tx pgx.Tx, actor string, c Command) error {
	if c.Action == "createAssignment" {
		if err := validateAssignment(c); err != nil {
			return err
		}
		ids := slices.Clone(c.StudentIDs)
		if len(ids) == 0 && c.StudentID != "" {
			ids = []string{c.StudentID}
		}
		if len(ids) == 0 {
			return invalid("select at least one learner")
		}
		status := c.Status
		if status == "" {
			status = "draft"
		}
		if status != "draft" && status != "assigned" {
			return invalid("new homework must be draft or assigned")
		}
		resources, err := snapshotResources(ctx, tx, actor, c.ResourceIDs)
		if err != nil {
			return err
		}
		content, err := encode(assignmentContent{Puzzles: c.Puzzles, Resources: resources})
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, student := range ids {
			if seen[student] {
				continue
			}
			seen[student] = true
			if err = authorizeStudent(ctx, tx, actor, student, "learning.manage", "", false); err != nil {
				return err
			}
			if c.ClassID != "" {
				var related bool
				err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM session_assignments a JOIN student_profiles s ON s.user_id=a.user_id WHERE a.class_session_id=$1::uuid AND s.id=$2::uuid AND a.role='student')`, c.ClassID, student).Scan(&related)
				if err != nil {
					return err
				}
				if !related {
					return invalid("the learner is not assigned to this class")
				}
			}
			id := platformid.New()
			_, err = tx.Exec(ctx, `INSERT INTO learning_assignments(id,assignment_key,student_profile_id,class_session_id,title,instructions,assignment_type,version,status,due_at,authored_by,created_at,content)
    VALUES($1::uuid,$1::uuid,$2::uuid,$3::uuid,$4,$5,$6,1,$7,$8,$9::uuid,now(),$10::jsonb)`, id, student, nullID(c.ClassID), c.Title, c.Description, c.Kind, status, c.DueAt, actor, content)
			if err != nil {
				return err
			}
			if status == "assigned" {
				if err = notifyStudent(ctx, tx, student, "New practice: "+c.Title, "/portal/homework"); err != nil {
					return err
				}
				if err = remindAssignment(ctx, tx, id, student, c.DueAt); err != nil {
					return err
				}
			}
		}
		return nil
	}
	a, err := scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentColumns+` FROM learning_assignments WHERE id=$1::uuid FOR UPDATE`, c.ID))
	if err != nil {
		return err
	}
	if err = conflictVersion(a.Version, c); err != nil {
		return err
	}
	studentAction := slices.Contains([]string{"submitAssignment", "puzzleMove", "puzzleHint", "puzzleReveal"}, c.Action)
	if studentAction {
		err = ownStudent(ctx, tx, actor, a.StudentID)
	} else {
		err = authorizeStudent(ctx, tx, actor, a.StudentID, "learning.manage", "", false)
	}
	if err != nil {
		return err
	}
	switch c.Action {
	case "editAssignment":
		if a.Status != "draft" {
			return ErrConflict
		}
		if err = validateAssignment(c); err != nil {
			return err
		}
		var content []byte
		resources, loadErr := snapshotResources(ctx, tx, actor, c.ResourceIDs)
		if loadErr != nil {
			return loadErr
		}
		content, err = encode(assignmentContent{Puzzles: c.Puzzles, Resources: resources})
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE learning_assignments SET title=$2,instructions=$3,assignment_type=$4,due_at=$5,content=$6::jsonb,version=version+1,revision=revision+1 WHERE id=$1::uuid`, a.ID, c.Title, c.Description, c.Kind, c.DueAt, content)
		return err
	case "assignAssignment":
		if a.Status != "draft" {
			return ErrConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE learning_assignments SET status='assigned',revision=revision+1 WHERE id=$1::uuid`, a.ID); err != nil {
			return err
		}
		if err = remindAssignment(ctx, tx, a.ID, a.StudentID, a.DueAt); err != nil {
			return err
		}
		return notifyStudent(ctx, tx, a.StudentID, "New practice: "+a.Title, "/portal/homework")
	case "closeAssignment":
		if a.Status != "assigned" {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE learning_assignments SET status='closed',revision=revision+1 WHERE id=$1::uuid`, a.ID)
		return err
	}
	if a.Status != "assigned" {
		return ErrConflict
	}
	attempts, err := loadAttempts(ctx, tx, a.ID, len(a.Puzzles))
	if err != nil {
		return err
	}
	attempt := Attempt{ID: platformid.New(), Number: 1, Status: "started", Puzzles: make([]PuzzleProgress, len(a.Puzzles))}
	exists := false
	if len(attempts) > 0 {
		attempt = attempts[len(attempts)-1]
		exists = true
		if attempt.Status == "returned" && studentAction {
			attempt = Attempt{ID: platformid.New(), Number: attempt.Number + 1, Status: "started", Puzzles: make([]PuzzleProgress, len(a.Puzzles))}
			exists = false
		}
	}
	switch c.Action {
	case "reviewAssignment", "returnAssignment":
		if attempt.Status != "submitted" {
			return ErrConflict
		}
		if !required(c.Feedback, 10000) {
			return invalid("feedback is required")
		}
		attempt.Feedback = c.Feedback
		attempt.Status = "reviewed"
		if c.Action == "returnAssignment" {
			attempt.Status = "returned"
		}
		_, err = tx.Exec(ctx, `UPDATE assignment_attempts SET status=$2,feedback=$3,reviewed_by=$4::uuid,reviewed_at=now() WHERE id=$1::uuid`, attempt.ID, attempt.Status, c.Feedback, actor)
		if err != nil {
			return err
		}
		if err = notifyStudent(ctx, tx, a.StudentID, "Coach feedback: "+a.Title, "/portal/homework"); err != nil {
			return err
		}
	case "submitAssignment", "puzzleMove", "puzzleHint", "puzzleReveal":
		if attempt.Status != "started" {
			return ErrConflict
		}
		if c.Action == "submitAssignment" {
			if a.Kind == "file_upload" && len(attempt.Files) == 0 {
				return invalid("upload your work before submitting")
			}
			if a.Kind != "puzzle_set" && a.Kind != "file_upload" && !required(c.Answer, 10000) {
				return invalid("add your response before submitting")
			}
			if len(c.Answer) > 10000 {
				return invalid("response is too long")
			}
			attempt.Answer = c.Answer
			attempt.Status = "submitted"
		} else {
			if c.PuzzleIndex < 0 || c.PuzzleIndex >= len(a.Puzzles) {
				return invalid("choose an available puzzle")
			}
			attempt.Puzzles[c.PuzzleIndex], err = applyPuzzle(a.Puzzles[c.PuzzleIndex], attempt.Puzzles[c.PuzzleIndex], c, nowUTC())
			if err != nil {
				return err
			}
		}
		payload, encodeErr := encode(attemptPayload{Answer: attempt.Answer, Puzzles: attempt.Puzzles})
		if encodeErr != nil {
			return encodeErr
		}
		if !exists {
			_, err = tx.Exec(ctx, `INSERT INTO assignment_attempts(id,assignment_id,student_profile_id,attempt_number,submitted_payload,status,submitted_at,created_at)
     VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb,$6,CASE WHEN $6='submitted' THEN now() ELSE NULL END,now())`, attempt.ID, a.ID, a.StudentID, attempt.Number, payload, attempt.Status)
		} else {
			_, err = tx.Exec(ctx, `UPDATE assignment_attempts SET submitted_payload=$2::jsonb,status=$3,submitted_at=CASE WHEN $3='submitted' THEN now() ELSE submitted_at END WHERE id=$1::uuid`, attempt.ID, payload, attempt.Status)
		}
		if err != nil {
			return err
		}
	default:
		return invalid("unknown assignment action")
	}
	_, err = tx.Exec(ctx, `UPDATE learning_assignments SET revision=revision+1 WHERE id=$1::uuid`, a.ID)
	return err
}

type rowsQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadAttempts(ctx context.Context, q rowsQuerier, id string, puzzleCount int) ([]Attempt, error) {
	rows, err := q.Query(ctx, `SELECT id::text,attempt_number,status,submitted_payload,COALESCE(feedback,'') FROM assignment_attempts WHERE assignment_id=$1::uuid ORDER BY attempt_number`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Attempt{}
	for rows.Next() {
		var a Attempt
		a.Files = []File{}
		var raw []byte
		if err = rows.Scan(&a.ID, &a.Number, &a.Status, &raw, &a.Feedback); err != nil {
			return nil, err
		}
		var p attemptPayload
		if err = json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		a.Answer = p.Answer
		a.Puzzles = p.Puzzles
		if len(a.Puzzles) != puzzleCount {
			a.Puzzles = make([]PuzzleProgress, puzzleCount)
		}
		a.Metrics = metrics(a.Puzzles, puzzleCount, a.Status != "started")
		result = append(result, a)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range result {
		files, fileErr := q.Query(ctx, `SELECT f.id::text,f.filename,f.media_type,f.size_bytes,f.status FROM assignment_evidence e JOIN teaching_files f ON f.id=e.file_id WHERE e.assignment_attempt_id=$1::uuid AND f.status='clean' ORDER BY f.created_at`, result[i].ID)
		if fileErr != nil {
			return nil, fileErr
		}
		for files.Next() {
			var f File
			if fileErr = files.Scan(&f.ID, &f.Filename, &f.MediaType, &f.SizeBytes, &f.Status); fileErr != nil {
				files.Close()
				return nil, fileErr
			}
			result[i].Files = append(result[i].Files, f)
		}
		fileErr = files.Err()
		files.Close()
		if fileErr != nil {
			return nil, fileErr
		}
	}
	return result, nil
}
