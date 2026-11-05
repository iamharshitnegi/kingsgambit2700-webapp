package report

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_PublishAndCorrectMonthlyReport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, err := NewService(NewMemoryRepository(), allowReportAccess{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	originalContent := validContent("original")
	report, err := service.CreateMonthly(ctx, CreateCommand{
		ReportID:        "report-1",
		StudentID:       "student-1",
		Year:            2026,
		Month:           time.August,
		VersionID:       "version-1",
		RubricVersionID: "rubric-v3",
		Content:         originalContent,
		ActorID:         "coach-1",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateMonthly() error = %v", err)
	}

	editedContent := validContent("reviewed")
	report, err = service.EditDraft(ctx, EditCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ExpectedRevision: report.Revision(),
		Content:          editedContent,
		ActorID:          "academic-1",
		UpdatedAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("EditDraft() error = %v", err)
	}
	report, err = service.Publish(ctx, PublishCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ExpectedRevision: report.Revision(),
		ActorID:          "academic-1",
		PublishedAt:      now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	_, err = service.EditDraft(ctx, EditCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ExpectedRevision: report.Revision(),
		Content:          validContent("mutated"),
		ActorID:          "coach-1",
		UpdatedAt:        now.Add(3 * time.Hour),
	})
	if !errors.Is(err, ErrPublishedImmutable) {
		t.Fatalf("EditDraft(published) error = %v, want %v", err, ErrPublishedImmutable)
	}

	report, err = service.StartCorrection(ctx, CorrectCommand{
		ReportID:         report.ID(),
		SourceVersionID:  "version-1",
		NewVersionID:     "version-2",
		ExpectedRevision: report.Revision(),
		ActorID:          "academic-1",
		CreatedAt:        now.Add(4 * time.Hour),
	})
	if err != nil {
		t.Fatalf("StartCorrection() error = %v", err)
	}
	versions := report.Versions()
	if len(versions) != 2 {
		t.Fatalf("Versions() length = %d, want 2", len(versions))
	}
	if versions[0].Status != VersionStatusPublished || versions[0].Content != editedContent {
		t.Fatalf("original published version changed: %+v", versions[0])
	}
	if versions[1].Status != VersionStatusDraft || versions[1].SourceVersionID != "version-1" {
		t.Fatalf("correction draft = %+v", versions[1])
	}

	correctedContent := validContent("corrected")
	report, err = service.EditDraft(ctx, EditCommand{
		ReportID:         report.ID(),
		VersionID:        "version-2",
		ExpectedRevision: report.Revision(),
		Content:          correctedContent,
		ActorID:          "academic-1",
		UpdatedAt:        now.Add(5 * time.Hour),
	})
	if err != nil {
		t.Fatalf("EditDraft(correction) error = %v", err)
	}
	report, err = service.Publish(ctx, PublishCommand{
		ReportID:         report.ID(),
		VersionID:        "version-2",
		ExpectedRevision: report.Revision(),
		ActorID:          "academic-1",
		PublishedAt:      now.Add(6 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Publish(correction) error = %v", err)
	}
	current, err := report.CurrentPublished()
	if err != nil {
		t.Fatalf("CurrentPublished() error = %v", err)
	}
	if current.ID != "version-2" || current.Content != correctedContent {
		t.Fatalf("CurrentPublished() = %+v, want corrected version", current)
	}
}

func TestService_ViewAndAcknowledgementAreIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, err := NewService(NewMemoryRepository(), allowReportAccess{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	report, err := service.CreateMonthly(ctx, CreateCommand{
		ReportID:        "report-1",
		StudentID:       "student-1",
		Year:            2026,
		Month:           time.August,
		VersionID:       "version-1",
		RubricVersionID: "rubric-v3",
		Content:         validContent("content"),
		ActorID:         "coach-1",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateMonthly() error = %v", err)
	}
	report, err = service.Publish(ctx, PublishCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ExpectedRevision: report.Revision(),
		ActorID:          "academic-1",
		PublishedAt:      now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	report, err = service.RecordView(ctx, ReceiptCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ViewerID:         "guardian-1",
		ExpectedRevision: report.Revision(),
		OccurredAt:       now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("RecordView() error = %v", err)
	}
	revisionAfterView := report.Revision()
	report, err = service.RecordView(ctx, ReceiptCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ViewerID:         "guardian-1",
		ExpectedRevision: report.Revision(),
		OccurredAt:       now.Add(3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("RecordView(retry) error = %v", err)
	}
	if report.Revision() != revisionAfterView {
		t.Fatalf("RecordView(retry) revision = %d, want %d", report.Revision(), revisionAfterView)
	}
	report, err = service.Acknowledge(ctx, ReceiptCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ViewerID:         "guardian-1",
		ExpectedRevision: report.Revision(),
		OccurredAt:       now.Add(4 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Acknowledge() error = %v", err)
	}
	receipts := report.Receipts()
	if len(receipts) != 1 || receipts[0].AcknowledgedAt == nil {
		t.Fatalf("Receipts() = %+v, want one acknowledged receipt", receipts)
	}
}

func TestService_RejectsStaleRevision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, err := NewService(NewMemoryRepository(), allowReportAccess{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	report, err := service.CreateMonthly(ctx, CreateCommand{
		ReportID:        "report-1",
		StudentID:       "student-1",
		Year:            2026,
		Month:           time.August,
		VersionID:       "version-1",
		RubricVersionID: "rubric-v3",
		Content:         validContent("content"),
		ActorID:         "coach-1",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateMonthly() error = %v", err)
	}
	_, err = service.Publish(ctx, PublishCommand{
		ReportID:         report.ID(),
		VersionID:        "version-1",
		ExpectedRevision: report.Revision() + 1,
		ActorID:          "academic-1",
		PublishedAt:      now.Add(time.Hour),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Publish(stale) error = %v, want %v", err, ErrConflict)
	}
}

func TestService_RejectsUnauthorizedReportCreation(t *testing.T) {
	t.Parallel()

	service, err := NewService(NewMemoryRepository(), denyReportAccess{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.CreateMonthly(t.Context(), CreateCommand{
		ReportID:        "report-1",
		StudentID:       "student-1",
		Year:            2026,
		Month:           time.August,
		VersionID:       "version-1",
		RubricVersionID: "rubric-v3",
		Content:         validContent("content"),
		ActorID:         "coach-1",
		CreatedAt:       time.Now().UTC(),
	})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CreateMonthly() error = %v, want %v", err, ErrPermissionDenied)
	}
}

func validContent(prefix string) Content {
	return Content{
		Learned:      Written(prefix + " learned"),
		Strengths:    Written(prefix + " strengths"),
		FocusNext:    Written(prefix + " focus next"),
		PracticePlan: Written(prefix + " practice plan"),
		CoachComment: Written(prefix + " coach comment"),
	}
}

type allowReportAccess struct{}

func (allowReportAccess) AuthorizeReport(
	context.Context,
	string,
	string,
	Action,
) error {
	return nil
}

type denyReportAccess struct{}

func (denyReportAccess) AuthorizeReport(
	context.Context,
	string,
	string,
	Action,
) error {
	return ErrPermissionDenied
}
