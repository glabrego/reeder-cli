package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type FeedbinClient interface {
	ListEntries(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
	ListEntriesByIDs(ctx context.Context, ids []int64) ([]feedbin.Entry, error)
	ListSubscriptions(ctx context.Context) ([]feedbin.Subscription, error)
	ListUnreadEntryIDs(ctx context.Context) ([]int64, error)
	ListStarredEntryIDs(ctx context.Context) ([]int64, error)
	ListTaggings(ctx context.Context) ([]feedbin.Tagging, error)
	ListUpdatedEntryIDsSince(ctx context.Context, since time.Time) ([]int64, error)
	MarkEntriesUnread(ctx context.Context, entryIDs []int64) error
	MarkEntriesRead(ctx context.Context, entryIDs []int64) error
	StarEntries(ctx context.Context, entryIDs []int64) error
	UnstarEntries(ctx context.Context, entryIDs []int64) error
}

type Repository interface {
	SaveSubscriptions(ctx context.Context, subscriptions []feedbin.Subscription) error
	SaveEntries(ctx context.Context, entries []feedbin.Entry) error
	SaveEntryStates(ctx context.Context, unreadIDs, starredIDs []int64) error
	GetSyncCursor(ctx context.Context, key string) (time.Time, error)
	SetSyncCursor(ctx context.Context, key string, value time.Time) error
	GetAppState(ctx context.Context, key string) (string, error)
	SetAppState(ctx context.Context, key, value string) error
	SetEntryUnread(ctx context.Context, entryID int64, unread bool) error
	SetEntryStarred(ctx context.Context, entryID int64, starred bool) error
	ListEntries(ctx context.Context, limit int) ([]feedbin.Entry, error)
	ListEntriesByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error)
}

type UIPreferences struct {
	Compact         bool
	MarkReadOnOpen  bool
	ConfirmOpenRead bool
	RelativeTime    bool
	ShowNumbers     bool
}

type Service struct {
	client          FeedbinClient
	repo            Repository
	lastStateSyncAt time.Time
	syncCursorKey   string
}

const (
	uiPrefCompactKey        = "ui_pref_compact"
	uiPrefMarkReadOnOpenKey = "ui_pref_mark_read_on_open"
	uiPrefConfirmOpenKey    = "ui_pref_confirm_open_read"
	uiPrefRelativeTimeKey   = "ui_pref_relative_time"
	uiPrefShowNumbersKey    = "ui_pref_show_numbers"
	DefaultCacheLimit       = 1000
)

func NewService(client FeedbinClient, repo Repository) *Service {
	return &Service{
		client:        client,
		repo:          repo,
		syncCursorKey: "updated_entries_since",
	}
}

func (s *Service) Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error) {
	entries, _, err := s.syncPage(ctx, page, perPage, true)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Service) LoadMore(ctx context.Context, page, perPage int, filter string, limit int) ([]feedbin.Entry, int, error) {
	_, fetchedCount, err := s.syncPage(ctx, page, perPage, false)
	if err != nil {
		return nil, 0, err
	}

	entries, err := s.ListCachedByFilter(ctx, limit, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("load entries from cache: %w", err)
	}
	return entries, fetchedCount, nil
}

func (s *Service) syncPage(ctx context.Context, page, perPage int, fullStateSync bool) ([]feedbin.Entry, int, error) {
	if s.lastStateSyncAt.IsZero() {
		if cursor, err := s.repo.GetSyncCursor(ctx, s.syncCursorKey); err == nil {
			s.lastStateSyncAt = cursor
		}
	}

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

	if err := s.repo.SaveEntries(ctx, entries); err != nil {
		return nil, 0, fmt.Errorf("save entries to cache: %w", err)
	}

	if fullStateSync || s.lastStateSyncAt.IsZero() {
		if err := s.syncFullState(ctx); err != nil {
			return nil, 0, err
		}
	} else {
		if err := s.syncIncrementalUpdatedEntries(ctx); err != nil {
			return nil, 0, err
		}
		if err := s.syncEntryStates(ctx); err != nil {
			return nil, 0, err
		}
		s.lastStateSyncAt = time.Now().UTC()
		if err := s.repo.SetSyncCursor(ctx, s.syncCursorKey, s.lastStateSyncAt); err != nil {
			return nil, 0, fmt.Errorf("persist incremental sync cursor: %w", err)
		}
	}

	listLimit := perPage
	if fullStateSync && DefaultCacheLimit > listLimit {
		listLimit = DefaultCacheLimit
	}
	cachedEntries, err := s.repo.ListEntries(ctx, listLimit)
	if err != nil {
		return nil, 0, fmt.Errorf("load entries from cache: %w", err)
	}
	return cachedEntries, len(entries), nil
}

