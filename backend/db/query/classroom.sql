-- name: GetClassroomState :one
SELECT class_session_id, revision, fen, moves, mode, active_actor_user_id,
       board_status, updated_at
FROM classroom_states
WHERE class_session_id = $1;

-- name: CommitClassroomStateCAS :execrows
UPDATE classroom_states
SET revision = $3, fen = $4, moves = $5, mode = $6,
    active_actor_user_id = $7, board_status = $8, updated_at = $9
WHERE class_session_id = $1 AND revision = $2;

-- name: ListClassroomEventsAfter :many
SELECT payload
FROM classroom_events
WHERE class_session_id = $1 AND revision > $2
ORDER BY revision;
