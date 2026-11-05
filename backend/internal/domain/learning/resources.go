package learning

import (
	"context"
	"errors"
	"slices"
	"strings"

	chess "github.com/corentings/chess/v2"
	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func validateResource(c Command) error {
	if !required(c.Title, 200) || !required(c.Content, 512<<10) {
		return invalid("add a title and resource content (maximum 512 KB)")
	}
	if len(c.Tags) > 20 || len(c.LevelSlug) > 100 {
		return invalid("resource metadata is too long")
	}
	for _, tag := range c.Tags {
		if !required(tag, 80) {
			return invalid("tags must contain 1–80 characters")
		}
	}
	switch c.Kind {
	case "pgn":
		return ValidatePGN(c.Content)
	case "fen":
		if _, err := chess.FEN(c.Content); err != nil {
			return invalid("FEN position is invalid")
		}
	case "study":
		if len(c.Content) > 20000 {
			return invalid("study notes must be smaller than 20 KB")
		}
	default:
		return invalid("choose a PGN, FEN or written study")
	}
	return nil
}
func (s *Service) resourceAction(ctx context.Context, tx pgx.Tx, actor string, c Command) error {
	staff, err := staffResourceAccess(ctx, tx, actor)
	if err != nil {
		return err
	}
	if !staff {
		return ErrDenied
	}
	publish, err := permission(ctx, tx, actor, "learning.publish")
	if err != nil {
		return err
	}
	if c.Action == "createResource" {
		if err = validateResource(c); err != nil {
			return err
		}
		visibility := c.Visibility
		if visibility == "" {
			visibility = "personal"
		}
		if visibility != "personal" && visibility != "shared" {
			return invalid("resource visibility must be personal or shared")
		}
		status := c.Status
		if status == "" {
			status = "draft"
		}
		if status != "draft" && status != "published" {
			return invalid("new resources must be draft or published")
		}
		if visibility == "shared" && status == "published" && !publish {
			return ErrDenied
		}
		tags := c.Tags
		if tags == nil {
			tags = []string{}
		}
		resourceID := platformid.New()
		_, err = tx.Exec(ctx, `INSERT INTO teaching_resources(id,owner_id,title,kind,visibility,status,level_slug,tags,content) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9)`, resourceID, actor, c.Title, c.Kind, visibility, status, c.LevelSlug, tags, c.Content)
		if err == nil && status == "published" {
			err = snapshotResourceVersion(ctx, tx, resourceID)
		}
		return err
	}
	var owner, status, visibility, fileID string
	var version int
	err = tx.QueryRow(ctx, `SELECT owner_id::text,status,visibility,version,COALESCE(file_id::text,'') FROM teaching_resources WHERE id=$1::uuid FOR UPDATE`, c.ID).Scan(&owner, &status, &visibility, &version, &fileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != actor && !(publish && visibility == "shared") {
		return ErrDenied
	}
	if err = conflictVersion(version, c); err != nil {
		return err
	}
	switch c.Action {
	case "reviseResource":
		if status != "published" {
			return ErrConflict
		}
		if err = snapshotResourceVersion(ctx, tx, c.ID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE teaching_resources SET status='draft',version=version+1,updated_at=now() WHERE id=$1::uuid`, c.ID)
	case "editResource":
		if status != "draft" {
			return ErrConflict
		}
		if fileID != "" {
			// A uploaded object is immutable. Metadata may change, its file and content may not.
			var kind, content string
			if err = tx.QueryRow(ctx, `SELECT kind,content FROM teaching_resources WHERE id=$1::uuid`, c.ID).Scan(&kind, &content); err != nil {
				return err
			}
			if c.Kind != kind || c.Content != content || !required(c.Title, 200) || len(c.Tags) > 20 || len(c.LevelSlug) > 100 {
				return invalid("uploaded file content is immutable; upload a new file to replace it")
			}
			for _, tag := range c.Tags {
				if !required(tag, 80) {
					return invalid("tags must contain 1–80 characters")
				}
			}
		} else if err = validateResource(c); err != nil {
			return err
		}
		tags := c.Tags
		if tags == nil {
			tags = []string{}
		}
		_, err = tx.Exec(ctx, `UPDATE teaching_resources SET title=$2,kind=$3,level_slug=$4,tags=$5,content=$6,version=version+1,updated_at=now() WHERE id=$1::uuid`, c.ID, c.Title, c.Kind, c.LevelSlug, tags, c.Content)
	case "publishResource":
		if status != "draft" {
			return ErrConflict
		}
		if visibility == "shared" && !publish {
			return ErrDenied
		}
		_, err = tx.Exec(ctx, `UPDATE teaching_resources SET status='published',version=version+1,updated_at=now() WHERE id=$1::uuid`, c.ID)
	case "archiveResource":
		if status == "archived" {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE teaching_resources SET status='archived',version=version+1,updated_at=now() WHERE id=$1::uuid`, c.ID)
	}
	if err == nil && c.Action == "publishResource" {
		err = snapshotResourceVersion(ctx, tx, c.ID)
	}
	return err
}
func snapshotResourceVersion(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO teaching_resource_versions(resource_id,version,snapshot) SELECT id,version,to_jsonb(r) FROM teaching_resources r WHERE id=$1::uuid ON CONFLICT DO NOTHING`, id)
	return err
}
func (s *Service) gameAction(ctx context.Context, tx pgx.Tx, actor string, c Command) error {
	if c.Action == "saveGame" {
		if !required(c.Title, 200) {
			return invalid("game title is required")
		}
		if err := ValidatePGN(c.PGN); err != nil {
			return err
		}
		// One history record represents one game; resource files can contain many.
		scanner := chess.NewScanner(strings.NewReader(c.PGN))
		if _, err := scanner.ParseNext(); err != nil {
			return invalid("game is invalid")
		}
		if scanner.HasNext() {
			return invalid("save one game at a time")
		}
		if err := authorizeStudent(ctx, tx, actor, c.StudentID, "learning.manage", "", false); err != nil {
			if !errors.Is(err, ErrDenied) {
				return err
			}
			if err = ownStudent(ctx, tx, actor, c.StudentID); err != nil {
				return err
			}
		}
		source := c.Source
		if source == "" {
			source = "imported"
		}
		if !slices.Contains([]string{"imported", "lesson", "homework"}, source) {
			return invalid("game source is invalid")
		}
		played := nowUTC()
		if c.PlayedAt != nil {
			played = *c.PlayedAt
		}
		if c.ClassID != "" {
			var related bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM session_assignments a JOIN student_profiles s ON s.user_id=a.user_id WHERE a.class_session_id=$1::uuid AND s.id=$2::uuid AND a.role='student')`, c.ClassID, c.StudentID).Scan(&related); err != nil {
				return err
			}
			if !related {
				return invalid("game class does not belong to the learner")
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO student_games(id,student_id,class_id,owner_id,title,source,pgn,played_at,shared) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7,$8,$9)`, platformid.New(), c.StudentID, nullID(c.ClassID), actor, c.Title, source, c.PGN, played, c.Shared)
		return err
	}
	var student, pgn string
	var version int
	err := tx.QueryRow(ctx, `SELECT student_id::text,pgn,version FROM student_games WHERE id=$1::uuid FOR UPDATE`, c.ID).Scan(&student, &pgn, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err = authorizeStudent(ctx, tx, actor, student, "learning.manage", "", false); err != nil {
		return err
	}
	if err = conflictVersion(version, c); err != nil {
		return err
	}
	if !required(c.Feedback, 10000) {
		return invalid("add review feedback")
	}
	if c.PGN != "" {
		if err = ValidatePGN(c.PGN); err != nil {
			return err
		}
		pgn = c.PGN
	}
	_, err = tx.Exec(ctx, `UPDATE student_games SET pgn=$2,review=$3,shared=$4,version=version+1,updated_at=now() WHERE id=$1::uuid`, c.ID, pgn, c.Feedback, c.Shared)
	if err != nil {
		return err
	}
	if c.Shared {
		return notifyStudent(ctx, tx, student, "Your coach shared a game review", "/portal/games")
	}
	return nil
}
