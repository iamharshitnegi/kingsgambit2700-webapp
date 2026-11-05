package academy

import (
	"errors"
	"time"
)

var (
	ErrDenied   = errors.New("academy: permission denied")
	ErrNotFound = errors.New("academy: record not found")
	ErrConflict = errors.New("academy: record changed or conflicts with existing records")
	ErrInvalid  = errors.New("academy: invalid input")
	ErrConsent  = errors.New("academy: published policies and guardian consent are required")
	ErrCredits  = errors.New("academy: insufficient available class credits")
)

type Capabilities struct {
	ManageAcademy    bool `json:"manageAcademy"`
	ManageScheduling bool `json:"manageScheduling"`
	ManageTeaching   bool `json:"manageTeaching"`
	ManageStaff      bool `json:"manageStaff"`
	ManagePolicies   bool `json:"managePolicies"`
}
type Student struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	FamilyID string `json:"familyId"`
	Name     string `json:"name"`
	AgeBand  string `json:"ageBand"`
	TimeZone string `json:"timeZone"`
	Status   string `json:"status"`
	Consent  bool   `json:"consent"`
	Country  string `json:"country"`
}
type Coach struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Batch struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CoachID    string   `json:"coachId"`
	Format     string   `json:"format"`
	Level      string   `json:"level"`
	TimeZone   string   `json:"timeZone"`
	Capacity   int      `json:"capacity"`
	StudentIDs []string `json:"studentIds"`
}
type Enrolment struct {
	ID        string     `json:"id"`
	StudentID string     `json:"studentId"`
	BatchID   string     `json:"batchId"`
	AccountID string     `json:"accountId"`
	Status    string     `json:"status"`
	Available int64      `json:"available"`
	Reserved  int64      `json:"reserved"`
	Consumed  int64      `json:"consumed"`
	StartsAt  time.Time  `json:"startsAt"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
}
type Class struct {
	ID         string            `json:"id"`
	BatchID    string            `json:"batchId"`
	Title      string            `json:"title"`
	CoachID    string            `json:"coachId"`
	Coach      string            `json:"coach"`
	Format     string            `json:"format"`
	StudentIDs []string          `json:"studentIds"`
	StartsAt   time.Time         `json:"startsAt"`
	EndsAt     time.Time         `json:"endsAt"`
	TimeZone   string            `json:"timeZone"`
	Status     string            `json:"status"`
	Revision   uint64            `json:"revision"`
	Notes      string            `json:"notes"`
	MeetingURL string            `json:"meetingUrl"`
	Attendance map[string]string `json:"attendance"`
}
type Policy struct {
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Content string `json:"content"`
}
type Settings struct {
	Name      string `json:"name"`
	LegalName string `json:"legalName"`
	Email     string `json:"email"`
}
type Workspace struct {
	Pagination   Pagination     `json:"pagination"`
	MakeUps      []MakeUp       `json:"makeUps"`
	TrialSlots   []TrialSlot    `json:"trialSlots"`
	Capabilities Capabilities   `json:"capabilities"`
	Students     []Student      `json:"students"`
	Coaches      []Coach        `json:"coaches"`
	Batches      []Batch        `json:"batches"`
	Enrolments   []Enrolment    `json:"enrolments"`
	Classes      []Class        `json:"classes"`
	Policies     []Policy       `json:"policies"`
	Settings     Settings       `json:"settings"`
	Profile      Profile        `json:"profile"`
	Availability []Availability `json:"availability"`
	Workload     Workload       `json:"workload"`
}
type Pagination struct {
	Limit     int  `json:"limit"`
	HasMore   bool `json:"hasMore"`
	NextLimit int  `json:"nextLimit"`
}
type MakeUp struct {
	ID                 string    `json:"id"`
	ClassID            string    `json:"classId"`
	StudentID          string    `json:"studentId"`
	Reason             string    `json:"reason"`
	Status             string    `json:"status"`
	ReplacementClassID string    `json:"replacementClassId,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}
type MakeUpFulfillment struct {
	ReplacementClassID string `json:"replacementClassId"`
	IdempotencyKey     string `json:"idempotencyKey"`
}
type MonthlyWorkload struct {
	Month                 string `json:"month"`
	CompletedClassMinutes int    `json:"completedClassMinutes"`
	CompletedTrialMinutes int    `json:"completedTrialMinutes"`
	CompletedClasses      int    `json:"completedClasses"`
	CompletedTrials       int    `json:"completedTrials"`
}
type SavedView struct {
	Name    string `json:"name"`
	Section string `json:"section"`
	Query   string `json:"query"`
}
type Profile struct {
	DisplayName string      `json:"displayName"`
	TimeZone    string      `json:"timeZone"`
	SavedViews  []SavedView `json:"savedViews"`
}
type ProfileInput struct {
	DisplayName string `json:"displayName"`
	TimeZone    string `json:"timeZone"`
}
type Preferences struct {
	TimeZone   string      `json:"timeZone"`
	SavedViews []SavedView `json:"savedViews"`
}
type Workload struct {
	Months            []MonthlyWorkload `json:"months"`
	ScheduledClasses  int               `json:"scheduledClasses"`
	PendingCompletion int               `json:"pendingCompletion"`
	CompletedClasses  int               `json:"completedClasses"`
	CompletedMinutes  int               `json:"completedMinutes"`
}
type StudentInput struct {
	Name          string `json:"name"`
	AgeBand       string `json:"ageBand"`
	TimeZone      string `json:"timeZone"`
	Country       string `json:"country"`
	GuardianEmail string `json:"guardianEmail"`
	GuardianName  string `json:"guardianName"`
}
type StaffInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
type BatchInput struct {
	Name     string `json:"name"`
	CoachID  string `json:"coachId"`
	Format   string `json:"format"`
	Level    string `json:"level"`
	TimeZone string `json:"timeZone"`
	Capacity int    `json:"capacity"`
	Status   string `json:"status,omitempty"`
}
type EnrolmentInput struct {
	StudentID string     `json:"studentId"`
	BatchID   string     `json:"batchId"`
	StartsAt  time.Time  `json:"startsAt"`
	EndsAt    *time.Time `json:"endsAt,omitempty"`
}
type PolicyInput struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Content string `json:"content"`
}
type ConsentInput struct {
	PolicyVersionIDs []string `json:"policyVersionIds"`
}
type AccessCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type ClassInput struct {
	BatchID         string `json:"batchId"`
	Title           string `json:"title"`
	LocalStart      string `json:"localStart"`
	TimeZone        string `json:"timeZone"`
	DurationMinutes int    `json:"durationMinutes"`
	Occurrences     int    `json:"occurrences"`
	RecurringUntil  string `json:"recurringUntil,omitempty"`
	MeetingURL      string `json:"meetingUrl,omitempty"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Revision        uint64 `json:"revision"`
	Scope           string `json:"scope,omitempty"`
}

type StudentUpdate struct {
	Name     string `json:"name"`
	AgeBand  string `json:"ageBand"`
	TimeZone string `json:"timeZone"`
	Country  string `json:"country"`
	Status   string `json:"status"`
}
type EnrolmentUpdate struct {
	Status string     `json:"status"`
	EndsAt *time.Time `json:"endsAt,omitempty"`
}
type Availability struct {
	ID       string    `json:"id"`
	CoachID  string    `json:"coachId"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
	TimeZone string    `json:"timeZone"`
	Status   string    `json:"status"`
}
type Lead struct {
	ID           string    `json:"id"`
	GuardianName string    `json:"guardianName"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	AgeBand      string    `json:"ageBand"`
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
	StudentID    string    `json:"studentId,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}
type LeadUpdate struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}
type ConversionInput struct {
	StudentID string `json:"studentId"`
}
type TrialSlot struct {
	ID             string    `json:"id"`
	CoachID        string    `json:"coachId"`
	StartsAt       time.Time `json:"startsAt"`
	EndsAt         time.Time `json:"endsAt"`
	TimeZone       string    `json:"timeZone"`
	Format         string    `json:"format"`
	Capacity       int       `json:"capacity"`
	AgeBands       []string  `json:"ageBands"`
	Status         string    `json:"status"`
	AvailableSeats int       `json:"availableSeats"`
}
type TrialBooking struct {
	ID           string    `json:"id"`
	SlotID       string    `json:"slotId"`
	LeadID       string    `json:"leadId"`
	GuardianName string    `json:"guardianName"`
	StudentName  string    `json:"studentName"`
	Status       string    `json:"status"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	Notes        string    `json:"notes"`
}
type TrialUpdate struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}
type TrialReschedule struct {
	SlotID         string `json:"slotId"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type Operations struct {
	Pagination    Pagination     `json:"pagination"`
	Leads         []Lead         `json:"leads"`
	TrialSlots    []TrialSlot    `json:"trialSlots"`
	TrialBookings []TrialBooking `json:"trialBookings"`
}
type ClassResult struct {
	Classes []Class `json:"classes"`
}
type CompletionInput struct {
	Revision       uint64            `json:"revision"`
	Notes          string            `json:"notes"`
	Attendance     map[string]string `json:"attendance"`
	IdempotencyKey string            `json:"idempotencyKey"`
}
type CancellationInput struct {
	Revision       uint64 `json:"revision"`
	StudentID      string `json:"studentId,omitempty"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type CancellationPreview struct {
	ClassID  string                `json:"classId"`
	Revision uint64                `json:"revision"`
	Outcomes []CancellationOutcome `json:"outcomes"`
}
type CancellationOutcome struct {
	StudentID string `json:"studentId"`
	Action    string `json:"action"`
	MakeUp    bool   `json:"makeUp"`
	Reason    string `json:"reason"`
}
