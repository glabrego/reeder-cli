package article

import "github.com/charmbracelet/lipgloss"

var (
	cpMauve    = lipgloss.Color("#cba6f7")
	cpPeach    = lipgloss.Color("#fab387")
	cpYellow   = lipgloss.Color("#f9e2af")
	cpGreen    = lipgloss.Color("#a6e3a1")
	cpTeal     = lipgloss.Color("#94e2d5")
	cpBlue     = lipgloss.Color("#89b4fa")
	cpLavender = lipgloss.Color("#b4befe")
	cpSubtext0 = lipgloss.Color("#a6adc8")
	cpSubtext1 = lipgloss.Color("#bac2de")
	cpOverlay0 = lipgloss.Color("#6c7086")
	cpOverlay1 = lipgloss.Color("#7f849c")
	cpSurface2 = lipgloss.Color("#585b70")

	detailHeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(cpLavender)
	detailHeadingBars  = []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(cpBlue),
		lipgloss.NewStyle().Bold(true).Foreground(cpMauve),
		lipgloss.NewStyle().Bold(true).Foreground(cpTeal),
		lipgloss.NewStyle().Bold(true).Foreground(cpGreen),
		lipgloss.NewStyle().Bold(true).Foreground(cpYellow),
		lipgloss.NewStyle().Bold(true).Foreground(cpPeach),
	}
	detailLinkURL     = lipgloss.NewStyle().Foreground(cpBlue).Faint(true)
	detailQuotePrefix = lipgloss.NewStyle().Foreground(cpOverlay1).Render("│ ")
	detailQuoteText   = lipgloss.NewStyle().Italic(true).Foreground(cpSubtext0)
	detailCitation    = lipgloss.NewStyle().Italic(true).Foreground(cpOverlay0).Faint(true)
	detailCodeStyle   = lipgloss.NewStyle().Foreground(cpPeach)
	detailTableBorder = lipgloss.NewStyle().Foreground(cpSurface2)
	detailTableHeader = lipgloss.NewStyle().Bold(true).Foreground(cpYellow)
	detailImageLabel  = lipgloss.NewStyle().Foreground(cpMauve).Faint(true).Italic(true)
	detailImageText   = lipgloss.NewStyle().Foreground(cpSubtext1).Italic(true)
)
