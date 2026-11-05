package academy

import (
	"context"
	"fmt"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Repository interface {
	Staff(context.Context, string) (StaffDirectory, error)
	UpdateStaff(context.Context, string, string, StaffUpdate, time.Time) (StaffRecord, error)
	Workspace(context.Context, string) (Workspace, error)
	CreateStudent(context.Context, string, StudentInput, time.Time) (Student, error)
	CreateStaff(context.Context, string, StaffInput, time.Time) (Coach, error)
	CreateBatch(context.Context, string, BatchInput, time.Time) (Batch, error)
	CreateEnrolment(context.Context, string, EnrolmentInput, time.Time) (Enrolment, error)
	PublishPolicy(context.Context, string, PolicyInput, time.Time) (Policy, error)
	SaveSettings(context.Context, string, Settings, time.Time) (Settings, error)
	AcceptConsent(context.Context, string, string, ConsentInput, time.Time) error
	IssueCode(context.Context, string, string, time.Time) (AccessCode, error)
	Schedule(context.Context, string, ClassInput, []time.Time, time.Time, bool) (ClassResult, error)
	UpdateClass(context.Context, string, string, ClassInput, time.Time, time.Time) (Class, error)
	Complete(context.Context, string, string, CompletionInput, time.Time) (Class, error)
	Cancel(context.Context, string, string, CancellationInput, time.Time, bool) (CancellationPreview, error)
	UpdateStudent(context.Context, string, string, StudentUpdate, time.Time) (Student, error)
	UpdateBatch(context.Context, string, string, BatchInput, time.Time) (Batch, error)
	UpdateEnrolment(context.Context, string, string, EnrolmentUpdate, time.Time) (Enrolment, error)
	Availability(context.Context, string) ([]Availability, error)
	SaveAvailability(context.Context, string, Availability, time.Time) (Availability, error)
	DeleteAvailability(context.Context, string, string, time.Time) error
	Operations(context.Context, string) (Operations, error)
	UpdateLead(context.Context, string, string, LeadUpdate, time.Time) (Lead, error)
	ConvertLead(context.Context, string, string, ConversionInput, time.Time) (Lead, error)
	CreateTrialSlot(context.Context, string, TrialSlot, time.Time) (TrialSlot, error)
	UpdateTrial(context.Context, string, string, TrialUpdate, time.Time) (TrialBooking, error)
	RescheduleTrial(context.Context, string, string, TrialReschedule, time.Time) (TrialBooking, error)
	FulfillMakeUp(context.Context, string, string, MakeUpFulfillment, time.Time) (MakeUp, error)
	UpdateProfile(context.Context, string, ProfileInput, time.Time) (Profile, error)
	SavePreferences(context.Context, string, Preferences, time.Time) (Profile, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrInvalid)
	}
	return &Service{repository: repository, now: time.Now}, nil
}
func invalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalid, message) }
func validID(id string) error {
	if !platformid.ValidReference(id) {
		return invalid("a valid record ID is required")
	}
	return nil
}
func validText(value string, min, max int) bool {
	n := len(strings.TrimSpace(value))
	return n >= min && n <= max
}
func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func (s *Service) Workspace(ctx context.Context, actor string) (Workspace, error) {
	return s.repository.Workspace(ctx, actor)
}
func (s *Service) CreateStudent(ctx context.Context, actor string, in StudentInput) (Student, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Country = strings.ToUpper(strings.TrimSpace(in.Country))
	in.GuardianEmail = strings.ToLower(strings.TrimSpace(in.GuardianEmail))
	if !validText(in.Name, 2, 100) || !slices.Contains([]string{"4_7", "8_12", "13_17", "18_plus"}, in.AgeBand) || len(in.Country) != 2 {
		return Student{}, invalid("name, age band and two-letter country are required")
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Student{}, invalid("choose an IANA time zone")
	}
	if in.GuardianEmail != "" && !validEmail(in.GuardianEmail) {
		return Student{}, invalid("enter a valid guardian email")
	}
	return s.repository.CreateStudent(ctx, actor, in, s.now().UTC())
}
func (s *Service) CreateStaff(ctx context.Context, actor string, in StaffInput) (Coach, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validText(in.Name, 2, 100) || !validEmail(in.Email) || !slices.Contains([]string{"coach", "academic_manager", "operations_admin", "finance_admin", "super_admin"}, in.Role) {
		return Coach{}, invalid("name, email and supported staff role are required")
	}
	return s.repository.CreateStaff(ctx, actor, in, s.now().UTC())
}
func (s *Service) CreateBatch(ctx context.Context, actor string, in BatchInput) (Batch, error) {
	if !validText(in.Name, 2, 120) || validID(in.CoachID) != nil || !slices.Contains([]string{"group", "one_to_one"}, in.Format) || !validText(in.Level, 1, 80) {
		return Batch{}, invalid("batch name, coach, format and level are required")
	}
	if in.Capacity == 0 {
		if in.Format == "one_to_one" {
			in.Capacity = 1
		} else {
			in.Capacity = 6
		}
	}
	if in.Capacity < 1 || in.Capacity > 6 || (in.Format == "one_to_one" && in.Capacity != 1) {
		return Batch{}, invalid("individual classes require one seat; groups allow up to six")
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil {
		return Batch{}, invalid("choose an IANA time zone")
	}
	return s.repository.CreateBatch(ctx, actor, in, s.now().UTC())
}
func (s *Service) CreateEnrolment(ctx context.Context, actor string, in EnrolmentInput) (Enrolment, error) {
	if validID(in.StudentID) != nil || validID(in.BatchID) != nil || in.StartsAt.IsZero() || (in.EndsAt != nil && !in.EndsAt.After(in.StartsAt)) {
		return Enrolment{}, invalid("student, batch and valid enrolment dates are required")
	}
	return s.repository.CreateEnrolment(ctx, actor, in, s.now().UTC())
}
func (s *Service) PublishPolicy(ctx context.Context, actor string, in PolicyInput) (Policy, error) {
	if !slices.Contains([]string{"privacy", "child-safety", "terms", "cancellation-refund"}, in.Slug) || !validText(in.Title, 2, 150) || !validText(in.Version, 1, 40) || !validText(in.Content, 20, 50000) {
		return Policy{}, invalid("privacy or child-safety policy, title, version and policy text are required")
	}
	return s.repository.PublishPolicy(ctx, actor, in, s.now().UTC())
}
func (s *Service) SaveSettings(ctx context.Context, actor string, in Settings) (Settings, error) {
	if !validText(in.Name, 2, 120) || !validText(in.LegalName, 2, 200) || !validEmail(in.Email) {
		return Settings{}, invalid("academy name, legal name and contact email are required")
	}
	return s.repository.SaveSettings(ctx, actor, in, s.now().UTC())
}
func (s *Service) AcceptConsent(ctx context.Context, actor, id string, in ConsentInput) error {
	if validID(id) != nil || len(in.PolicyVersionIDs) != 2 {
		return invalid("accept both current policies")
	}
	for _, id := range in.PolicyVersionIDs {
		if err := validID(id); err != nil {
			return err
		}
	}
	return s.repository.AcceptConsent(ctx, actor, id, in, s.now().UTC())
}
func (s *Service) IssueCode(ctx context.Context, actor, id string) (AccessCode, error) {
	if err := validID(id); err != nil {
		return AccessCode{}, err
	}
	return s.repository.IssueCode(ctx, actor, id, s.now().UTC())
}

// OccurrenceStarts preserves wall-clock time across weekly DST transitions.
// Nonexistent and ambiguous times must be corrected explicitly by the user.
func OccurrenceStarts(in ClassInput, now time.Time) ([]time.Time, error) {
	if validID(in.BatchID) != nil || !validText(in.Title, 2, 160) || in.DurationMinutes < 15 || in.DurationMinutes > 240 || in.Occurrences < 1 || in.Occurrences > 52 {
		return nil, invalid("choose a batch, title, 15–240 minute duration and up to 52 weekly occurrences")
	}
	if in.MeetingURL != "" {
		u, err := url.Parse(in.MeetingURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(in.MeetingURL) > 2048 {
			return nil, invalid("meeting link must be an HTTPS URL")
		}
	}
	zone, err := time.LoadLocation(in.TimeZone)
	if err != nil {
		return nil, invalid("choose an IANA time zone")
	}
	local, err := time.Parse("2006-01-02T15:04", in.LocalStart)
	if err != nil {
		return nil, invalid("local start must use YYYY-MM-DDTHH:mm")
	}
	if in.RecurringUntil != "" {
		until, err := time.Parse("2006-01-02", in.RecurringUntil)
		if err != nil || until.Before(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)) {
			return nil, invalid("choose an end date on or after the first class")
		}
		in.Occurrences = int(until.Sub(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC))/(7*24*time.Hour)) + 1
		if in.Occurrences > 52 {
			return nil, invalid("weekly recurrence is limited to 52 occurrences")
		}
	}
	starts := make([]time.Time, 0, in.Occurrences)
	for i := 0; i < in.Occurrences; i++ {
		wall := local.AddDate(0, 0, i*7)
		start := time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), 0, 0, zone)
		if start.Format("2006-01-02T15:04") != wall.Format("2006-01-02T15:04") {
			return nil, invalid("this local time does not exist due to daylight saving; choose another time")
		}
		for _, delta := range []time.Duration{-2 * time.Hour, -time.Hour, -30 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour} {
			if start.Add(delta).In(zone).Format("2006-01-02T15:04") == inWall(wall) {
				return nil, invalid("this local time is ambiguous due to daylight saving; choose another time")
			}
		}
		if !start.After(now) || start.After(now.AddDate(1, 0, 0)) {
			return nil, invalid("classes must start in the next twelve months")
		}
		starts = append(starts, start.UTC())
	}
	return starts, nil
}
func inWall(t time.Time) string { return t.Format("2006-01-02T15:04") }
func (s *Service) Schedule(ctx context.Context, actor string, in ClassInput, preview bool) (ClassResult, error) {
	if in.Occurrences == 0 {
		in.Occurrences = 1
	}
	now := s.now().UTC()
	starts, err := OccurrenceStarts(in, now)
	if err != nil {
		return ClassResult{}, err
	}
	if !preview && !validText(in.IdempotencyKey, 1, 200) {
		return ClassResult{}, invalid("idempotency key is required")
	}
	return s.repository.Schedule(ctx, actor, in, starts, now, preview)
}
func (s *Service) UpdateClass(ctx context.Context, actor, id string, in ClassInput) (Class, error) {
	if validID(id) != nil || in.Occurrences > 1 || !slices.Contains([]string{"", "one", "future"}, in.Scope) {
		return Class{}, invalid("choose one occurrence or this and future occurrences")
	}
	in.RecurringUntil = ""
	in.Occurrences = 1
	now := s.now().UTC()
	starts, err := OccurrenceStarts(in, now)
	if err != nil {
		return Class{}, err
	}
	return s.repository.UpdateClass(ctx, actor, id, in, starts[0], now)
}
func (s *Service) Complete(ctx context.Context, actor, id string, in CompletionInput) (Class, error) {
	if validID(id) != nil || !validText(in.Notes, 10, 5000) || len(in.Attendance) < 1 || len(in.Attendance) > 6 || !validText(in.IdempotencyKey, 1, 200) {
		return Class{}, invalid("resolve attendance, add lesson notes and supply an idempotency key")
	}
	for id, status := range in.Attendance {
		if validID(id) != nil || !slices.Contains([]string{"on_time", "late", "excused", "unexcused", "technical_failure"}, status) {
			return Class{}, invalid("attendance status is invalid")
		}
	}
	return s.repository.Complete(ctx, actor, id, in, s.now().UTC())
}
func (s *Service) Cancel(ctx context.Context, actor, id string, in CancellationInput, preview bool) (CancellationPreview, error) {
	if validID(id) != nil || (in.StudentID != "" && validID(in.StudentID) != nil) || !validText(in.Reason, 3, 500) || (!preview && !validText(in.IdempotencyKey, 1, 200)) {
		return CancellationPreview{}, invalid("class, reason and idempotency key are required")
	}
	return s.repository.Cancel(ctx, actor, id, in, s.now().UTC(), preview)
}
