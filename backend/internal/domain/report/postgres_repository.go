package report

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

const (
	reportObjectType        = "progress_report"
	actionDraftCreated      = "report.draft_created"
	actionDraftEdited       = "report.draft_edited"
	actionPublished         = "report.published"
	actionCorrectionStarted = "report.correction_started"
	actionViewed            = "report.viewed"
	actionAcknowledged      = "report.acknowledged"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("report repository: pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Report(ctx context.Context, reportID string) (Report, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return Report{}, fmt.Errorf("report repository: beginning read: %w", err)
	}
	defer tx.Rollback(ctx)

	report, err := loadReport(ctx, tx, reportID, false)
	if err != nil {
		return Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Report{}, fmt.Errorf("report repository: committing read: %w", err)
	}
	return report, nil
}

func (r *PostgresRepository) Save(
	ctx context.Context,
	report Report,
	expectedRevision uint64,
) error {
	if err := report.validate(); err != nil {
		return err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("report repository: beginning save: %w", err)
	}
	defer tx.Rollback(ctx)

	var mutation reportMutation
	if expectedRevision == 0 {
		mutation, err = insertReport(ctx, tx, report)
	} else {
		mutation, err = updateReport(ctx, tx, report, expectedRevision)
	}
	if err != nil {
		return err
	}
	if err := insertReportAudit(ctx, tx, report, mutation); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if isPostgresConflict(err) {
			return ErrConflict
		}
		return fmt.Errorf("report repository: committing save: %w", err)
	}
	return nil
}

type reportQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type reportRow struct {
	id        string
	studentID string
	period    Period
	createdAt time.Time
}

func loadReport(
	ctx context.Context,
	db reportQueryer,
	reportID string,
	isLocked bool,
) (Report, error) {
	query := `
		SELECT id::text, student_profile_id::text, reporting_period_start,
		       reporting_period_end, created_at
		FROM progress_reports
		WHERE id = $1::uuid`
	if isLocked {
		query += " FOR UPDATE"
	}

	var row reportRow
	err := db.QueryRow(ctx, query, reportID).Scan(
		&row.id,
		&row.studentID,
		&row.period.Start,
		&row.period.End,
		&row.createdAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, fmt.Errorf("report repository: scanning report: %w", err)
	}

	versions, err := loadVersions(ctx, db, reportID)
	if err != nil {
		return Report{}, err
	}
	audit, err := loadReportAudit(ctx, db, reportID)
	if err != nil {
		return Report{}, err
	}
	if len(audit) == 0 || audit[0].Action != actionDraftCreated {
		return Report{}, fmt.Errorf(
			"report repository: restoring report: %w: creation audit event is missing",
			ErrInvalidReport,
		)
	}
	for index, event := range audit {
		if event.Metadata.Revision != uint64(index+1) {
			return Report{}, fmt.Errorf(
				"report repository: restoring report: %w: non-sequential audit revision",
				ErrInvalidReport,
			)
		}
	}

	restored, err := Restore(Record{
		ID:        row.id,
		StudentID: row.studentID,
		Period:    row.period,
		Versions:  versions,
		Receipts:  receiptsFromAudit(audit),
		Revision:  uint64(len(audit)),
		CreatedAt: row.createdAt,
	})
	if err != nil {
		return Report{}, fmt.Errorf("report repository: restoring report: %w", err)
	}
	return restored, nil
}

