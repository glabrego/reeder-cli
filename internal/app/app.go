package app

import (
	"context"
	"fmt"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type FeedbinClient interface {
	ListEntries(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
	ListSubscriptions(ctx context.Context) ([]feedbin.Subscription, error)
	ListUnreadEntryIDs(ctx context.Context) ([]int64, error)
	ListStarredEntryIDs(ctx context.Context) ([]int64, error)
}

type Repository interface {
	SaveSubscriptions(ctx context.Context, subscriptions []feedbin.Subscription) error
	SaveEntries(ctx context.Context, entries []feedbin.Entry) error
	SaveEntryStates(ctx context.Context, unreadIDs, starredIDs []int64) error
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

	subscriptions, err := s.client.ListSubscriptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch subscriptions from feedbin: %w", err)
	}
	if err := s.repo.SaveSubscriptions(ctx, subscriptions); err != nil {
		return nil, fmt.Errorf("save subscriptions to cache: %w", err)
	}

	unreadIDs, err := s.client.ListUnreadEntryIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch unread entries from feedbin: %w", err)
	}

	starredIDs, err := s.client.ListStarredEntryIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch starred entries from feedbin: %w", err)
	}

	enrichEntries(entries, subscriptions, unreadIDs, starredIDs)

	if err := s.repo.SaveEntries(ctx, entries); err != nil {
		return nil, fmt.Errorf("save entries to cache: %w", err)
	}

	if err := s.repo.SaveEntryStates(ctx, unreadIDs, starredIDs); err != nil {
		return nil, fmt.Errorf("save entry state to cache: %w", err)
	}

	cachedEntries, err := s.repo.ListEntries(ctx, perPage)
	if err != nil {
		return nil, fmt.Errorf("load entries from cache: %w", err)
	}
	return cachedEntries, nil
}

func (s *Service) ListCached(ctx context.Context, limit int) ([]feedbin.Entry, error) {
	entries, err := s.repo.ListEntries(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("load entries from cache: %w", err)
	}
	return entries, nil
}

func enrichEntries(entries []feedbin.Entry, subscriptions []feedbin.Subscription, unreadIDs, starredIDs []int64) {
	feedTitles := make(map[int64]string, len(subscriptions))
	for _, sub := range subscriptions {
		feedTitles[sub.ID] = sub.Title
	}

	unreadSet := make(map[int64]struct{}, len(unreadIDs))
	for _, id := range unreadIDs {
		unreadSet[id] = struct{}{}
	}

	starredSet := make(map[int64]struct{}, len(starredIDs))
	for _, id := range starredIDs {
		starredSet[id] = struct{}{}
	}

	for i := range entries {
		entries[i].FeedTitle = feedTitles[entries[i].FeedID]
		_, entries[i].IsUnread = unreadSet[entries[i].ID]
		_, entries[i].IsStarred = starredSet[entries[i].ID]
	}
}
