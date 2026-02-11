package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuiactions "github.com/glabrego/reeder-cli/internal/tui/actions"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestModelUpdate_HandlesAllActionMessageTypes(t *testing.T) {
	now := time.Date(2026, 2, 11, 16, 0, 0, 0, time.UTC)
	baseEntries := []feedbin.Entry{
		{
			ID:          1,
			Title:       "Base",
			FeedTitle:   "Feed",
			URL:         "https://example.com/1",
			IsUnread:    true,
			PublishedAt: now,
		},
	}
	m := NewModel(fakeRefresher{entries: baseEntries}, baseEntries)
	m.nowFn = func() time.Time { return now }

	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{
			name: "refresh success",
			msg: tuiactions.RefreshSuccessMsg{
				Entries:  baseEntries,
				Duration: 120 * time.Millisecond,
				Source:   "manual",
			},
		},
		{
			name: "refresh error",
			msg: tuiactions.RefreshErrorMsg{
				Err:      assertErr("refresh failed"),
				Duration: 120 * time.Millisecond,
				Source:   "manual",
			},
		},
		{
			name: "filter load success",
			msg: tuiactions.FilterLoadSuccessMsg{
				Filter:  "unread",
				Entries: baseEntries,
			},
		},
		{
			name: "filter load error",
			msg:  tuiactions.FilterLoadErrorMsg{Err: assertErr("filter failed")},
		},
		{
			name: "search load success",
			msg: tuiactions.SearchLoadSuccessMsg{
				Filter:  "all",
				Query:   "base",
				Entries: baseEntries,
			},
		},
		{
			name: "search load error",
			msg:  tuiactions.SearchLoadErrorMsg{Err: assertErr("search failed")},
		},
		{
			name: "load more success",
			msg: tuiactions.LoadMoreSuccessMsg{
				Page:         2,
				FetchedCount: 1,
				Entries:      append([]feedbin.Entry{}, baseEntries...),
			},
		},
		{
			name: "load more error",
			msg:  tuiactions.LoadMoreErrorMsg{Err: assertErr("load more failed")},
		},
		{
			name: "toggle unread success",
			msg: tuiactions.ToggleUnreadSuccessMsg{
				EntryID:    1,
				NextUnread: false,
				Status:     "Marked as read",
			},
		},
		{
			name: "toggle starred success",
			msg: tuiactions.ToggleStarredSuccessMsg{
				EntryID:     1,
				NextStarred: true,
				Status:      "Starred entry",
			},
		},
		{
			name: "toggle action error",
			msg:  tuiactions.ToggleActionErrorMsg{Err: assertErr("toggle failed")},
		},
		{
			name: "open url success",
			msg: tuiactions.OpenURLSuccessMsg{
				Status:       "Opened URL in browser",
				EntryID:      1,
				UnreadBefore: true,
				Opened:       true,
			},
		},
		{
			name: "open url error",
			msg:  tuiactions.OpenURLErrorMsg{Err: assertErr("open failed")},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			updated, _ := m.Update(tc.msg)
			next, ok := updated.(Model)
			if !ok {
				t.Fatalf("expected Model after update, got %T", updated)
			}
			m = next
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func assertErr(s string) error { return errString(s) }
