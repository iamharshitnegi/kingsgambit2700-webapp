package database

import (
	"context"
	"testing"
)

func TestOpenRequiresURL(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("Open() expected error")
	}
}
