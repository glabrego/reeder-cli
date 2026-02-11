package view

import (
	"strings"
	"testing"
	"time"

	tuitheme "github.com/glabrego/reeder-cli/internal/tui/theme"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestRenderEntryLine_AbsoluteDateWhenRelativeDisabled(t *testing.T) {
	now := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	th := tuitheme.Default()

	line := RenderEntryLine(EntryLineParams{
		Entry: feedbin.Entry{
			ID:          1,
			Title:       "Absolute date rendering",
			PublishedAt: now.Add(-2 * time.Hour),
			IsUnread:    true,
		},
		Now:          now,
		RelativeTime: false,
		Width:        60,
	}, th)
	plain := stripANSI(line)
	if !strings.HasSuffix(plain, "[2026-02-09]") {
		t.Fatalf("expected absolute date suffix at right edge, got %q", plain)
	}
}

func TestCompactEntryLabel(t *testing.T) {
	withFolder := CompactEntryLabel(feedbin.Entry{
		Title:      "Article",
		FeedTitle:  "Feed A",
		FeedFolder: "Formula 1",
	})
	if withFolder != "Formula 1 | Feed A | Article" {
		t.Fatalf("unexpected compact label with folder: %q", withFolder)
	}

	withoutFolder := CompactEntryLabel(feedbin.Entry{
		Title:     "Article",
		FeedTitle: "Feed A",
	})
	if withoutFolder != "Feed A | Article" {
		t.Fatalf("unexpected compact label without folder: %q", withoutFolder)
	}
}

func TestRelativeTimeLabel(t *testing.T) {
	now := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		then time.Time
		want string
	}{
		{then: now.Add(-30 * time.Second), want: "just now"},
		{then: now.Add(-1 * time.Minute), want: "1 minute ago"},
		{then: now.Add(-3 * time.Minute), want: "3 minutes ago"},
		{then: now.Add(-1 * time.Hour), want: "1 hour ago"},
		{then: now.Add(-7 * time.Hour), want: "7 hours ago"},
		{then: now.Add(-1 * 24 * time.Hour), want: "1 day ago"},
		{then: now.Add(-7 * 24 * time.Hour), want: "7 days ago"},
	}
	for _, tc := range cases {
		if got := RelativeTimeLabel(now, tc.then); got != tc.want {
			t.Fatalf("RelativeTimeLabel(%s) = %q, want %q", tc.then.UTC().Format(time.RFC3339), got, tc.want)
		}
	}
}
