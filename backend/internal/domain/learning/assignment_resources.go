package learning

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

// Copy the selected, published version into homework so later archival never
// changes what a learner was assigned or exposes another teacher's private draft.
func snapshotResources(ctx context.Context, tx pgx.Tx, actor string, ids []string) ([]AssignedResource, error) {
	result := []AssignedResource{}
	if len(ids) > 10 {
		return nil, invalid("attach at most 10 resources")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !platformid.ValidReference(id) {
			return nil, invalid("resource reference is invalid")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		var r AssignedResource
		err := tx.QueryRow(ctx, `SELECT r.id::text,r.version,r.title,r.kind,r.content,COALESCE(r.file_id::text,''),COALESCE(f.filename,'') FROM teaching_resources r LEFT JOIN teaching_files f ON f.id=r.file_id WHERE r.id=$1::uuid AND r.status='published' AND (r.owner_id=$2::uuid OR r.visibility='shared') FOR SHARE OF r`, id, actor).Scan(&r.ID, &r.Version, &r.Title, &r.Kind, &r.Content, &r.FileID, &r.Filename)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, invalid("choose published resources from your personal or shared library")
		}
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func assignedFileAccess(ctx context.Context, q querier, actor, fileID string) (bool, error) {
	// Apply live authorization in the discovery query; a widely assigned file must
	// never lose access because unrelated learners appear earlier in a result page.
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM student_profiles s WHERE `+studentAccessSQL+` AND EXISTS(SELECT 1 FROM learning_assignments a WHERE a.student_profile_id=s.id AND a.status IN ('assigned','closed') AND EXISTS(SELECT 1 FROM jsonb_array_elements(COALESCE(a.content->'resources','[]'::jsonb)) r WHERE r->>'fileId'=$5)))`, actor, "learning.view", "view_progress", true, fileID).Scan(&allowed)
	return allowed, err
}
