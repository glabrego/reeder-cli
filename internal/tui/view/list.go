package view

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	tuitheme "github.com/glabrego/reeder-cli/internal/tui/theme"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var reANSICodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type EntryLineParams struct {
	Entry        feedbin.Entry
	Now          time.Time
	RelativeTime bool
	Compact      bool
	ShowNumbers  bool
	VisiblePos   int
	Active       bool
	Selected     bool
	Width        int
}

func RenderEntryLine(p EntryLineParams, th tuitheme.Theme) string {
	date := p.Entry.PublishedAt.UTC().Format(time.DateOnly)
	if p.RelativeTime {
		date = RelativeTimeLabel(p.Now, p.Entry.PublishedAt)
	}

	cursorMarker := " "
	if p.Active {
		cursorMarker = ">"
	}
	selectedMarker := " "
	if p.Selected {
		selectedMarker = "*"
	}

	prefix := fmt.Sprintf("    %s%s ", cursorMarker, selectedMarker)
	if p.ShowNumbers {
		prefix = fmt.Sprintf("    %s%s%2d. ", cursorMarker, selectedMarker, p.VisiblePos+1)
	}
	dateLabel := "[" + date + "]"
	available := p.Width - visibleLen(prefix) - 1 - visibleLen(dateLabel)
	if available < 1 {
		available = 1
	}

	label := strings.TrimSpace(p.Entry.Title)
	if p.Compact {
		label = CompactEntryLabel(p.Entry)
	}
	label = truncateRunes(label, available)
	styledTitle := th.StyleArticleTitle(p.Entry, label)
	gap := p.Width - visibleLen(prefix) - visibleLen(label) - visibleLen(dateLabel)
	if gap < 1 {
		gap = 1
	}
	return th.RenderActiveLine(p.Active, prefix+styledTitle+strings.Repeat(" ", gap)+dateLabel)
}

func RenderTreeNodeLine(left string, unreadCount, width int, active bool, th tuitheme.Theme) string {
	if unreadCount <= 0 {
		return th.RenderActiveLine(active, left)
	}
	right := th.UnreadCount.Render(fmt.Sprintf("%d", unreadCount))
	available := width - visibleLen(right) - 1
	if available < 1 {
		available = 1
	}
	left = truncateRunes(left, available)
	gap := width - visibleLen(left) - visibleLen(right)
	if gap < 1 {
		gap = 1
	}
	return th.RenderActiveLine(active, left+strings.Repeat(" ", gap)+right)
}

func RenderSectionLine(label string, unreadCount, width int, active bool, nerdIcons bool, th tuitheme.Theme) string {
	icon := "■"
	if label == "Folders" {
		icon = "▦"
	}
	if nerdIcons {
		if label == "Folders" {
			icon = "󰉋"
		} else {
			icon = "󰈙"
		}
	}
	left := fmt.Sprintf("%s %s", icon, label)
	return RenderTreeNodeLine(th.Section.Render(left), unreadCount, width, active, th)
}

func CompactEntryLabel(entry feedbin.Entry) string {
	title := strings.TrimSpace(entry.Title)
	if title == "" {
		title = "(untitled)"
	}

	parts := make([]string, 0, 3)
	if folder := strings.TrimSpace(entry.FeedFolder); folder != "" {
		parts = append(parts, folder)
	}
	feed := strings.TrimSpace(entry.FeedTitle)
	if feed == "" {
		feed = "unknown feed"
	}
	parts = append(parts, feed, title)
	return strings.Join(parts, " | ")
}

func RelativeTimeLabel(now, then time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	if then.IsZero() {
		return "unknown"
	}
	if then.After(now) {
		return "just now"
	}
	d := now.Sub(then)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		n := int(d / time.Minute)
		if n == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", n)
	}
	if d < 24*time.Hour {
		n := int(d / time.Hour)
		if n == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", n)
	}
	n := int(d / (24 * time.Hour))
	if n == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", n)
}

func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return strings.Repeat(".", maxLen)
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}

func visibleLen(s string) int {
	return utf8.RuneCountInString(stripANSIText(s))
}

func stripANSIText(s string) string {
	return reANSICodes.ReplaceAllString(s, "")
}
