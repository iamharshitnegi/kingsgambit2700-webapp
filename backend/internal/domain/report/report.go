package report

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidReport      = errors.New("report: invalid report")
	ErrDraftNotFound      = errors.New("report: draft not found")
	ErrVersionNotFound    = errors.New("report: version not found")
	ErrPublishedImmutable = errors.New("report: published version is immutable")
	ErrDraftExists        = errors.New("report: draft already exists")
)

type Period struct {
	Start time.Time
	End   time.Time
}

func NewMonthlyPeriod(year int, month time.Month) (Period, error) {
	if year < 1 || month < time.January || month > time.December {
		return Period{}, fmt.Errorf("%w: year or month is invalid", ErrInvalidReport)
	}
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return Period{Start: start, End: start.AddDate(0, 1, 0)}, nil
}

func (p Period) ValidateMonthly() error {
	if p.Start.IsZero() || p.End.IsZero() || !p.Start.Before(p.End) {
		return fmt.Errorf("%w: period bounds are invalid", ErrInvalidReport)
	}
	want, err := NewMonthlyPeriod(p.Start.Year(), p.Start.Month())
	if err != nil {
		return err
	}
	if !p.Start.Equal(want.Start) || !p.End.Equal(want.End) {
		return fmt.Errorf("%w: period must be one utc calendar month", ErrInvalidReport)
	}
	return nil
}

// Section requires either meaningful text or an explicit omission reason. This
// prevents empty generated sections from being mistaken for learner evidence.
type Section struct {
	Text          string
	OmittedReason string
}

func Written(text string) Section {
	return Section{Text: strings.TrimSpace(text)}
}

func Omitted(reason string) Section {
	return Section{OmittedReason: strings.TrimSpace(reason)}
}

func (s Section) Validate() error {
	hasText := strings.TrimSpace(s.Text) != ""
	hasOmission := strings.TrimSpace(s.OmittedReason) != ""
	if hasText == hasOmission {
		return fmt.Errorf("%w: section needs text or one omission reason", ErrInvalidReport)
	}
	return nil
}

type Content struct {
	Learned      Section
	Strengths    Section
	FocusNext    Section
	PracticePlan Section
	CoachComment Section
}

func (c Content) Validate() error {
	sections := []Section{c.Learned, c.Strengths, c.FocusNext, c.PracticePlan, c.CoachComment}
	for index, section := range sections {
		if err := section.Validate(); err != nil {
			return fmt.Errorf("section %d: %w", index+1, err)
		}
	}
	return nil
}

type VersionStatus string

const (
	VersionStatusUnknown   VersionStatus = ""
	VersionStatusDraft     VersionStatus = "draft"
	VersionStatusPublished VersionStatus = "published"
)

type Version struct {
	ID              string
	Number          uint32
	Status          VersionStatus
	Content         Content
	RubricVersionID string
	SourceVersionID string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedBy       string
	UpdatedAt       time.Time
	PublishedBy     string
	PublishedAt     *time.Time
}

func (v Version) validate() error {
	if v.ID == "" || v.Number == 0 || v.RubricVersionID == "" || v.CreatedBy == "" {
		return fmt.Errorf("%w: version identity is required", ErrInvalidReport)
	}
	if v.CreatedAt.IsZero() || v.UpdatedBy == "" || v.UpdatedAt.IsZero() || v.UpdatedAt.Before(v.CreatedAt) {
		return fmt.Errorf("%w: version timestamps are invalid", ErrInvalidReport)
	}
	if err := v.Content.Validate(); err != nil {
		return err
	}

	switch v.Status {
	case VersionStatusDraft:
		if v.PublishedBy != "" || v.PublishedAt != nil {
			return fmt.Errorf("%w: draft contains publish metadata", ErrInvalidReport)
		}
	case VersionStatusPublished:
		if v.PublishedBy == "" || v.PublishedAt == nil || v.PublishedAt.IsZero() {
			return fmt.Errorf("%w: published version needs publish metadata", ErrInvalidReport)
		}
		if v.PublishedAt.Before(v.CreatedAt) {
			return fmt.Errorf("%w: publish time precedes creation", ErrInvalidReport)
		}
	default:
		return fmt.Errorf("%w: version status %q is unsupported", ErrInvalidReport, v.Status)
	}
	return nil
}

