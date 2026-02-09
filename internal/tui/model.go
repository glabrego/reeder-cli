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
	ToggleUnread(ctx context.Context, entryID int64, currentUnread bool) (bool, error)
	ToggleStarred(ctx context.Context, entryID int64, currentStarred bool) (bool, error)
}

type refreshSuccessMsg struct {
	entries []feedbin.Entry
}

type refreshErrorMsg struct {
	err error
}

type Model struct {
	service    Service
	entries    []feedbin.Entry
	cursor     int
	selectedID int64
	inDetail   bool
	loading    bool
	status     string
	err        error
}

func NewModel(service Service, entries []feedbin.Entry) Model {
	return Model{service: service, entries: entries}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inDetail {
			switch msg.String() {
			case "esc", "backspace":
				m.inDetail = false
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
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
			return m, nil
		case "r":
			if m.service == nil {
				return m, nil
			}
			m.loading = true
			m.status = ""
			m.err = nil
			return m, refreshCmd(m.service)
		case "m":
			return m.toggleUnreadCurrent()
		case "s":
			return m.toggleStarredCurrent()
		}
	case refreshSuccessMsg:
		m.loading = false
		m.entries = msg.entries
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
	case toggleUnreadSuccessMsg:
		m.loading = false
		m.err = nil
		m.status = msg.status
		m.setEntryUnread(msg.entryID, msg.nextUnread)
		return m, nil
	case toggleStarredSuccessMsg:
		m.loading = false
		m.err = nil
		m.status = msg.status
		m.setEntryStarred(msg.entryID, msg.nextStarred)
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
	b.WriteString("j/k or arrows: move | enter: details | m: unread | s: star | r: refresh | q: quit\n\n")

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
	var b strings.Builder
	b.WriteString(entry.Title)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", len(entry.Title)))
	b.WriteString("\n\n")

	if entry.FeedTitle != "" {
		b.WriteString("Feed: ")
		b.WriteString(entry.FeedTitle)
		b.WriteString("\n")
	}
	b.WriteString("Date: ")
	b.WriteString(entry.PublishedAt.UTC().Format(time.RFC3339))
	b.WriteString("\n")
	b.WriteString("Unread: ")
	if entry.IsUnread {
		b.WriteString("yes\n")
	} else {
		b.WriteString("no\n")
	}
	b.WriteString("Starred: ")
	if entry.IsStarred {
		b.WriteString("yes\n")
	} else {
		b.WriteString("no\n")
	}

	if entry.Author != "" {
		b.WriteString("Author: ")
		b.WriteString(entry.Author)
		b.WriteString("\n")
	}
	if entry.URL != "" {
		b.WriteString("URL: ")
		b.WriteString(entry.URL)
		b.WriteString("\n")
	}

	if entry.Summary != "" {
		b.WriteString("\n")
		b.WriteString(entry.Summary)
		b.WriteString("\n")
	}

	return b.String()
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
