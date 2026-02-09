package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type Service interface {
	Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
	ListCachedByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error)
	ToggleUnread(ctx context.Context, entryID int64, currentUnread bool) (bool, error)
	ToggleStarred(ctx context.Context, entryID int64, currentStarred bool) (bool, error)
}

type refreshSuccessMsg struct {
	entries []feedbin.Entry
}

type refreshErrorMsg struct {
	err error
}

type filterLoadSuccessMsg struct {
	filter  string
	entries []feedbin.Entry
}

type filterLoadErrorMsg struct {
	err error
}

type Model struct {
	service    Service
	entries    []feedbin.Entry
	cursor     int
	selectedID int64
	filter     string
	inDetail   bool
	detailTop  int
	width      int
	height     int
	loading    bool
	status     string
	err        error
}

func NewModel(service Service, entries []feedbin.Entry) Model {
	return Model{service: service, entries: entries, filter: "all"}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.inDetail {
			switch msg.String() {
			case "esc", "backspace":
				m.inDetail = false
				m.detailTop = 0
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.detailTop > 0 {
					m.detailTop--
				}
				return m, nil
			case "down", "j":
				entry := m.entries[m.cursor]
				lines := buildDetailLines(entry, m.contentWidth())
				maxTop := 0
				if max := len(lines) - m.detailBodyHeight(); max > 0 {
					maxTop = max
				}
				if m.detailTop < maxTop {
					m.detailTop++
				}
				return m, nil
			case "m":
				return m.toggleUnreadCurrent()
			case "s":
				return m.toggleStarredCurrent()
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if len(m.entries) == 0 {
				return m, nil
			}
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if len(m.entries) == 0 {
				return m, nil
			}
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(m.entries) == 0 {
				return m, nil
			}
			m.selectedID = m.entries[m.cursor].ID
			m.inDetail = true
			m.detailTop = 0
			return m, nil
		case "r":
			if m.service == nil {
				return m, nil
			}
			m.loading = true
			m.status = ""
			m.err = nil
			return m, refreshCmd(m.service)
		case "a":
			return m.switchFilter("all")
		case "u":
			return m.switchFilter("unread")
		case "*":
			return m.switchFilter("starred")
		case "m":
			return m.toggleUnreadCurrent()
		case "s":
			return m.toggleStarredCurrent()
		}
	case refreshSuccessMsg:
		m.loading = false
		m.entries = msg.entries
		m.applyCurrentFilter()
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.err = nil
		return m, nil
	case refreshErrorMsg:
		m.loading = false
		m.status = ""
		m.err = msg.err
		return m, nil
	case filterLoadSuccessMsg:
		m.loading = false
		m.err = nil
		m.filter = msg.filter
		m.entries = msg.entries
		m.cursor = 0
		if m.filter == "all" {
			m.status = "Filter: all"
		} else if m.filter == "unread" {
			m.status = "Filter: unread"
		} else {
			m.status = "Filter: starred"
		}
		return m, nil
	case filterLoadErrorMsg:
		m.loading = false
		m.status = ""
		m.err = msg.err
		return m, nil
	case toggleUnreadSuccessMsg:
		m.loading = false
		m.err = nil
		m.status = msg.status
		m.setEntryUnread(msg.entryID, msg.nextUnread)
		m.applyCurrentFilter()
		m.clampCursor()
		return m, nil
	case toggleStarredSuccessMsg:
		m.loading = false
		m.err = nil
		m.status = msg.status
		m.setEntryStarred(msg.entryID, msg.nextStarred)
		m.applyCurrentFilter()
		m.clampCursor()
		return m, nil
	case toggleActionErrorMsg:
		m.loading = false
		m.status = ""
		m.err = msg.err
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("Feedbin CLI\n")
	if m.inDetail {
		b.WriteString("m: toggle unread | s: toggle star | esc/backspace: back | q: quit\n\n")
		if m.status != "" {
			b.WriteString("Status: ")
			b.WriteString(m.status)
			b.WriteString("\n\n")
		}
		b.WriteString(m.detailView())
		return b.String()
	}
	b.WriteString("j/k or arrows: move | enter: details | a: all | u: unread | *: starred | m: unread | s: star | r: refresh | q: quit\n\n")

	if m.status != "" {
		b.WriteString("Status: ")
		b.WriteString(m.status)
		b.WriteString("\n\n")
	}

	if m.loading {
		b.WriteString("Loading entries...\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString("Error: ")
		b.WriteString(m.err.Error())
		b.WriteString("\n")
	}

	if len(m.entries) == 0 {
		b.WriteString("No entries available.\n")
		return b.String()
	}

	for i, entry := range m.entries {
		date := entry.PublishedAt.UTC().Format(time.DateOnly)
		cursorMarker := " "
		if i == m.cursor {
			cursorMarker = ">"
		}
		selectedMarker := " "
		if entry.ID == m.selectedID {
			selectedMarker = "*"
		}
		b.WriteString(fmt.Sprintf("%s%s%2d. [%s] %s %s", cursorMarker, selectedMarker, i+1, date, unreadMarker(entry), starredMarker(entry)))
		b.WriteString(entry.Title)
		if entry.FeedTitle != "" {
			b.WriteString(" - ")
			b.WriteString(entry.FeedTitle)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) detailView() string {
	if len(m.entries) == 0 {
		return "No entry selected.\n"
	}

	entry := m.entries[m.cursor]
	lines := buildDetailLines(entry, m.contentWidth())
	return renderDetailLines(lines, m.detailTop, m.detailBodyHeight())
}

func refreshCmd(service Service) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := service.Refresh(ctx, 1, 50)
		if err != nil {
			return refreshErrorMsg{err: err}
		}
		return refreshSuccessMsg{entries: entries}
	}
}

type toggleUnreadSuccessMsg struct {
	entryID    int64
	nextUnread bool
	status     string
}

type toggleStarredSuccessMsg struct {
	entryID     int64
	nextStarred bool
	status      string
}

type toggleActionErrorMsg struct {
	err error
}

func (m Model) toggleUnreadCurrent() (tea.Model, tea.Cmd) {
	if m.service == nil || len(m.entries) == 0 {
		return m, nil
	}
	entry := m.entries[m.cursor]
	m.loading = true
	m.status = ""
	m.err = nil
	return m, toggleUnreadCmd(m.service, entry.ID, entry.IsUnread)
}

func (m Model) toggleStarredCurrent() (tea.Model, tea.Cmd) {
	if m.service == nil || len(m.entries) == 0 {
		return m, nil
	}
	entry := m.entries[m.cursor]
	m.loading = true
	m.status = ""
	m.err = nil
	return m, toggleStarredCmd(m.service, entry.ID, entry.IsStarred)
}

func (m Model) switchFilter(filter string) (tea.Model, tea.Cmd) {
	if m.service == nil {
		return m, nil
	}
	m.loading = true
	m.status = ""
	m.err = nil
	return m, loadFilterCmd(m.service, filter, 50)
}

func toggleUnreadCmd(service Service, entryID int64, currentUnread bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nextUnread, err := service.ToggleUnread(ctx, entryID, currentUnread)
		if err != nil {
			return toggleActionErrorMsg{err: err}
		}

		status := "Marked as read"
		if nextUnread {
			status = "Marked as unread"
		}
		return toggleUnreadSuccessMsg{entryID: entryID, nextUnread: nextUnread, status: status}
	}
}

func toggleStarredCmd(service Service, entryID int64, currentStarred bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nextStarred, err := service.ToggleStarred(ctx, entryID, currentStarred)
		if err != nil {
			return toggleActionErrorMsg{err: err}
		}

		status := "Unstarred entry"
		if nextStarred {
			status = "Starred entry"
		}
		return toggleStarredSuccessMsg{entryID: entryID, nextStarred: nextStarred, status: status}
	}
}

