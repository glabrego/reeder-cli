package tree

import (
	"fmt"
	"testing"
	"time"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func BenchmarkBuildRows_DefaultTree(b *testing.B) {
	entries := benchmarkEntries(1200)
	opts := BuildOptions{
		Compact:           false,
		CollapsedFolders:  map[string]bool{},
		CollapsedFeeds:    map[string]bool{},
		CollapsedSections: map[string]bool{},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BuildRows(entries, opts)
	}
}

func BenchmarkBuildRows_Compact(b *testing.B) {
	entries := benchmarkEntries(1200)
	opts := BuildOptions{
		Compact:           true,
		CollapsedFolders:  map[string]bool{},
		CollapsedFeeds:    map[string]bool{},
		CollapsedSections: map[string]bool{},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BuildRows(entries, opts)
	}
}

func benchmarkEntries(n int) []feedbin.Entry {
	out := make([]feedbin.Entry, 0, n)
	base := time.Date(2026, 2, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		folder := ""
		if i%3 != 0 {
			folder = fmt.Sprintf("Folder %d", i%15)
		}
		out = append(out, feedbin.Entry{
			ID:          int64(i + 1),
			Title:       fmt.Sprintf("Article %04d", i),
			FeedTitle:   fmt.Sprintf("Feed %02d", i%30),
			FeedFolder:  folder,
			PublishedAt: base.Add(-time.Duration(i) * time.Minute),
		})
	}
	return out
}
