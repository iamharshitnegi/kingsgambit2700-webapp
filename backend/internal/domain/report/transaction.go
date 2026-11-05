package report

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// LoadInTransaction lets related learning evidence share the report's lock and
// transaction. Callers must authorize the actor before exposing the record.
func LoadInTransaction(ctx context.Context, tx pgx.Tx, id string, lock bool) (Report, error) {
	return loadReport(ctx, tx, id, lock)
}

// SaveInTransaction preserves the normal immutable-version and append-only
// audit rules while committing rubric evidence in the caller's transaction.
func SaveInTransaction(ctx context.Context, tx pgx.Tx, r Report, expectedRevision uint64) error {
	if err := r.validate(); err != nil {
		return err
	}
	var mutation reportMutation
	var err error
	if expectedRevision == 0 {
		mutation, err = insertReport(ctx, tx, r)
	} else {
		mutation, err = updateReport(ctx, tx, r, expectedRevision)
	}
	if err != nil {
		return err
	}
	return insertReportAudit(ctx, tx, r, mutation)
}
