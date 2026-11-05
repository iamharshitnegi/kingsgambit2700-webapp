package classroom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type PostgresRepository struct {
	pool             *pgxpool.Pool
	snapshotInterval uint64
}

func NewPostgresRepository(pool *pgxpool.Pool, snapshotInterval uint64) (*PostgresRepository, error) {
	if pool == nil || snapshotInterval == 0 {
		return nil, fmt.Errorf("%w: pool and snapshot interval are required", ErrInvalidBoard)
	}
	return &PostgresRepository{pool: pool, snapshotInterval: snapshotInterval}, nil
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Create(ctx context.Context, state State) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if state.Revision != 0 {
		return fmt.Errorf("%w: initial revision must be zero", ErrInvalidBoard)
	}
	serialized, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("classroom repository: encoding initial state: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("classroom repository: beginning create: %w", err)
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	lesson, err := json.Marshal(cloneState(state).Lesson)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO classroom_states
			(class_session_id, revision, fen, moves, mode, active_actor_user_id, board_status, updated_at, lesson_state)
		VALUES ($1::uuid, 0, $2, $3, $4, NULLIF($5, '')::uuid, $6, $7, $8)`,
		state.SessionID, state.FEN, state.Moves, state.Mode, state.ActiveActorID, state.Status, now, lesson)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrBoardExists
		}
		return fmt.Errorf("classroom repository: inserting state: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO classroom_snapshots (id, class_session_id, revision, state, created_at)
		VALUES ($1::uuid, $2::uuid, 0, $3, $4)`, platformid.New(), state.SessionID, serialized, now)
	if err != nil {
		return fmt.Errorf("classroom repository: inserting initial snapshot: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("classroom repository: committing create: %w", err)
	}
	return nil
}

func (r *PostgresRepository) State(ctx context.Context, sessionID string) (State, error) {
	return scanState(r.pool.QueryRow(ctx, `
		SELECT class_session_id::text, fen, moves, mode,
		       COALESCE(active_actor_user_id::text, ''), revision, board_status, lesson_state
		FROM classroom_states WHERE class_session_id = $1::uuid`, sessionID))
}

func (r *PostgresRepository) Commit(ctx context.Context, commit Commit) (State, error) {
	if err := commit.State.Validate(); err != nil {
		return State{}, err
	}
	if err := commit.Event.validate(commit.State); err != nil {
		return State{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return State{}, fmt.Errorf("classroom repository: beginning commit: %w", err)
	}
	defer tx.Rollback(ctx)
	current, err := stateInTx(ctx, tx, commit.State.SessionID)
	if err != nil {
		return State{}, err
	}
	if current.Revision != commit.ExpectedRevision {
		return State{}, &RevisionConflictError{Expected: commit.ExpectedRevision, Current: current.Revision}
	}
	if err := validateTransition(current, commit.State, commit.Event); err != nil {
		return State{}, err
	}
	lesson, err := json.Marshal(commit.State.Lesson)
	if err != nil {
		return State{}, err
	}
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `
		UPDATE classroom_states
		SET revision = $3, fen = $4, moves = $5, mode = $6,
		    active_actor_user_id = NULLIF($7, '')::uuid,
		    board_status = $8, updated_at = $9, lesson_state = $10
		WHERE class_session_id = $1::uuid AND revision = $2`,
		commit.State.SessionID, commit.ExpectedRevision, commit.State.Revision,
		commit.State.FEN, commit.State.Moves, commit.State.Mode,
		commit.State.ActiveActorID, commit.State.Status, now, lesson)
	if err != nil {
		if isSerializationFailure(err) {
			return State{}, ErrRevisionConflict
		}
		return State{}, fmt.Errorf("classroom repository: updating state: %w", err)
	}
	if result.RowsAffected() != 1 {
		current, loadErr := stateInTx(ctx, tx, commit.State.SessionID)
		if loadErr != nil {
			return State{}, loadErr
		}
		return State{}, &RevisionConflictError{Expected: commit.ExpectedRevision, Current: current.Revision}
	}
	payload, err := json.Marshal(commit.Event)
	if err != nil {
		return State{}, fmt.Errorf("classroom repository: encoding event: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO classroom_events
			(id, class_session_id, revision, event_type, actor_user_id, payload, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, NULLIF($5, '')::uuid, $6, $7)`,
		commit.Event.ID, commit.Event.SessionID, commit.Event.Revision,
		commit.Event.Kind, commit.Event.ActorID, payload, commit.Event.OccurredAt)
	if err != nil {
		if isUniqueViolation(err) {
			return State{}, ErrEventConflict
		}
		return State{}, fmt.Errorf("classroom repository: inserting event: %w", err)
	}
	if commit.State.Revision%r.snapshotInterval == 0 {
		snapshot, err := json.Marshal(commit.State)
		if err != nil {
			return State{}, fmt.Errorf("classroom repository: encoding snapshot: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO classroom_snapshots (id, class_session_id, revision, state, created_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
			platformid.New(), commit.State.SessionID, commit.State.Revision, snapshot, now); err != nil {
			return State{}, fmt.Errorf("classroom repository: inserting snapshot: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		if isSerializationFailure(err) {
			return State{}, ErrRevisionConflict
		}
		return State{}, fmt.Errorf("classroom repository: committing state: %w", err)
	}
	return cloneState(commit.State), nil
}

func (r *PostgresRepository) Recovery(ctx context.Context, sessionID string, afterRevision uint64) (Recovery, error) {
	current, err := r.State(ctx, sessionID)
	if err != nil {
		return Recovery{}, err
	}
	if afterRevision > current.Revision {
		return Recovery{}, &RevisionConflictError{Expected: afterRevision, Current: current.Revision}
	}
	var raw []byte
	err = r.pool.QueryRow(ctx, `
		SELECT state FROM classroom_snapshots
		WHERE class_session_id = $1::uuid
		ORDER BY revision DESC LIMIT 1`, sessionID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recovery{}, ErrBoardNotFound
	}
	if err != nil {
		return Recovery{}, fmt.Errorf("classroom repository: selecting snapshot: %w", err)
	}
	var snapshot State
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Recovery{}, fmt.Errorf("classroom repository: decoding snapshot: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT payload FROM classroom_events
		WHERE class_session_id = $1::uuid AND revision > $2
		ORDER BY revision`, sessionID, snapshot.Revision)
	if err != nil {
		return Recovery{}, fmt.Errorf("classroom repository: selecting events: %w", err)
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var eventRaw []byte
		if err := rows.Scan(&eventRaw); err != nil {
			return Recovery{}, fmt.Errorf("classroom repository: scanning event: %w", err)
		}
		var event Event
		if err := json.Unmarshal(eventRaw, &event); err != nil {
			return Recovery{}, fmt.Errorf("classroom repository: decoding event: %w", err)
		}
		events = append(events, event)
	}
	return Recovery{Snapshot: cloneState(snapshot), Events: events}, rows.Err()
}

type stateScanner interface{ Scan(...any) error }

func scanState(row stateScanner) (State, error) {
	var state State
	var lesson []byte
	err := row.Scan(&state.SessionID, &state.FEN, &state.Moves, &state.Mode, &state.ActiveActorID, &state.Revision, &state.Status, &lesson)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{}, ErrBoardNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("classroom repository: scanning state: %w", err)
	}
	if err := json.Unmarshal(lesson, &state.Lesson); err != nil {
		return State{}, fmt.Errorf("classroom repository: decoding lesson: %w", err)
	}
	return cloneState(state), nil
}

func stateInTx(ctx context.Context, tx pgx.Tx, sessionID string) (State, error) {
	return scanState(tx.QueryRow(ctx, `
		SELECT class_session_id::text, fen, moves, mode,
		       COALESCE(active_actor_user_id::text, ''), revision, board_status, lesson_state
		FROM classroom_states WHERE class_session_id = $1::uuid`, sessionID))
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01")
}