type storedVersion struct {
	SchemaVersion   uint8     `json:"schemaVersion"`
	Content         Content   `json:"content"`
	RubricVersionID string    `json:"rubricVersionId"`
	SourceVersionID string    `json:"sourceVersionId,omitempty"`
	UpdatedBy       string    `json:"updatedBy"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func loadVersions(
	ctx context.Context,
	db reportQueryer,
	reportID string,
) ([]Version, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, version, content, author_user_id::text,
		       COALESCE(published_by::text, ''), published_at, created_at
		FROM progress_report_versions
		WHERE progress_report_id = $1::uuid
		ORDER BY version`,
		reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("report repository: selecting report versions: %w", err)
	}
	defer rows.Close()

	versions := make([]Version, 0)
	for rows.Next() {
		var version Version
		var number int32
		var rawContent []byte
		if err := rows.Scan(
			&version.ID,
			&number,
			&rawContent,
			&version.CreatedBy,
			&version.PublishedBy,
			&version.PublishedAt,
			&version.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("report repository: scanning report version: %w", err)
		}
		if number <= 0 {
			return nil, fmt.Errorf("report repository: invalid report version number %d", number)
		}
		version.Number = uint32(number)

		var stored storedVersion
		if err := json.Unmarshal(rawContent, &stored); err != nil {
			return nil, fmt.Errorf("report repository: decoding report version: %w", err)
		}
		if stored.SchemaVersion != 1 {
			return nil, fmt.Errorf(
				"report repository: unsupported stored report schema %d",
				stored.SchemaVersion,
			)
		}
		version.Content = stored.Content
		version.RubricVersionID = stored.RubricVersionID
		version.SourceVersionID = stored.SourceVersionID
		version.UpdatedBy = stored.UpdatedBy
		version.UpdatedAt = stored.UpdatedAt
		version.Status = VersionStatusDraft
		if version.PublishedAt != nil {
			version.Status = VersionStatusPublished
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report repository: iterating report versions: %w", err)
	}
	return versions, nil
}

type reportAuditMetadata struct {
	Revision       uint64     `json:"revision"`
	VersionID      string     `json:"versionId,omitempty"`
	ViewerID       string     `json:"viewerId,omitempty"`
	FirstViewedAt  *time.Time `json:"firstViewedAt,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

type reportAuditEvent struct {
	Action     string
	ActorID    string
	OccurredAt time.Time
	Metadata   reportAuditMetadata
}

func loadReportAudit(
	ctx context.Context,
	db reportQueryer,
	reportID string,
) ([]reportAuditEvent, error) {
	rows, err := db.Query(ctx, `
		SELECT action, COALESCE(actor_id::text, ''), occurred_at, metadata
		FROM audit_events
		WHERE object_type = $1 AND object_id = $2
		  AND action IN ($3, $4, $5, $6, $7, $8)
		ORDER BY (metadata->>'revision')::bigint`,
		reportObjectType,
		reportID,
		actionDraftCreated,
		actionDraftEdited,
		actionPublished,
		actionCorrectionStarted,
		actionViewed,
		actionAcknowledged,
	)
	if err != nil {
		return nil, fmt.Errorf("report repository: selecting report audit: %w", err)
	}
	defer rows.Close()

	events := make([]reportAuditEvent, 0)
	for rows.Next() {
		var event reportAuditEvent
		var rawMetadata []byte
		if err := rows.Scan(
			&event.Action,
			&event.ActorID,
			&event.OccurredAt,
			&rawMetadata,
		); err != nil {
			return nil, fmt.Errorf("report repository: scanning report audit: %w", err)
		}
		if err := json.Unmarshal(rawMetadata, &event.Metadata); err != nil {
			return nil, fmt.Errorf("report repository: decoding report audit: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report repository: iterating report audit: %w", err)
	}
	return events, nil
}

func receiptsFromAudit(events []reportAuditEvent) []Receipt {
	receipts := make([]Receipt, 0)
	for _, event := range events {
		if event.Action != actionViewed && event.Action != actionAcknowledged {
			continue
		}
		index := receiptIndex(receipts, event.Metadata.VersionID, event.Metadata.ViewerID)
		if index < 0 {
			firstViewedAt := event.OccurredAt
			if event.Metadata.FirstViewedAt != nil {
				firstViewedAt = *event.Metadata.FirstViewedAt
			}
			receipts = append(receipts, Receipt{
				VersionID:     event.Metadata.VersionID,
				ViewerID:      event.Metadata.ViewerID,
				FirstViewedAt: firstViewedAt,
			})
			index = len(receipts) - 1
		}
		if event.Action == actionAcknowledged {
			acknowledgedAt := event.OccurredAt
			if event.Metadata.AcknowledgedAt != nil {
				acknowledgedAt = *event.Metadata.AcknowledgedAt
			}
			receipts[index].AcknowledgedAt = cloneTime(&acknowledgedAt)
		}
	}
	return receipts
}

func insertReport(ctx context.Context, tx pgx.Tx, report Report) (reportMutation, error) {
	if report.Revision() != 1 || len(report.Versions()) != 1 {
		return reportMutation{}, ErrConflict
	}
	record := report.Record()
	version := record.Versions[0]
	_, err := tx.Exec(ctx, `
		INSERT INTO progress_reports
			(id, student_profile_id, reporting_period_start, reporting_period_end,
			 status, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, 'draft', $5, $5)`,
		record.ID,
		record.StudentID,
		record.Period.Start,
		record.Period.End,
		record.CreatedAt,
	)
	if isPostgresConflict(err) {
		return reportMutation{}, ErrConflict
	}
	if err != nil {
		return reportMutation{}, fmt.Errorf("report repository: inserting report: %w", err)
	}
	if err := insertVersion(ctx, tx, report.ID(), version); err != nil {
		return reportMutation{}, err
	}
	return reportMutation{
		action:     actionDraftCreated,
		actorID:    version.CreatedBy,
		occurredAt: version.CreatedAt,
		versionID:  version.ID,
	}, nil
}

func updateReport(
	ctx context.Context,
	tx pgx.Tx,
	updated Report,
	expectedRevision uint64,
) (reportMutation, error) {
	current, err := loadReport(ctx, tx, updated.ID(), true)
	if err != nil {
		return reportMutation{}, err
	}
	if current.Revision() != expectedRevision || updated.Revision() != expectedRevision+1 {
		return reportMutation{}, ErrConflict
	}
	if current.StudentID() != updated.StudentID() || current.Period() != updated.Period() {
		return reportMutation{}, fmt.Errorf("report repository: report identity cannot change: %w", ErrConflict)
	}
	if publishedVersionsChanged(current, updated) {
		return reportMutation{}, ErrPublishedImmutable
	}

	mutation, err := describeMutation(current, updated)
	if err != nil {
		return reportMutation{}, err
	}
	if err := persistVersions(ctx, tx, current, updated); err != nil {
		return reportMutation{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE progress_reports
		SET status = $2, updated_at = $3
		WHERE id = $1::uuid`,
		updated.ID(),
		reportStatus(updated),
		mutation.occurredAt,
	)
	if err != nil {
		return reportMutation{}, fmt.Errorf("report repository: updating report: %w", err)
	}
	return mutation, nil
}

