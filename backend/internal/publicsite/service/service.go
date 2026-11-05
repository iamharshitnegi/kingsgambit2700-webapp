package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/kingsgambit2700/platform/backend/internal/publicsite/model"
	"github.com/kingsgambit2700/platform/backend/internal/publicsite/repository"
)

type Service struct{ repo repository.Repository }

func New(repo repository.Repository) *Service { return &Service{repo: repo} }

func (s *Service) Organization(ctx context.Context) (model.Organization, error) {
	value, err := s.repo.Organization(ctx)
	if err != nil {
		return model.Organization{}, fmt.Errorf("loading organization: %w", err)
	}
	if value.SocialLinks == nil {
		value.SocialLinks = map[string]string{}
	}
	return value, nil
}

func (s *Service) Policies(ctx context.Context) ([]model.PolicySummary, error) {
	items, err := s.repo.Policies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing policies: %w", err)
	}
	if items == nil {
		items = []model.PolicySummary{}
	}
	return items, nil
}

func (s *Service) Policy(ctx context.Context, slug string) (model.Policy, error) {
	value, err := s.repo.PolicyBySlug(ctx, strings.ToLower(strings.TrimSpace(slug)))
	if err != nil {
		return model.Policy{}, err
	}
	return value, nil
}

func (s *Service) PolicyVersion(ctx context.Context, slug, version string) (model.Policy, error) {
	return s.repo.PolicyVersion(
		ctx,
		strings.ToLower(strings.TrimSpace(slug)),
		strings.TrimSpace(version),
	)
}

func (s *Service) Programs(ctx context.Context) ([]model.Program, error) {
	items, err := s.repo.Programs(ctx)
	if items == nil {
		items = []model.Program{}
	}
	return items, err
}

func (s *Service) Coaches(ctx context.Context) ([]model.Coach, error) {
	items, err := s.repo.Coaches(ctx)
	if items == nil {
		items = []model.Coach{}
	}
	return items, err
}

func (s *Service) Curriculum(ctx context.Context) (model.Curriculum, error) {
	value, err := s.repo.Curriculum(ctx)
	if value.Tracks == nil {
		value.Tracks = []model.Track{}
	}
	return value, err
}

func (s *Service) Prices(ctx context.Context, currency string) ([]model.Price, error) {
	items, err := s.repo.Prices(ctx, strings.ToUpper(strings.TrimSpace(currency)))
	if items == nil {
		items = []model.Price{}
	}
	return items, err
}