func (m *Model) setEntryUnread(entryID int64, unread bool) {
	for i := range m.entries {
		if m.entries[i].ID == entryID {
			m.entries[i].IsUnread = unread
			return
		}
	}
}

func (m *Model) setEntryStarred(entryID int64, starred bool) {
	for i := range m.entries {
		if m.entries[i].ID == entryID {
			m.entries[i].IsStarred = starred
			return
		}
	}
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) applyCurrentFilter() {
	if m.filter == "all" {
		return
	}
	filtered := make([]feedbin.Entry, 0, len(m.entries))
	for _, entry := range m.entries {
		if m.filter == "unread" && entry.IsUnread {
			filtered = append(filtered, entry)
		}
		if m.filter == "starred" && entry.IsStarred {
			filtered = append(filtered, entry)
		}
	}
	m.entries = filtered
}

func (m Model) contentWidth() int {
	if m.width > 0 {
		return m.width - 1
	}
	return 100
}

func (m Model) detailBodyHeight() int {
	if m.height > 0 {
		usedByHeader := 4
		if m.status != "" {
			usedByHeader += 2
		}
		if h := m.height - usedByHeader; h > 3 {
			return h
		}
	}
	return 16
}

func buildDetailLines(entry feedbin.Entry, width int) []string {
	lines := make([]string, 0, 16)
	lines = append(lines, wrapText(entry.Title, width)...)
	lines = append(lines, strings.Repeat("=", max(1, min(width, len(entry.Title)))))
	lines = append(lines, "")

	if entry.FeedTitle != "" {
		lines = append(lines, wrapText("Feed: "+entry.FeedTitle, width)...)
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
		lines = append(lines, wrapText("Author: "+entry.Author, width)...)
	}
	if entry.URL != "" {
		lines = append(lines, wrapText("URL: "+entry.URL, width)...)
	}
	if entry.Summary != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(entry.Summary, width)...)
	}

	return lines
}

func renderDetailLines(lines []string, top, maxLines int) string {
	if len(lines) == 0 {
		return ""
	}
	if top < 0 {
		top = 0
	}
	if top > len(lines)-1 {
		top = len(lines) - 1
	}
	end := len(lines)
	if maxLines > 0 && top+maxLines < end {
		end = top + maxLines
	}
	return strings.Join(lines[top:end], "\n") + "\n"
}

func wrapText(text string, width int) []string {
	if width < 1 {
		return []string{text}
	}
	paragraphs := strings.Split(text, "\n")
	out := make([]string, 0, len(paragraphs))

	for _, p := range paragraphs {
		if p == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(p)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range words {
			for len(word) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				out = append(out, word[:width])
				word = word[width:]
			}

			if line == "" {
				line = word
				continue
			}
			if len(line)+1+len(word) <= width {
				line += " " + word
				continue
			}
			out = append(out, line)
			line = word
		}
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

func loadFilterCmd(service Service, filter string, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := service.ListCachedByFilter(ctx, limit, filter)
		if err != nil {
			return filterLoadErrorMsg{err: err}
		}
		return filterLoadSuccessMsg{filter: filter, entries: entries}
	}
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

func unreadMarker(entry feedbin.Entry) string {
	if entry.IsUnread {
		return "[U]"
	}
	return "[ ]"
}

func starredMarker(entry feedbin.Entry) string {
	if entry.IsStarred {
		return "[*]"
	}
	return "[ ]"
}
