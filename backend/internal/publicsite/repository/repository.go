package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/kingsgambit2700/platform/backend/internal/publicsite/model"
)

var ErrNotFound = errors.New("public site repository: not found")

type Repository interface {
	Organization(context.Context) (model.Organization, error)
	Policies(context.Context) ([]model.PolicySummary, error)
	PolicyBySlug(context.Context, string) (model.Policy, error)
	PolicyVersion(context.Context, string, string) (model.Policy, error)
	Programs(context.Context) ([]model.Program, error)
	Coaches(context.Context) ([]model.Coach, error)
	Curriculum(context.Context) (model.Curriculum, error)
	Prices(context.Context, string) ([]model.Price, error)
}

type Seed struct {
	Organization model.Organization
	Policies     []model.Policy
	Programs     []model.Program
	Coaches      []model.Coach
	Curriculum   model.Curriculum
	Prices       []model.Price
}

type Memory struct {
	mu   sync.RWMutex
	seed Seed
}

func NewMemory(seed Seed) *Memory { return &Memory{seed: seed} }

func (m *Memory) Organization(_ context.Context) (model.Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.seed.Organization, nil
}

func (m *Memory) Policies(_ context.Context) ([]model.PolicySummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.PolicySummary, 0, len(m.seed.Policies))
	for _, policy := range m.seed.Policies {
		items = append(items, policy.PolicySummary)
	}
	return items, nil
}

func (m *Memory) PolicyBySlug(_ context.Context, slug string) (model.Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, policy := range m.seed.Policies {
		if policy.Slug == slug {
			return policy, nil
		}
	}
	return model.Policy{}, ErrNotFound
}

func (m *Memory) PolicyVersion(_ context.Context, slug, version string) (model.Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, policy := range m.seed.Policies {
		if policy.Slug == slug && policy.Version == version {
			return policy, nil
		}
	}
	return model.Policy{}, ErrNotFound
}

func (m *Memory) Programs(_ context.Context) ([]model.Program, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.Program(nil), m.seed.Programs...), nil
}

func (m *Memory) Coaches(_ context.Context) ([]model.Coach, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.Coach, 0, len(m.seed.Coaches))
	for _, coach := range m.seed.Coaches {
		if coach.VerifiedAt != nil {
			items = append(items, coach)
		}
	}
	return items, nil
}

func (m *Memory) Curriculum(_ context.Context) (model.Curriculum, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.seed.Curriculum, nil
}

func (m *Memory) Prices(_ context.Context, currency string) ([]model.Price, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]model.Price, 0)
	for _, price := range m.seed.Prices {
		if currency == "" || len(currency) == 3 {
			items = append(items, price)
		}
	}
	return items, nil
}