func persistVersions(
	ctx context.Context,
	tx pgx.Tx,
	current Report,
	updated Report,
) error {
	currentVersions := current.Versions()
	updatedVersions := updated.Versions()
	if len(updatedVersions) < len(currentVersions) || len(updatedVersions) > len(currentVersions)+1 {
		return fmt.Errorf("report repository: invalid version count change: %w", ErrConflict)
	}

	for index, currentVersion := range currentVersions {
		candidate := updatedVersions[index]
		if currentVersion.ID != candidate.ID || currentVersion.Number != candidate.Number ||
			currentVersion.CreatedBy != candidate.CreatedBy ||
			!currentVersion.CreatedAt.Equal(candidate.CreatedAt) {
			return fmt.Errorf("report repository: version identity cannot change: %w", ErrConflict)
		}
		if sameVersion(currentVersion, candidate) {
			continue
		}
		if currentVersion.Status == VersionStatusPublished {
			return ErrPublishedImmutable
		}
		if err := updateVersion(ctx, tx, candidate); err != nil {
			return err
		}
	}
	if len(updatedVersions) == len(currentVersions)+1 {
		if err := insertVersion(
			ctx,
			tx,
			updated.ID(),
			updatedVersions[len(updatedVersions)-1],
		); err != nil {
			return err
		}
	}
	return nil
}