type Receipt struct {
	VersionID      string
	ViewerID       string
	FirstViewedAt  time.Time
	AcknowledgedAt *time.Time
}

type Record struct {
	ID        string
	StudentID string
	Period    Period
	Versions  []Version
	Receipts  []Receipt
	Revision  uint64
	CreatedAt time.Time
}

type Report struct {
	id        string
	studentID string
	period    Period
	versions  []Version
	receipts  []Receipt
	revision  uint64
	createdAt time.Time
}

type NewParams struct {
	ID              string
	StudentID       string
	Period          Period
	VersionID       string
	RubricVersionID string
	Content         Content
	ActorID         string
	CreatedAt       time.Time
}

func New(params NewParams) (Report, error) {
	version := Version{
		ID:              params.VersionID,
		Number:          1,
		Status:          VersionStatusDraft,
		Content:         params.Content,
		RubricVersionID: params.RubricVersionID,
		CreatedBy:       params.ActorID,
		CreatedAt:       params.CreatedAt,
		UpdatedBy:       params.ActorID,
		UpdatedAt:       params.CreatedAt,
	}
	report := Report{
		id:        params.ID,
		studentID: params.StudentID,
		period:    params.Period,
		versions:  []Version{version},
		receipts:  []Receipt{},
		revision:  1,
		createdAt: params.CreatedAt,
	}
	if err := report.validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func Restore(record Record) (Report, error) {
	report := Report{
		id:        record.ID,
		studentID: record.StudentID,
		period:    record.Period,
		versions:  cloneVersions(record.Versions),
		receipts:  cloneReceipts(record.Receipts),
		revision:  record.Revision,
		createdAt: record.CreatedAt,
	}
	if err := report.validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func (r Report) ID() string          { return r.id }
func (r Report) StudentID() string   { return r.studentID }
func (r Report) Period() Period      { return r.period }
func (r Report) Revision() uint64    { return r.revision }
func (r Report) Versions() []Version { return cloneVersions(r.versions) }
func (r Report) Receipts() []Receipt { return cloneReceipts(r.receipts) }

func (r Report) Record() Record {
	return Record{
		ID:        r.id,
		StudentID: r.studentID,
		Period:    r.period,
		Versions:  r.Versions(),
		Receipts:  r.Receipts(),
		Revision:  r.revision,
		CreatedAt: r.createdAt,
	}
}

func (r Report) Version(versionID string) (Version, error) {
	for _, version := range r.versions {
		if version.ID == versionID {
			return cloneVersion(version), nil
		}
	}
	return Version{}, ErrVersionNotFound
}

func (r Report) CurrentPublished() (Version, error) {
	for index := len(r.versions) - 1; index >= 0; index-- {
		if r.versions[index].Status == VersionStatusPublished {
			return cloneVersion(r.versions[index]), nil
		}
	}
	return Version{}, ErrVersionNotFound
}

func (r Report) EditDraft(
	versionID string,
	content Content,
	actorID string,
	at time.Time,
) (Report, error) {
	if actorID == "" || at.IsZero() {
		return Report{}, fmt.Errorf("%w: edit actor and time are required", ErrInvalidReport)
	}
	if err := content.Validate(); err != nil {
		return Report{}, err
	}

	updated := r.clone()
	for index := range updated.versions {
		version := &updated.versions[index]
		if version.ID != versionID {
			continue
		}
		if version.Status == VersionStatusPublished {
			return Report{}, ErrPublishedImmutable
		}
		if at.Before(version.CreatedAt) {
			return Report{}, fmt.Errorf("%w: edit time precedes version creation", ErrInvalidReport)
		}
		version.Content = content
		version.UpdatedBy = actorID
		version.UpdatedAt = at
		updated.revision++
		return updated, nil
	}
	return Report{}, ErrDraftNotFound
}

func (r Report) Publish(versionID, actorID string, at time.Time) (Report, error) {
	if actorID == "" || at.IsZero() {
		return Report{}, fmt.Errorf("%w: publisher and time are required", ErrInvalidReport)
	}

	updated := r.clone()
	for index := range updated.versions {
		version := &updated.versions[index]
		if version.ID != versionID {
			continue
		}
		if version.Status == VersionStatusPublished {
			return Report{}, ErrPublishedImmutable
		}
		if at.Before(version.CreatedAt) {
			return Report{}, fmt.Errorf("%w: publish time precedes version creation", ErrInvalidReport)
		}
		version.Status = VersionStatusPublished
		version.UpdatedBy = actorID
		version.UpdatedAt = at
		version.PublishedBy = actorID
		version.PublishedAt = cloneTime(&at)
		updated.revision++
		return updated, nil
	}
	return Report{}, ErrDraftNotFound
}

func (r Report) StartCorrection(
	newVersionID string,
	sourceVersionID string,
	actorID string,
	at time.Time,
) (Report, error) {
	if newVersionID == "" || sourceVersionID == "" || actorID == "" || at.IsZero() {
		return Report{}, fmt.Errorf("%w: correction metadata is required", ErrInvalidReport)
	}
	for _, version := range r.versions {
		if version.Status == VersionStatusDraft {
			return Report{}, ErrDraftExists
		}
		if version.ID == newVersionID {
			return Report{}, fmt.Errorf("%w: version id already exists", ErrInvalidReport)
		}
	}

	source, err := r.Version(sourceVersionID)
	if err != nil {
		return Report{}, err
	}
	if source.Status != VersionStatusPublished {
		return Report{}, fmt.Errorf("%w: correction source must be published", ErrInvalidReport)
	}
	if source.PublishedAt == nil || at.Before(*source.PublishedAt) {
		return Report{}, fmt.Errorf("%w: correction cannot precede source publication", ErrInvalidReport)
	}

	updated := r.clone()
	updated.versions = append(updated.versions, Version{
		ID:              newVersionID,
		Number:          uint32(len(updated.versions) + 1),
		Status:          VersionStatusDraft,
		Content:         source.Content,
		RubricVersionID: source.RubricVersionID,
		SourceVersionID: source.ID,
		CreatedBy:       actorID,
		CreatedAt:       at,
		UpdatedBy:       actorID,
		UpdatedAt:       at,
	})
	updated.revision++
	return updated, nil
}

func (r Report) RecordView(versionID, viewerID string, at time.Time) (Report, bool, error) {
	if viewerID == "" || at.IsZero() {
		return Report{}, false, fmt.Errorf("%w: viewer and time are required", ErrInvalidReport)
	}
	version, err := r.Version(versionID)
	if err != nil {
		return Report{}, false, err
	}
	if version.Status != VersionStatusPublished {
		return Report{}, false, fmt.Errorf("%w: only published versions can be viewed", ErrInvalidReport)
	}
	if version.PublishedAt == nil || at.Before(*version.PublishedAt) {
		return Report{}, false, fmt.Errorf("%w: view cannot precede publication", ErrInvalidReport)
	}
	for _, receipt := range r.receipts {
		if receipt.VersionID == versionID && receipt.ViewerID == viewerID {
			return r.clone(), false, nil
		}
	}

	updated := r.clone()
	updated.receipts = append(updated.receipts, Receipt{
		VersionID:     versionID,
		ViewerID:      viewerID,
		FirstViewedAt: at,
	})
	updated.revision++
	return updated, true, nil
}

func (r Report) Acknowledge(versionID, viewerID string, at time.Time) (Report, bool, error) {
	if viewerID == "" || at.IsZero() {
		return Report{}, false, fmt.Errorf("%w: viewer and time are required", ErrInvalidReport)
	}
	version, err := r.Version(versionID)
	if err != nil {
		return Report{}, false, err
	}
	if version.Status != VersionStatusPublished {
		return Report{}, false, fmt.Errorf("%w: only published versions can be acknowledged", ErrInvalidReport)
	}
	if version.PublishedAt == nil || at.Before(*version.PublishedAt) {
		return Report{}, false, fmt.Errorf("%w: acknowledgement cannot precede publication", ErrInvalidReport)
	}

	updated := r.clone()
	for index := range updated.receipts {
		receipt := &updated.receipts[index]
		if receipt.VersionID != versionID || receipt.ViewerID != viewerID {
			continue
		}
		if receipt.AcknowledgedAt != nil {
			return updated, false, nil
		}
		receipt.AcknowledgedAt = cloneTime(&at)
		updated.revision++
		return updated, true, nil
	}

	updated.receipts = append(updated.receipts, Receipt{
		VersionID:      versionID,
		ViewerID:       viewerID,
		FirstViewedAt:  at,
		AcknowledgedAt: cloneTime(&at),
	})
	updated.revision++
	return updated, true, nil
}

func (r Report) validate() error {
	if r.id == "" || r.studentID == "" || r.createdAt.IsZero() || r.revision == 0 {
		return fmt.Errorf("%w: report identity is required", ErrInvalidReport)
	}
	if err := r.period.ValidateMonthly(); err != nil {
		return err
	}
	if len(r.versions) == 0 {
		return fmt.Errorf("%w: at least one version is required", ErrInvalidReport)
	}

	versionIDs := make(map[string]struct{}, len(r.versions))
	draftCount := 0
	for index, version := range r.versions {
		if err := version.validate(); err != nil {
			return err
		}
		if version.Number != uint32(index+1) {
			return fmt.Errorf("%w: version numbers must be sequential", ErrInvalidReport)
		}
		if version.CreatedAt.Before(r.createdAt) {
			return fmt.Errorf("%w: version predates report", ErrInvalidReport)
		}
		if index > 0 && version.CreatedAt.Before(r.versions[index-1].CreatedAt) {
			return fmt.Errorf("%w: versions are not chronological", ErrInvalidReport)
		}
		if _, exists := versionIDs[version.ID]; exists {
			return fmt.Errorf("%w: duplicate version id", ErrInvalidReport)
		}
		versionIDs[version.ID] = struct{}{}
		if version.Status == VersionStatusDraft {
			draftCount++
		}
	}
	if draftCount > 1 {
		return fmt.Errorf("%w: more than one draft", ErrInvalidReport)
	}

	type receiptKey struct {
		versionID string
		viewerID  string
	}
	receiptKeys := make(map[receiptKey]struct{}, len(r.receipts))
	for _, receipt := range r.receipts {
		if receipt.VersionID == "" || receipt.ViewerID == "" || receipt.FirstViewedAt.IsZero() {
			return fmt.Errorf("%w: receipt is invalid", ErrInvalidReport)
		}
		key := receiptKey{versionID: receipt.VersionID, viewerID: receipt.ViewerID}
		if _, exists := receiptKeys[key]; exists {
			return fmt.Errorf("%w: duplicate receipt", ErrInvalidReport)
		}
		receiptKeys[key] = struct{}{}
		version, err := r.Version(receipt.VersionID)
		if err != nil || version.Status != VersionStatusPublished {
			return fmt.Errorf("%w: receipt version is not published", ErrInvalidReport)
		}
		if version.PublishedAt == nil || receipt.FirstViewedAt.Before(*version.PublishedAt) {
			return fmt.Errorf("%w: receipt predates publication", ErrInvalidReport)
		}
		if receipt.AcknowledgedAt != nil && receipt.AcknowledgedAt.Before(receipt.FirstViewedAt) {
			return fmt.Errorf("%w: acknowledgement precedes first view", ErrInvalidReport)
		}
	}
	return nil
}

func (r Report) clone() Report {
	cloned := r
	cloned.versions = cloneVersions(r.versions)
	cloned.receipts = cloneReceipts(r.receipts)
	return cloned
}

func cloneVersion(version Version) Version {
	cloned := version
	cloned.PublishedAt = cloneTime(version.PublishedAt)
	return cloned
}

func cloneVersions(versions []Version) []Version {
	cloned := make([]Version, 0, len(versions))
	for _, version := range versions {
		cloned = append(cloned, cloneVersion(version))
	}
	return cloned
}

func cloneReceipts(receipts []Receipt) []Receipt {
	cloned := make([]Receipt, 0, len(receipts))
	for _, receipt := range receipts {
		copy := receipt
		copy.AcknowledgedAt = cloneTime(receipt.AcknowledgedAt)
		cloned = append(cloned, copy)
	}
	return cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
