package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

type Theme struct {
	Title       lipgloss.Style
	ModePill    lipgloss.Style
	Section     lipgloss.Style
	UnreadCount lipgloss.Style
	ActiveLine  lipgloss.Style
	MetaLabel   lipgloss.Style
	MetaValue   lipgloss.Style
	StateIdle   lipgloss.Style
	StateWarn   lipgloss.Style
	StateLoad   lipgloss.Style

	TitleUnread  lipgloss.Style
	TitleStarred lipgloss.Style
	TitleRead    lipgloss.Style
	TitleBoth    lipgloss.Style
}

func Default() Theme {
	cpRosewater := lipgloss.Color("#f5e0dc")
	cpMauve := lipgloss.Color("#cba6f7")
	cpRed := lipgloss.Color("#f38ba8")
	cpPeach := lipgloss.Color("#fab387")
	cpYellow := lipgloss.Color("#f9e2af")
	cpGreen := lipgloss.Color("#a6e3a1")
	cpTeal := lipgloss.Color("#94e2d5")
	cpLavender := lipgloss.Color("#b4befe")
	cpText := lipgloss.Color("#cdd6f4")
	cpSubtext0 := lipgloss.Color("#a6adc8")
	cpSubtext1 := lipgloss.Color("#bac2de")
	cpOverlay1 := lipgloss.Color("#7f849c")
	cpSurface0 := lipgloss.Color("#313244")

	return Theme{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(cpMauve),
		ModePill:    lipgloss.NewStyle().Foreground(cpLavender).Background(cpSurface0).Padding(0, 1),
		Section:     lipgloss.NewStyle().Bold(true).Foreground(cpTeal),
		UnreadCount: lipgloss.NewStyle().Foreground(cpYellow).Bold(true),
		ActiveLine:  lipgloss.NewStyle().Background(cpSurface0).Foreground(cpText),
		MetaLabel:   lipgloss.NewStyle().Foreground(cpOverlay1),
		MetaValue:   lipgloss.NewStyle().Foreground(cpSubtext1),
		StateIdle:   lipgloss.NewStyle().Foreground(cpGreen),
		StateWarn:   lipgloss.NewStyle().Foreground(cpRed),
		StateLoad:   lipgloss.NewStyle().Foreground(cpPeach),
		TitleUnread: lipgloss.NewStyle().Bold(true).Foreground(cpText),
		TitleStarred: lipgloss.NewStyle().
			Italic(true).
			Foreground(cpLavender),
		TitleRead: lipgloss.NewStyle().Foreground(cpSubtext0),
		TitleBoth: lipgloss.NewStyle().Bold(true).Italic(true).Foreground(cpRosewater),
	}
}

func (t Theme) StyleArticleTitle(entry feedbin.Entry, title string) string {
	if title == "" {
		return title
	}
	switch {
	case entry.IsUnread && entry.IsStarred:
		return t.TitleBoth.Render(title)
	case entry.IsUnread:
		return t.TitleUnread.Render(title)
	case entry.IsStarred:
		return t.TitleStarred.Render(title)
	default:
		return t.TitleRead.Render(title)
	}
}

func (t Theme) RenderActiveLine(active bool, line string) string {
	if !active {
		return line
	}
	return t.ActiveLine.Render(line)
}
