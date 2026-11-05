package service

import (
	"context"
	"testing"

	"github.com/kingsgambit2700/platform/backend/internal/lead/model"
	"github.com/kingsgambit2700/platform/backend/internal/lead/repository"
)

func TestCreateDeduplicatesNormalizedContact(t *testing.T) {
	svc := New(repository.NewMemory())
	input := model.CreateInput{
		GuardianName:   "Parent",
		Email:          "PARENT@example.com",
		Phone:          "+91 99999 99999",
		StudentAgeBand: "8_12",
		Country:        "in",
		TimeZone:       "Asia/Kolkata",
		ContactConsent: true,
	}
	first, created, err := svc.Create(context.Background(), input)
	if err != nil || !created {
		t.Fatalf("first Create() = (%v, %v)", created, err)
	}
	second, created, err := svc.Create(context.Background(), input)
	if err != nil || created {
		t.Fatalf("second Create() = (%v, %v)", created, err)
	}
	if first.ID != second.ID {
		t.Fatalf("dedupe IDs = %q, %q", first.ID, second.ID)
	}
}

func TestCreateRequiresConsentAndTimezone(t *testing.T) {
	svc := New(repository.NewMemory())
	_, _, err := svc.Create(context.Background(), model.CreateInput{
		GuardianName:   "Parent",
		Email:          "parent@example.com",
		StudentAgeBand: "8_12",
		Country:        "IN",
		TimeZone:       "not/a-zone",
	})
	if err == nil {
		t.Fatal("Create() expected validation error")
	}
}
