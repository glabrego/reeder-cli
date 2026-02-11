package view

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tuitheme "github.com/glabrego/reeder-cli/internal/tui/theme"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var updateViewGolden = flag.Bool("update-view-golden", false, "update view golden files")

func TestListRendering_Golden(t *testing.T) {
	th := tuitheme.Default()
	now := time.Date(2026, 2, 11, 16, 0, 0, 0, time.UTC)
	entry := feedbin.Entry{
		ID:          42,
		Title:       "Jonny Ive Designed a Car Interior",
		FeedTitle:   "512 Pixels",
		FeedFolder:  "Design",
		IsUnread:    true,
		PublishedAt: now.Add(-3 * time.Hour),
	}

	lines := []string{
		RenderSectionLine("Folders", 6, 78, false, false, th),
		RenderTreeNodeLine("▾ Design", 6, 78, false, th),
		RenderTreeNodeLine("  ▾ 512 Pixels", 6, 78, true, th),
		RenderEntryLine(EntryLineParams{
			Entry:        entry,
			Now:          now,
			RelativeTime: true,
			Compact:      false,
			ShowNumbers:  true,
			VisiblePos:   0,
			Active:       true,
			Selected:     true,
			Width:        78,
		}, th),
		RenderEntryLine(EntryLineParams{
			Entry:        entry,
			Now:          now,
			RelativeTime: false,
			Compact:      true,
			ShowNumbers:  false,
			VisiblePos:   0,
			Active:       false,
			Selected:     false,
			Width:        78,
		}, th),
	}
	got := stripANSI(strings.Join(lines, "\n"))
	assertViewGolden(t, "list_rendering.golden", got)
}

func assertViewGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateViewGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := strings.TrimRight(string(wantBytes), "\n")
	got = strings.TrimRight(got, "\n")
	if got != want {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
