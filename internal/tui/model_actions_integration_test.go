package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuiactions "github.com/glabrego/reeder-cli/internal/tui/actions"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestModelKeypressFlows_EmitActionsMessages(t *testing.T) {
	now := time.Date(2026, 2, 11, 16, 0, 0, 0, time.UTC)
	entries := []feedbin.Entry{
		{
			ID:          1,
			Title:       "Story One",
			FeedTitle:   "Feed A",
			URL:         "https://example.com/1",
			PublishedAt: now,
			IsUnread:    true,
		},
	}
	svc := fakeRefresher{
		entries:      entries,
		unreadResult: false,
		starResult:   true,
	}

	m := NewModel(svc, entries)
	m.openURLFn = func(string) error { return nil }
	m.copyURLFn = func(string) error { return nil }

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if _, ok := cmd().(tuiactions.RefreshSuccessMsg); !ok {
		t.Fatalf("expected RefreshSuccessMsg, got %T", cmd())
	}
	model := updated.(Model)

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected unread filter command")
	}
	if _, ok := cmd().(tuiactions.FilterLoadSuccessMsg); !ok {
		t.Fatalf("expected FilterLoadSuccessMsg, got %T", cmd())
	}
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	model = updated.(Model)
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected search command")
	}
	if _, ok := cmd().(tuiactions.SearchLoadSuccessMsg); !ok {
		t.Fatalf("expected SearchLoadSuccessMsg, got %T", cmd())
	}
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !model.inDetail {
		t.Fatal("expected detail mode after enter")
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected open command in detail mode")
	}
	if _, ok := cmd().(tuiactions.OpenURLSuccessMsg); !ok {
		t.Fatalf("expected OpenURLSuccessMsg, got %T", cmd())
	}
	_ = updated
}
