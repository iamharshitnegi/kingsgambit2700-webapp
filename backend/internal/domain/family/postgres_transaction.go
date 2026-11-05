package family

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// CreateInTx stores a newly provisioned guardian relationship and its evidence
// in the same transaction as the student profile. The caller authorizes creation.
func CreateInTx(ctx context.Context, tx pgx.Tx, relationship Relationship, event Event) error {
	if relationship.Revision() != 1 || event.Revision != 1 {
		return ErrInvalidCommand
	}
	if err := validateEvent(relationship, event); err != nil {
		return err
	}
	if err := insertRelationship(ctx, tx, relationship); err != nil {
		return err
	}
	return insertRelationshipEvent(ctx, tx, relationship, event)
}
