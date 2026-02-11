package view

import (
	"strings"
	"testing"
	"time"

	article "github.com/glabrego/reeder-cli/internal/render/article"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestCenterLines(t *testing.T) {
	lines := centerLines([]string{"abc"}, 9)
	if len(lines) != 1 {
		t.Fatalf("expected one line, got %d", len(lines))
	}
	if lines[0] != "   abc" {
		t.Fatalf("expected centered line with padding, got %q", lines[0])
	}
}

func TestDetailLines_UsesMarginsAndPreview(t *testing.T) {
	entry := feedbin.Entry{
		Title:       "Entry",
		FeedTitle:   "Feed A",
		PublishedAt: time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC),
	}
	lines := DetailLines(
		entry,
		60,
		4,
		article.DefaultOptions,
		func(s string, _ int) []string { return []string{s} },
		InlineImagePreviewState{
			Enabled: true,
			Err:     "render failed",
		},
	)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "    Feed: Feed A") {
		t.Fatalf("expected detail metadata with margin, got %q", joined)
	}
	if !strings.Contains(joined, "Image preview unavailable: render failed") {
		t.Fatalf("expected preview fallback error line, got %q", joined)
	}
}
