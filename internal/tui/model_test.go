package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type fakeRefresher struct {
	entries []feedbin.Entry
	err     error
}

func (f fakeRefresher) Refresh(context.Context, int, int) ([]feedbin.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entries, nil
}

func TestModelView_ShowsEntriesWithMetadata(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{{
		ID:          1,
		Title:       "First Entry",
		FeedTitle:   "Feed A",
		IsUnread:    true,
		IsStarred:   true,
		PublishedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}})

	view := m.View()
	if !strings.Contains(view, "First Entry") {
		t.Fatalf("expected entry title in view, got: %s", view)
	}
	if !strings.Contains(view, "Feed A") {
		t.Fatalf("expected feed title in view, got: %s", view)
	}
	if !strings.Contains(view, "[U] [*]") {
		t.Fatalf("expected state markers in view, got: %s", view)
	}
	if !strings.Contains(view, "> ") {
		t.Fatalf("expected cursor marker in view, got: %s", view)
	}
}

func TestModelUpdate_RefreshError(t *testing.T) {
	m := NewModel(fakeRefresher{err: errors.New("network")}, nil)

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command")
	}

	msg := cmd()
	updatedModel, _ = updatedModel.Update(msg)
	finalModel := updatedModel.(Model)
	if finalModel.err == nil {
		t.Fatal("expected refresh error")
	}
}

func TestModelUpdate_NavigateAndSelect(t *testing.T) {
	m := NewModel(nil, []feedbin.Entry{
		{ID: 1, Title: "First", PublishedAt: time.Now().UTC()},
		{ID: 2, Title: "Second", PublishedAt: time.Now().UTC()},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)
	if model.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", model.cursor)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.selectedID != 2 {
		t.Fatalf("expected selected id 2, got %d", model.selectedID)
	}
}
