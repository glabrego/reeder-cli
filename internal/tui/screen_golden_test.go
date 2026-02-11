package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var updateScreenGolden = flag.Bool("update-tui-screen-golden", false, "update TUI screen golden files")

var ansiScreenStrip = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestScreenGolden_ListAndDetail(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("FEEDBIN_INLINE_IMAGE_PREVIEW", "0")

	now := time.Date(2026, 2, 11, 16, 0, 0, 0, time.UTC)
	entries := []feedbin.Entry{
		{
			ID:          1,
			Title:       "Jonny Ive Designed a Car Interior",
			FeedTitle:   "512 Pixels",
			FeedFolder:  "Design",
			URL:         "https://example.com/1",
			Summary:     "A story about interior design.",
			IsUnread:    true,
			PublishedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:          2,
			Title:       "Top-level Feed Story",
			FeedTitle:   "Engadget",
			URL:         "https://example.com/2",
			Summary:     "Another story.",
			PublishedAt: now.Add(-6 * time.Hour),
		},
	}

	m := NewModel(nil, entries)
	m.nowFn = func() time.Time { return now }
	m.relativeTime = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 26})
	model := updated.(Model)
	listView := ansiScreenStrip.ReplaceAllString(model.View(), "")
	assertScreenGolden(t, "list_screen.golden", listView)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	detailView := ansiScreenStrip.ReplaceAllString(model.View(), "")
	assertScreenGolden(t, "detail_screen.golden", detailView)
}

func assertScreenGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateScreenGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
	}
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	want := strings.TrimRight(string(wantBytes), "\n")
	got = strings.TrimRight(got, "\n")
	if got != want {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