func (s *Service) syncFullState(ctx context.Context) error {
	var (
		subscriptions []feedbin.Subscription
		taggings      []feedbin.Tagging
		unreadIDs     []int64
		starredIDs    []int64
		firstErr      error
		mu            sync.Mutex
		wg            sync.WaitGroup
	)

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}

	wg.Add(4)
	go func() {
		defer wg.Done()
		subs, err := s.client.ListSubscriptions(ctx)
		if err != nil {
			setErr(fmt.Errorf("fetch subscriptions from feedbin: %w", err))
			return
		}
		mu.Lock()
		subscriptions = subs
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		t, err := s.client.ListTaggings(ctx)
		if err != nil {
			setErr(fmt.Errorf("fetch taggings from feedbin: %w", err))
			return
		}
		mu.Lock()
		taggings = t
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		ids, err := s.client.ListUnreadEntryIDs(ctx)
		if err != nil {
			setErr(fmt.Errorf("fetch unread entries from feedbin: %w", err))
			return
		}
		mu.Lock()
		unreadIDs = ids
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		ids, err := s.client.ListStarredEntryIDs(ctx)
		if err != nil {
			setErr(fmt.Errorf("fetch starred entries from feedbin: %w", err))
			return
		}
		mu.Lock()
		starredIDs = ids
		mu.Unlock()
	}()

	wg.Wait()
	if firstErr != nil {
		return firstErr
	}

	applyTaggingsToSubscriptions(subscriptions, taggings)

	if err := s.repo.SaveSubscriptions(ctx, subscriptions); err != nil {
		return fmt.Errorf("save subscriptions to cache: %w", err)
	}
	if err := s.hydrateStateEntries(ctx, unreadIDs, starredIDs); err != nil {
		return err
	}
	if err := s.repo.SaveEntryStates(ctx, unreadIDs, starredIDs); err != nil {
		return fmt.Errorf("save entry state to cache: %w", err)
	}

	s.lastStateSyncAt = time.Now().UTC()
	if err := s.repo.SetSyncCursor(ctx, s.syncCursorKey, s.lastStateSyncAt); err != nil {
		return fmt.Errorf("persist full sync cursor: %w", err)
	}
	return nil
}

func (s *Service) hydrateStateEntries(ctx context.Context, unreadIDs, starredIDs []int64) error {
	idSet := make(map[int64]struct{}, len(unreadIDs)+len(starredIDs))
	for _, id := range unreadIDs {
		idSet[id] = struct{}{}
	}
	for _, id := range starredIDs {
		idSet[id] = struct{}{}
	}
	if len(idSet) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	entries, err := s.client.ListEntriesByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("fetch unread/starred entries from feedbin: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	if err := s.repo.SaveEntries(ctx, entries); err != nil {
		return fmt.Errorf("save unread/starred entries to cache: %w", err)
	}
	return nil
}

func applyTaggingsToSubscriptions(subscriptions []feedbin.Subscription, taggings []feedbin.Tagging) {
	feedFolders := make(map[int64]string, len(taggings))
	for _, tagging := range taggings {
		name := strings.TrimSpace(tagging.Name)
		if name == "" {
			continue
		}
		if existing, ok := feedFolders[tagging.FeedID]; !ok || strings.ToLower(name) < strings.ToLower(existing) {
			feedFolders[tagging.FeedID] = name
		}
	}
	for i := range subscriptions {
		subscriptions[i].Folder = feedFolders[subscriptions[i].ID]
	}
}

func (s *Service) syncEntryStates(ctx context.Context) error {
	unreadIDs, err := s.client.ListUnreadEntryIDs(ctx)
	if err != nil {
		return fmt.Errorf("fetch unread entries from feedbin: %w", err)
	}

	starredIDs, err := s.client.ListStarredEntryIDs(ctx)
	if err != nil {
		return fmt.Errorf("fetch starred entries from feedbin: %w", err)
	}

	if err := s.repo.SaveEntryStates(ctx, unreadIDs, starredIDs); err != nil {
		return fmt.Errorf("save entry state to cache: %w", err)
	}
	return nil
}

