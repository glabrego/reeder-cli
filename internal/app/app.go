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
	MarkEntriesUnread(ctx context.Context, entryIDs []int64) error
	MarkEntriesRead(ctx context.Context, entryIDs []int64) error
	StarEntries(ctx context.Context, entryIDs []int64) error
	UnstarEntries(ctx context.Context, entryIDs []int64) error
}

type Repository interface {
	SaveSubscriptions(ctx context.Context, subscriptions []feedbin.Subscription) error
	SaveEntries(ctx context.Context, entries []feedbin.Entry) error
	SaveEntryStates(ctx context.Context, unreadIDs, starredIDs []int64) error
	SetEntryUnread(ctx context.Context, entryID int64, unread bool) error
	SetEntryStarred(ctx context.Context, entryID int64, starred bool) error
	ListEntries(ctx context.Context, limit int) ([]feedbin.Entry, error)
	ListEntriesByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error)
}

type Service struct {
	client FeedbinClient
	repo   Repository
}

func NewService(client FeedbinClient, repo Repository) *Service {
	return &Service{client: client, repo: repo}
}

func (s *Service) Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error) {
	entries, _, err := s.syncPage(ctx, page, perPage)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Service) LoadMore(ctx context.Context, page, perPage int, filter string, limit int) ([]feedbin.Entry, int, error) {
	_, fetchedCount, err := s.syncPage(ctx, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	entries, err := s.ListCachedByFilter(ctx, limit, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("load entries from cache: %w", err)
	}
	return entries, fetchedCount, nil
}

func (s *Service) syncPage(ctx context.Context, page, perPage int) ([]feedbin.Entry, int, error) {
	entries, err := s.client.ListEntries(ctx, page, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch entries from feedbin: %w", err)
	}

	if len(entries) == 0 {
		cachedEntries, err := s.repo.ListEntries(ctx, perPage)
		if err != nil {
			return nil, 0, fmt.Errorf("load entries from cache: %w", err)
		}
		return cachedEntries, 0, nil
	}

	subscriptions, err := s.client.ListSubscriptions(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch subscriptions from feedbin: %w", err)
	}
	if err := s.repo.SaveSubscriptions(ctx, subscriptions); err != nil {
		return nil, 0, fmt.Errorf("save subscriptions to cache: %w", err)
	}

	unreadIDs, err := s.client.ListUnreadEntryIDs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch unread entries from feedbin: %w", err)
	}

	starredIDs, err := s.client.ListStarredEntryIDs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch starred entries from feedbin: %w", err)
	}

	enrichEntries(entries, subscriptions, unreadIDs, starredIDs)

	if err := s.repo.SaveEntries(ctx, entries); err != nil {
		return nil, 0, fmt.Errorf("save entries to cache: %w", err)
	}

	if err := s.repo.SaveEntryStates(ctx, unreadIDs, starredIDs); err != nil {
		return nil, 0, fmt.Errorf("save entry state to cache: %w", err)
	}

	cachedEntries, err := s.repo.ListEntries(ctx, perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("load entries from cache: %w", err)
	}
	return cachedEntries, len(entries), nil
}

func (s *Service) ListCached(ctx context.Context, limit int) ([]feedbin.Entry, error) {
	return s.ListCachedByFilter(ctx, limit, "all")
}

func (s *Service) ListCachedByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error) {
	var (
		entries []feedbin.Entry
		err     error
	)
	if filter == "all" {
		entries, err = s.repo.ListEntries(ctx, limit)
	} else {
		entries, err = s.repo.ListEntriesByFilter(ctx, limit, filter)
	}
	if err != nil {
		return nil, fmt.Errorf("load entries from cache: %w", err)
	}
	return entries, nil
}

func (s *Service) ToggleUnread(ctx context.Context, entryID int64, currentUnread bool) (bool, error) {
	nextUnread := !currentUnread
	if nextUnread {
		if err := s.client.MarkEntriesUnread(ctx, []int64{entryID}); err != nil {
			return currentUnread, fmt.Errorf("mark unread in feedbin: %w", err)
		}
	} else {
		if err := s.client.MarkEntriesRead(ctx, []int64{entryID}); err != nil {
			return currentUnread, fmt.Errorf("mark read in feedbin: %w", err)
		}
	}

	if err := s.repo.SetEntryUnread(ctx, entryID, nextUnread); err != nil {
		return currentUnread, fmt.Errorf("save unread state in cache: %w", err)
	}

	return nextUnread, nil
}

func (s *Service) ToggleStarred(ctx context.Context, entryID int64, currentStarred bool) (bool, error) {
	nextStarred := !currentStarred
	if nextStarred {
		if err := s.client.StarEntries(ctx, []int64{entryID}); err != nil {
			return currentStarred, fmt.Errorf("star entry in feedbin: %w", err)
		}
	} else {
		if err := s.client.UnstarEntries(ctx, []int64{entryID}); err != nil {
			return currentStarred, fmt.Errorf("unstar entry in feedbin: %w", err)
		}
	}

	if err := s.repo.SetEntryStarred(ctx, entryID, nextStarred); err != nil {
		return currentStarred, fmt.Errorf("save starred state in cache: %w", err)
	}

	return nextStarred, nil
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
