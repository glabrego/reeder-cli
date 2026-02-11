package view

import (
	"strings"
	"time"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

type WrapFunc func(string, int) []string

func DetailMetaLines(entry feedbin.Entry, width int, wrap WrapFunc) []string {
	lines := make([]string, 0, 16)
	lines = append(lines, wrap(entry.Title, width)...)
	lines = append(lines, strings.Repeat("=", max(1, min(width, len(entry.Title)))))
	lines = append(lines, "")

	if entry.FeedTitle != "" {
		lines = append(lines, wrap("Feed: "+entry.FeedTitle, width)...)
	}
	lines = append(lines, "Date: "+entry.PublishedAt.UTC().Format(time.RFC3339))
	if entry.IsUnread {
		lines = append(lines, "Unread: yes")
	} else {
		lines = append(lines, "Unread: no")
	}
	if entry.IsStarred {
		lines = append(lines, "Starred: yes")
	} else {
		lines = append(lines, "Starred: no")
	}

	if entry.Author != "" {
		lines = append(lines, wrap("Author: "+entry.Author, width)...)
	}
	if entry.URL != "" {
		lines = append(lines, wrap("URL: "+entry.URL, width)...)
	}

	return lines
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
