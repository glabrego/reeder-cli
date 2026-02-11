package actions

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

type Service interface {
	Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
	ListCachedByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error)
	SearchCached(ctx context.Context, limit int, filter, query string) ([]feedbin.Entry, error)
	LoadMore(ctx context.Context, page, perPage int, filter string, limit int) ([]feedbin.Entry, int, error)
	ToggleUnread(ctx context.Context, entryID int64, currentUnread bool) (bool, error)
	ToggleStarred(ctx context.Context, entryID int64, currentStarred bool) (bool, error)
}

type RefreshSuccessMsg struct {
	Entries  []feedbin.Entry
	Duration time.Duration
	Source   string
}

type RefreshErrorMsg struct {
	Err      error
	Duration time.Duration
	Source   string
}

type FilterLoadSuccessMsg struct {
	Filter  string
	Entries []feedbin.Entry
}

type FilterLoadErrorMsg struct {
	Err error
}

type SearchLoadSuccessMsg struct {
	Filter  string
	Query   string
	Entries []feedbin.Entry
}

type SearchLoadErrorMsg struct {
	Err error
}

type LoadMoreSuccessMsg struct {
	Page         int
	FetchedCount int
	Entries      []feedbin.Entry
}

type LoadMoreErrorMsg struct {
	Err error
}

type ToggleUnreadSuccessMsg struct {
	EntryID    int64
	NextUnread bool
	Status     string
}

type ToggleStarredSuccessMsg struct {
	EntryID     int64
	NextStarred bool
	Status      string
}

type ToggleActionErrorMsg struct {
	Err error
}

type OpenURLSuccessMsg struct {
	Status       string
	EntryID      int64
	UnreadBefore bool
	Opened       bool
}

type OpenURLErrorMsg struct {
	Err error
}

func RefreshCmd(service Service, perPage int, source string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		start := time.Now()

		entries, err := service.Refresh(ctx, 1, perPage)
		if err != nil {
			return RefreshErrorMsg{Err: err, Duration: time.Since(start), Source: source}
		}
		return RefreshSuccessMsg{Entries: entries, Duration: time.Since(start), Source: source}
	}
}

func LoadFilterCmd(service Service, filter string, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := service.ListCachedByFilter(ctx, limit, filter)
		if err != nil {
			return FilterLoadErrorMsg{Err: err}
		}
		return FilterLoadSuccessMsg{Filter: filter, Entries: entries}
	}
}

func LoadSearchCmd(service Service, filter, query string, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := service.SearchCached(ctx, limit, filter, query)
		if err != nil {
			return SearchLoadErrorMsg{Err: err}
		}
		return SearchLoadSuccessMsg{Filter: filter, Query: query, Entries: entries}
	}
}

func LoadMoreCmd(service Service, page, perPage int, filter string, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		entries, fetchedCount, err := service.LoadMore(ctx, page, perPage, filter, limit)
		if err != nil {
			return LoadMoreErrorMsg{Err: err}
		}
		return LoadMoreSuccessMsg{Page: page, FetchedCount: fetchedCount, Entries: entries}
	}
}

func ToggleUnreadCmd(service Service, entryID int64, currentUnread bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nextUnread, err := service.ToggleUnread(ctx, entryID, currentUnread)
		if err != nil {
			return ToggleActionErrorMsg{Err: err}
		}

		status := "Marked as read"
		if nextUnread {
			status = "Marked as unread"
		}
		return ToggleUnreadSuccessMsg{EntryID: entryID, NextUnread: nextUnread, Status: status}
	}
}

func ToggleStarredCmd(service Service, entryID int64, currentStarred bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nextStarred, err := service.ToggleStarred(ctx, entryID, currentStarred)
		if err != nil {
			return ToggleActionErrorMsg{Err: err}
		}

		status := "Unstarred entry"
		if nextStarred {
			status = "Starred entry"
		}
		return ToggleStarredSuccessMsg{EntryID: entryID, NextStarred: nextStarred, Status: status}
	}
}

func OpenURLCmd(entryID int64, unreadBefore bool, url string, openFn, copyFn func(string) error) tea.Cmd {
	return func() tea.Msg {
		if openFn != nil {
			if err := openFn(url); err == nil {
				return OpenURLSuccessMsg{Status: "Opened URL in browser", EntryID: entryID, UnreadBefore: unreadBefore, Opened: true}
			}
		}
		if copyFn != nil {
			if err := copyFn(url); err == nil {
				return OpenURLSuccessMsg{Status: "Could not open browser, URL copied to clipboard", EntryID: entryID, UnreadBefore: unreadBefore, Opened: false}
			}
		}
		return OpenURLErrorMsg{Err: fmt.Errorf("could not open URL or copy to clipboard")}
	}
}

func CopyURLCmd(url string, copyFn func(string) error) tea.Cmd {
	return func() tea.Msg {
		if copyFn != nil {
			if err := copyFn(url); err == nil {
				return OpenURLSuccessMsg{Status: "URL copied to clipboard"}
			}
		}
		return OpenURLErrorMsg{Err: fmt.Errorf("could not copy URL to clipboard")}
	}
}
