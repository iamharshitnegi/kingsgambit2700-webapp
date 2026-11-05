package ticket

import (
	"context"
	"errors"
	"testing"
)

func TestService_ConsumeRejectsInvalidIdentityBeforeDatabaseAccess(t *testing.T) {
	t.Parallel()

	service := &Service{}
	if _, err := service.Consume(context.Background(), "opaque-ticket", "not-a-uuidv7"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Consume() error = %v, want %v", err, ErrInvalid)
	}
}
