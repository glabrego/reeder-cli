package app

import (
	"context"
	"fmt"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type FeedbinClient interface {
	ListEntries(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
}

type Repository interface {
	SaveEntries(ctx context.Context, entries []feedbin.Entry) error
	ListEntries(ctx context.Context, limit int) ([]feedbin.Entry, error)
}

type Service struct {
	client FeedbinClient
	repo   Repository
}

func NewService(client FeedbinClient, repo Repository) *Service {
	return &Service{client: client, repo: repo}
}

func (s *Service) Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error) {
	entries, err := s.client.ListEntries(ctx, page, perPage)
	if err != nil {
		return nil, fmt.Errorf("fetch entries from feedbin: %w", err)
	}
	if err := s.repo.SaveEntries(ctx, entries); err != nil {
		return nil, fmt.Errorf("save entries to cache: %w", err)
	}
	return entries, nil
}

func (s *Service) ListCached(ctx context.Context, limit int) ([]feedbin.Entry, error) {
	entries, err := s.repo.ListEntries(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("load entries from cache: %w", err)
	}
	return entries, nil
}
