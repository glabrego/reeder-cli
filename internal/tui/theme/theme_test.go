package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestStyleArticleTitle_ByState(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	th := Default()

	unread := th.StyleArticleTitle(feedbin.Entry{IsUnread: true}, "Unread")
	if !strings.Contains(unread, "\x1b[") {
		t.Fatalf("expected styled unread title, got %q", unread)
	}

	starredRead := th.StyleArticleTitle(feedbin.Entry{IsStarred: true}, "Starred")
	if !strings.Contains(starredRead, "\x1b[") {
		t.Fatalf("expected styled starred title, got %q", starredRead)
	}

	read := th.StyleArticleTitle(feedbin.Entry{}, "Read")
	if !strings.Contains(read, "\x1b[") {
		t.Fatalf("expected styled read title, got %q", read)
	}

	unreadStarred := th.StyleArticleTitle(feedbin.Entry{IsUnread: true, IsStarred: true}, "Both")
	if !strings.Contains(unreadStarred, "\x1b[") {
		t.Fatalf("expected styled unread+starred title, got %q", unreadStarred)
	}
}
