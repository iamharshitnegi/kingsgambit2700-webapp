package report

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	reports map[string]Report
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{reports: make(map[string]Report)}
}

var _ Repository = (*MemoryRepository)(nil)

func (r *MemoryRepository) Report(ctx context.Context, reportID string) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	report, exists := r.reports[reportID]
	if !exists {
		return Report{}, ErrNotFound
	}
	return report.clone(), nil
}

func (r *MemoryRepository) Save(
	ctx context.Context,
	report Report,
	expectedRevision uint64,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := report.validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, exists := r.reports[report.ID()]
	if expectedRevision == 0 {
		if exists || report.Revision() != 1 {
			return ErrConflict
		}
	} else {
		if !exists || current.Revision() != expectedRevision {
			return ErrConflict
		}
		if report.Revision() != expectedRevision+1 {
			return ErrConflict
		}
		if publishedVersionsChanged(current, report) {
			return ErrPublishedImmutable
		}
	}

	r.reports[report.ID()] = report.clone()
	return nil
}

func publishedVersionsChanged(current, updated Report) bool {
	currentVersions := current.Versions()
	updatedVersions := updated.Versions()
	for _, version := range currentVersions {
		if version.Status != VersionStatusPublished {
			continue
		}
		found := false
		for _, candidate := range updatedVersions {
			if candidate.ID != version.ID {
				continue
			}
			found = true
			if !sameVersion(version, candidate) {
				return true
			}
			break
		}
		if !found {
			return true
		}
	}
	return false
}

func sameVersion(left, right Version) bool {
	if left.ID != right.ID ||
		left.Number != right.Number ||
		left.Status != right.Status ||
		left.Content != right.Content ||
		left.RubricVersionID != right.RubricVersionID ||
		left.SourceVersionID != right.SourceVersionID ||
		left.CreatedBy != right.CreatedBy ||
		!left.CreatedAt.Equal(right.CreatedAt) ||
		left.UpdatedBy != right.UpdatedBy ||
		!left.UpdatedAt.Equal(right.UpdatedAt) ||
		left.PublishedBy != right.PublishedBy {
		return false
	}
	if left.PublishedAt == nil || right.PublishedAt == nil {
		return left.PublishedAt == nil && right.PublishedAt == nil
	}
	return left.PublishedAt.Equal(*right.PublishedAt)
}