func (s *Service) syncIncrementalUpdatedEntries(ctx context.Context) error {
	updatedIDs, err := s.client.ListUpdatedEntryIDsSince(ctx, s.lastStateSyncAt)
	if err != nil {
		return fmt.Errorf("fetch updated entries from feedbin: %w", err)
	}
	if len(updatedIDs) == 0 {
		return nil
	}
	updatedEntries, err := s.client.ListEntriesByIDs(ctx, updatedIDs)
	if err != nil {
		return fmt.Errorf("fetch updated entry payloads from feedbin: %w", err)
	}
	if len(updatedEntries) == 0 {
		return nil
	}
	if err := s.repo.SaveEntries(ctx, updatedEntries); err != nil {
		return fmt.Errorf("save updated entries to cache: %w", err)
	}
	return nil
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

func (s *Service) LoadUIPreferences(ctx context.Context) (UIPreferences, error) {
	compact, err := s.loadBoolPreference(ctx, uiPrefCompactKey)
	if err != nil {
		return UIPreferences{}, err
	}
	markReadOnOpen, err := s.loadBoolPreference(ctx, uiPrefMarkReadOnOpenKey)
	if err != nil {
		return UIPreferences{}, err
	}
	confirmOpenRead, err := s.loadBoolPreference(ctx, uiPrefConfirmOpenKey)
	if err != nil {
		return UIPreferences{}, err
	}
	relativeTime := true
	relativeValue, err := s.repo.GetAppState(ctx, uiPrefRelativeTimeKey)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return UIPreferences{}, fmt.Errorf("load preference %q: %w", uiPrefRelativeTimeKey, err)
		}
	} else {
		parsed, parseErr := strconv.ParseBool(relativeValue)
		if parseErr != nil {
			return UIPreferences{}, fmt.Errorf("parse preference %q value %q: %w", uiPrefRelativeTimeKey, relativeValue, parseErr)
		}
		relativeTime = parsed
	}
	showNumbers, err := s.loadBoolPreference(ctx, uiPrefShowNumbersKey)
	if err != nil {
		return UIPreferences{}, err
	}

	return UIPreferences{
		Compact:         compact,
		MarkReadOnOpen:  markReadOnOpen,
		ConfirmOpenRead: confirmOpenRead,
		RelativeTime:    relativeTime,
		ShowNumbers:     showNumbers,
	}, nil
}

func (s *Service) SaveUIPreferences(ctx context.Context, prefs UIPreferences) error {
	if err := s.repo.SetAppState(ctx, uiPrefCompactKey, strconv.FormatBool(prefs.Compact)); err != nil {
		return fmt.Errorf("save compact preference: %w", err)
	}
	if err := s.repo.SetAppState(ctx, uiPrefMarkReadOnOpenKey, strconv.FormatBool(prefs.MarkReadOnOpen)); err != nil {
		return fmt.Errorf("save mark-read-on-open preference: %w", err)
	}
	if err := s.repo.SetAppState(ctx, uiPrefConfirmOpenKey, strconv.FormatBool(prefs.ConfirmOpenRead)); err != nil {
		return fmt.Errorf("save confirm-open-read preference: %w", err)
	}
	if err := s.repo.SetAppState(ctx, uiPrefRelativeTimeKey, strconv.FormatBool(prefs.RelativeTime)); err != nil {
		return fmt.Errorf("save relative-time preference: %w", err)
	}
	if err := s.repo.SetAppState(ctx, uiPrefShowNumbersKey, strconv.FormatBool(prefs.ShowNumbers)); err != nil {
		return fmt.Errorf("save show-numbers preference: %w", err)
	}
	return nil
}

func (s *Service) loadBoolPreference(ctx context.Context, key string) (bool, error) {
	value, err := s.repo.GetAppState(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("load preference %q: %w", key, err)
	}
	parsed, parseErr := strconv.ParseBool(value)
	if parseErr != nil {
		return false, fmt.Errorf("parse preference %q value %q: %w", key, value, parseErr)
	}
	return parsed, nil
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