func insertVersion(
	ctx context.Context,
	tx pgx.Tx,
	reportID string,
	version Version,
) error {
	content, err := encodeVersion(version)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO progress_report_versions
			(id, progress_report_id, version, content, author_user_id,
			 published_by, published_at, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid,
		        NULLIF($6, '')::uuid, $7, $8)`,
		version.ID,
		reportID,
		version.Number,
		content,
		version.CreatedBy,
		version.PublishedBy,
		version.PublishedAt,
		version.CreatedAt,
	)
	if isPostgresConflict(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("report repository: inserting report version: %w", err)
	}
	return nil
}

func updateVersion(ctx context.Context, tx pgx.Tx, version Version) error {
	content, err := encodeVersion(version)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		UPDATE progress_report_versions
		SET content = $2, published_by = NULLIF($3, '')::uuid, published_at = $4
		WHERE id = $1::uuid AND published_at IS NULL`,
		version.ID,
		content,
		version.PublishedBy,
		version.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("report repository: updating report version: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func encodeVersion(version Version) ([]byte, error) {
	encoded, err := json.Marshal(storedVersion{
		SchemaVersion:   1,
		Content:         version.Content,
		RubricVersionID: version.RubricVersionID,
		SourceVersionID: version.SourceVersionID,
		UpdatedBy:       version.UpdatedBy,
		UpdatedAt:       version.UpdatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("report repository: encoding report version: %w", err)
	}
	return encoded, nil
}

type reportMutation struct {
	action         string
	actorID        string
	occurredAt     time.Time
	versionID      string
	viewerID       string
	firstViewedAt  *time.Time
	acknowledgedAt *time.Time
}

func describeMutation(current, updated Report) (reportMutation, error) {
	currentVersions := current.Versions()
	updatedVersions := updated.Versions()
	if len(updatedVersions) == len(currentVersions)+1 {
		if !sameReceipts(current.Receipts(), updated.Receipts()) {
			return reportMutation{}, fmt.Errorf(
				"report repository: receipt changed with correction: %w",
				ErrConflict,
			)
		}
		version := updatedVersions[len(updatedVersions)-1]
		return reportMutation{
			action:     actionCorrectionStarted,
			actorID:    version.CreatedBy,
			occurredAt: version.CreatedAt,
			versionID:  version.ID,
		}, nil
	}

	changedVersion := -1
	for index := range currentVersions {
		if !sameVersion(currentVersions[index], updatedVersions[index]) {
			if changedVersion >= 0 {
				return reportMutation{}, fmt.Errorf(
					"report repository: multiple versions changed: %w",
					ErrConflict,
				)
			}
			changedVersion = index
		}
	}
	if changedVersion >= 0 {
		if !sameReceipts(current.Receipts(), updated.Receipts()) {
			return reportMutation{}, fmt.Errorf(
				"report repository: receipt changed with version: %w",
				ErrConflict,
			)
		}
		before := currentVersions[changedVersion]
		after := updatedVersions[changedVersion]
		action := actionDraftEdited
		actorID := after.UpdatedBy
		occurredAt := after.UpdatedAt
		if before.Status == VersionStatusDraft && after.Status == VersionStatusPublished {
			action = actionPublished
			actorID = after.PublishedBy
			if after.PublishedAt == nil {
				return reportMutation{}, fmt.Errorf(
					"report repository: publication time is missing: %w",
					ErrInvalidReport,
				)
			}
			occurredAt = *after.PublishedAt
		}
		return reportMutation{
			action:     action,
			actorID:    actorID,
			occurredAt: occurredAt,
			versionID:  after.ID,
		}, nil
	}

	return describeReceiptMutation(current.Receipts(), updated.Receipts())
}

func describeReceiptMutation(
	current []Receipt,
	updated []Receipt,
) (reportMutation, error) {
	if len(updated) < len(current) || len(updated) > len(current)+1 {
		return reportMutation{}, fmt.Errorf(
			"report repository: invalid receipt count change: %w",
			ErrConflict,
		)
	}
	for _, receipt := range current {
		if receiptIndex(updated, receipt.VersionID, receipt.ViewerID) < 0 {
			return reportMutation{}, fmt.Errorf(
				"report repository: receipt cannot be removed: %w",
				ErrConflict,
			)
		}
	}
	var mutation *reportMutation
	for _, candidate := range updated {
		index := receiptIndex(current, candidate.VersionID, candidate.ViewerID)
		if index < 0 {
			action := actionViewed
			occurredAt := candidate.FirstViewedAt
			if candidate.AcknowledgedAt != nil {
				action = actionAcknowledged
				occurredAt = *candidate.AcknowledgedAt
			}
			candidateMutation := reportMutation{
				action:         action,
				actorID:        candidate.ViewerID,
				occurredAt:     occurredAt,
				versionID:      candidate.VersionID,
				viewerID:       candidate.ViewerID,
				firstViewedAt:  cloneTime(&candidate.FirstViewedAt),
				acknowledgedAt: cloneTime(candidate.AcknowledgedAt),
			}
			if mutation != nil {
				return reportMutation{}, fmt.Errorf(
					"report repository: multiple receipts changed: %w",
					ErrConflict,
				)
			}
			mutation = &candidateMutation
			continue
		}
		before := current[index]
		isAcknowledgedNow := before.AcknowledgedAt == nil && candidate.AcknowledgedAt != nil
		if isAcknowledgedNow {
			if !before.FirstViewedAt.Equal(candidate.FirstViewedAt) {
				return reportMutation{}, fmt.Errorf(
					"report repository: first view cannot change: %w",
					ErrConflict,
				)
			}
			candidateMutation := reportMutation{
				action:         actionAcknowledged,
				actorID:        candidate.ViewerID,
				occurredAt:     *candidate.AcknowledgedAt,
				versionID:      candidate.VersionID,
				viewerID:       candidate.ViewerID,
				firstViewedAt:  cloneTime(&candidate.FirstViewedAt),
				acknowledgedAt: cloneTime(candidate.AcknowledgedAt),
			}
			if mutation != nil {
				return reportMutation{}, fmt.Errorf(
					"report repository: multiple receipts changed: %w",
					ErrConflict,
				)
			}
			mutation = &candidateMutation
			continue
		}
		if !sameReceipt(before, candidate) {
			return reportMutation{}, fmt.Errorf(
				"report repository: receipt is immutable: %w",
				ErrConflict,
			)
		}
	}
	if mutation != nil {
		return *mutation, nil
	}
	return reportMutation{}, fmt.Errorf("report repository: no supported mutation: %w", ErrConflict)
}

func insertReportAudit(
	ctx context.Context,
	tx pgx.Tx,
	report Report,
	mutation reportMutation,
) error {
	metadata, err := json.Marshal(reportAuditMetadata{
		Revision:       report.Revision(),
		VersionID:      mutation.versionID,
		ViewerID:       mutation.viewerID,
		FirstViewedAt:  mutation.firstViewedAt,
		AcknowledgedAt: mutation.acknowledgedAt,
	})
	if err != nil {
		return fmt.Errorf("report repository: encoding report audit: %w", err)
	}
	recordJSON, err := json.Marshal(report.Record())
	if err != nil {
		return fmt.Errorf("report repository: encoding report digest: %w", err)
	}
	afterDigest := sha256.Sum256(recordJSON)
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, actor_id, action, object_type,
			 object_id, scope_id, after_digest, metadata)
		VALUES ($1::uuid, $2, 'user', $3::uuid, $4, $5, $6, $7::uuid, $8, $9)`,
		platformid.New(),
		mutation.occurredAt,
		mutation.actorID,
		mutation.action,
		reportObjectType,
		report.ID(),
		report.StudentID(),
		afterDigest[:],
		metadata,
	)
	if isPostgresConflict(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("report repository: inserting report audit: %w", err)
	}
	return nil
}

func reportStatus(report Report) string {
	hasPublished := false
	for _, version := range report.Versions() {
		if version.Status == VersionStatusDraft {
			if hasPublished {
				return "review"
			}
			return "draft"
		}
		hasPublished = true
	}
	if hasPublished {
		return "published"
	}
	return "draft"
}

func receiptIndex(receipts []Receipt, versionID, viewerID string) int {
	for index, receipt := range receipts {
		if receipt.VersionID == versionID && receipt.ViewerID == viewerID {
			return index
		}
	}
	return -1
}

func sameReceipts(left, right []Receipt) bool {
	if len(left) != len(right) {
		return false
	}
	for _, receipt := range left {
		index := receiptIndex(right, receipt.VersionID, receipt.ViewerID)
		if index < 0 || !sameReceipt(receipt, right[index]) {
			return false
		}
	}
	return true
}

func sameReceipt(left, right Receipt) bool {
	if left.VersionID != right.VersionID || left.ViewerID != right.ViewerID ||
		!left.FirstViewedAt.Equal(right.FirstViewedAt) {
		return false
	}
	if left.AcknowledgedAt == nil || right.AcknowledgedAt == nil {
		return left.AcknowledgedAt == nil && right.AcknowledgedAt == nil
	}
	return left.AcknowledgedAt.Equal(*right.AcknowledgedAt)
}

func isPostgresConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" || pgErr.Code == "40001"
}
