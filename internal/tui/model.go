package tui

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type Service interface {
	Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
	ListCachedByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error)
	LoadMore(ctx context.Context, page, perPage int, filter string, limit int) ([]feedbin.Entry, int, error)
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

type loadMoreSuccessMsg struct {
	page         int
	fetchedCount int
	entries      []feedbin.Entry
}

type loadMoreErrorMsg struct {
	err error
}

type openURLSuccessMsg struct {
	status       string
	entryID      int64
	unreadBefore bool
	opened       bool
}

type openURLErrorMsg struct {
	err error
}

type clearStatusMsg struct {
	id int
}

type preferenceSaveErrorMsg struct {
	err error
}

type inlineImagePreviewSuccessMsg struct {
	entryID int64
	preview string
}

type inlineImagePreviewErrorMsg struct {
	entryID int64
	err     error
}

type Preferences struct {
	Compact         bool
	MarkReadOnOpen  bool
	ConfirmOpenRead bool
}

type Model struct {
	service                Service
	entries                []feedbin.Entry
	cursor                 int
	selectedID             int64
	filter                 string
	page                   int
	perPage                int
	lastFetchCount         int
	compact                bool
	markReadOnOpen         bool
	confirmOpenRead        bool
	pendingOpenReadEntryID int64
	lastOpenReadEntryID    int64
	lastOpenReadAt         time.Time
	autoReadDebounce       time.Duration
	showHelp               bool
	inDetail               bool
	detailTop              int
	width                  int
	height                 int
	loading                bool
	status                 string
	statusID               int
	err                    error
	openURLFn              func(string) error
	copyURLFn              func(string) error
	nowFn                  func() time.Time
	savePreferencesFn      func(Preferences) error
	renderImageFn          func(string, int) (string, error)
	imagePreview           map[int64]string
	imagePreviewErr        map[int64]string
	imagePreviewLoading    map[int64]bool
}

func NewModel(service Service, entries []feedbin.Entry) Model {
	return Model{
		service:             service,
		entries:             entries,
		filter:              "all",
		page:                1,
		perPage:             20,
		openURLFn:           openURLInBrowser,
		copyURLFn:           copyURLToClipboard,
		nowFn:               time.Now,
		autoReadDebounce:    5 * time.Second,
		renderImageFn:       renderInlineImagePreview,
		imagePreview:        make(map[int64]string),
		imagePreviewErr:     make(map[int64]string),
		imagePreviewLoading: make(map[int64]bool),
	}
}

func (m Model) Init() tea.Cmd {
	if m.service == nil {
		return nil
	}
	return refreshCmd(m.service, m.perPage)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "M":
			return m.confirmPendingOpenRead()
		}

		if m.showHelp {
			switch msg.String() {
			case "esc":
				m.showHelp = false
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			}
			return m, nil
		}

		if m.inDetail {
			switch msg.String() {
			case "esc", "backspace":
				m.inDetail = false
				m.detailTop = 0
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			case "o":
				return m.openCurrentURL()
			case "y":
				return m.copyCurrentURL()
			case "up", "k":
				if m.detailTop > 0 {
					m.detailTop--
				}
				return m, nil
			case "down", "j":
				entry := m.entries[m.cursor]
				lines := buildDetailLines(entry, m.contentWidth())
				lines = m.appendInlineImagePreview(lines, entry.ID)
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
			case "[":
				if len(m.entries) == 0 {
					return m, nil
				}
				if m.cursor > 0 {
					m.cursor--
					m.selectedID = m.entries[m.cursor].ID
					m.detailTop = 0
					return m, m.ensureInlineImagePreviewCmd()
				}
				return m, nil
			case "]":
				if len(m.entries) == 0 {
					return m, nil
				}
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					m.selectedID = m.entries[m.cursor].ID
					m.detailTop = 0
					return m, m.ensureInlineImagePreviewCmd()
				}
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "pgup", "ctrl+b":
			m.pageUpList()
			return m, nil
		case "pgdown", "ctrl+f":
			m.pageDownList()
			return m, nil
		case "g":
			if len(m.entries) > 0 {
				m.cursor = 0
			}
			return m, nil
		case "G":
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
			return m, nil
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
			return m, m.ensureInlineImagePreviewCmd()
		case "r":
			if m.service == nil {
				return m, nil
			}
			m.loading = true
			m.status = ""
			m.err = nil
			m.page = 1
			return m, refreshCmd(m.service, m.perPage)
		case "n":
			return m.loadMore()
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
		case "y":
			return m.copyCurrentURL()
		case "c":
			m.compact = !m.compact
			m.err = nil
			if m.compact {
				m.status = "Compact mode: on"
			} else {
				m.status = "Compact mode: off"
			}
			return m, persistPreferencesCmd(m.savePreferencesFn, m.preferences())
		case "t":
			m.markReadOnOpen = !m.markReadOnOpen
			m.err = nil
			if m.markReadOnOpen {
				m.status = "Mark read on open: on"
			} else {
				m.status = "Mark read on open: off"
			}
			return m, persistPreferencesCmd(m.savePreferencesFn, m.preferences())
		case "p":
			m.confirmOpenRead = !m.confirmOpenRead
			m.err = nil
			if m.confirmOpenRead {
				m.status = "Confirm open->read: on"
			} else {
				m.status = "Confirm open->read: off"
			}
			return m, persistPreferencesCmd(m.savePreferencesFn, m.preferences())
		}
	case refreshSuccessMsg:
		anchorID := m.anchorEntryID()
		m.loading = false
		m.entries = msg.entries
		m.applyCurrentFilter()
		m.restoreSelection(anchorID)
		m.err = nil
		return m, nil
	case loadMoreSuccessMsg:
		anchorID := m.anchorEntryID()
		m.loading = false
		m.err = nil
		m.lastFetchCount = msg.fetchedCount
		if msg.fetchedCount == 0 {
			m.status = "No more entries"
			return m, nil
		}
		m.page = msg.page
		m.entries = msg.entries
		m.restoreSelection(anchorID)
		m.status = fmt.Sprintf("Loaded page %d", msg.page)
		return m, nil
	case loadMoreErrorMsg:
		m.loading = false
		m.status = ""
		m.err = msg.err
		return m, nil
	case refreshErrorMsg:
		m.loading = false
		m.status = ""
		m.err = msg.err
		return m, nil
	case filterLoadSuccessMsg:
		anchorID := m.anchorEntryID()
		m.loading = false
		m.err = nil
		m.filter = msg.filter
		m.entries = msg.entries
		m.restoreSelection(anchorID)
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
		anchorID := m.anchorEntryID()
		m.loading = false
		m.err = nil
		m.status = msg.status
		m.setEntryUnread(msg.entryID, msg.nextUnread)
		m.applyCurrentFilter()
		m.restoreSelection(anchorID)
		return m, nil
	case toggleStarredSuccessMsg:
		anchorID := m.anchorEntryID()
		m.loading = false
		m.err = nil
		m.status = msg.status
		m.setEntryStarred(msg.entryID, msg.nextStarred)
		m.applyCurrentFilter()
		m.restoreSelection(anchorID)
		return m, nil
	case toggleActionErrorMsg:
		m.loading = false
		m.status = ""
		m.err = msg.err
		return m, nil
	case openURLSuccessMsg:
		m.err = nil
		m.status = msg.status
		if msg.opened && msg.unreadBefore && m.markReadOnOpen && m.service != nil {
			now := m.nowFn()
			if m.lastOpenReadEntryID == msg.entryID && now.Sub(m.lastOpenReadAt) < m.autoReadDebounce {
				m.status = "Skipped mark-read (debounced)"
				m.statusID++
				return m, clearStatusCmd(m.statusID, 3*time.Second)
			}
			if m.confirmOpenRead {
				m.pendingOpenReadEntryID = msg.entryID
				m.status = "Press Shift+M to confirm mark as read"
				m.statusID++
				return m, clearStatusCmd(m.statusID, 4*time.Second)
			}
			m.lastOpenReadEntryID = msg.entryID
			m.lastOpenReadAt = now
			m.loading = true
			return m, toggleUnreadCmd(m.service, msg.entryID, true)
		}
		m.statusID++
		return m, clearStatusCmd(m.statusID, 3*time.Second)
	case openURLErrorMsg:
		m.err = nil
		m.status = msg.err.Error()
		m.statusID++
		return m, clearStatusCmd(m.statusID, 4*time.Second)
	case clearStatusMsg:
		if msg.id == m.statusID {
			m.status = ""
		}
		return m, nil
	case preferenceSaveErrorMsg:
		m.err = msg.err
		m.status = "Could not persist UI preferences"
		return m, nil
	case inlineImagePreviewSuccessMsg:
		delete(m.imagePreviewLoading, msg.entryID)
		delete(m.imagePreviewErr, msg.entryID)
		m.imagePreview[msg.entryID] = msg.preview
		return m, nil
	case inlineImagePreviewErrorMsg:
		delete(m.imagePreviewLoading, msg.entryID)
		m.imagePreviewErr[msg.entryID] = msg.err.Error()
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("Feedbin CLI\n")
	if m.showHelp {
		b.WriteString("Help (? to close)\n\n")
		b.WriteString(m.helpView())
		b.WriteString("\n")
		b.WriteString(m.messagePanel())
		b.WriteString("\n")
		b.WriteString(m.footer())
		b.WriteString("\n")
		return b.String()
	}
	if m.inDetail {
		b.WriteString("j/k: scroll | [ ]: prev/next | o: open URL | y: copy URL | m: toggle unread | s: toggle star | esc/backspace: back | ?: help | q: quit\n\n")
		b.WriteString(m.detailView())
		b.WriteString("\n")
		b.WriteString(m.messagePanel())
		b.WriteString("\n")
		b.WriteString(m.footer())
		b.WriteString("\n")
		return b.String()
	}
	b.WriteString("j/k/arrows: move | g/G: top/bottom | pgup/pgdown: jump | c: compact | t: mark-on-open | p: confirm prompt | enter: details | a/u/*: filter | n: more | m/s: toggle | y: copy URL | ?: help | r: refresh | q: quit\n\n")

	if m.loading {
		b.WriteString("Loading entries...\n")
	} else {
		if len(m.entries) == 0 {
			b.WriteString("No entries available.\n")
		} else {
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
				var line string
				if m.compact {
					line = fmt.Sprintf("%s%s%2d. %s %s%s", cursorMarker, selectedMarker, i+1, unreadMarker(entry), starredMarker(entry), entry.Title)
				} else {
					line = fmt.Sprintf("%s%s%2d. [%s] %s %s%s", cursorMarker, selectedMarker, i+1, date, unreadMarker(entry), starredMarker(entry), entry.Title)
					if entry.FeedTitle != "" {
						line += " - " + entry.FeedTitle
					}
				}
				b.WriteString(renderActiveListLine(i == m.cursor, line))
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(m.messagePanel())
	b.WriteString("\n")
	b.WriteString(m.footer())
	b.WriteString("\n")

	return b.String()
}

func (m Model) detailView() string {
	if len(m.entries) == 0 {
		return "No entry selected.\n"
	}

	entry := m.entries[m.cursor]
	lines := buildDetailLines(entry, m.contentWidth())
	lines = m.appendInlineImagePreview(lines, entry.ID)
	return renderDetailLines(lines, m.detailTop, m.detailBodyHeight())
}

func (m Model) appendInlineImagePreview(lines []string, entryID int64) []string {
	if m.imagePreviewLoading[entryID] {
		return append(lines, "", "Inline image preview: loading...")
	}
	if preview := strings.TrimSpace(m.imagePreview[entryID]); preview != "" {
		out := append(lines, "", "Inline image preview:")
		out = append(out, strings.Split(preview, "\n")...)
		return out
	}
	if errMsg := strings.TrimSpace(m.imagePreviewErr[entryID]); errMsg != "" {
		return append(lines, "", "Inline image preview unavailable: "+errMsg)
	}
	return lines
}

func refreshCmd(service Service, perPage int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := service.Refresh(ctx, 1, perPage)
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
	return m, loadFilterCmd(m.service, filter, m.currentLimit())
}

func (m Model) loadMore() (tea.Model, tea.Cmd) {
	if m.service == nil {
		return m, nil
	}
	m.loading = true
	m.status = ""
	m.err = nil
	nextPage := m.page + 1
	return m, loadMoreCmd(m.service, nextPage, m.perPage, m.filter, m.currentLimit()+m.perPage)
}

func (m Model) openCurrentURL() (tea.Model, tea.Cmd) {
	if len(m.entries) == 0 {
		return m, nil
	}
	validURL, err := validateEntryURL(m.entries[m.cursor].URL)
	if err != nil {
		m.err = nil
		m.status = err.Error()
		m.statusID++
		return m, clearStatusCmd(m.statusID, 4*time.Second)
	}
	entry := m.entries[m.cursor]
	return m, openURLCmd(entry.ID, entry.IsUnread, validURL, m.openURLFn, m.copyURLFn)
}

func (m Model) copyCurrentURL() (tea.Model, tea.Cmd) {
	if len(m.entries) == 0 {
		return m, nil
	}
	validURL, err := validateEntryURL(m.entries[m.cursor].URL)
	if err != nil {
		m.err = nil
		m.status = err.Error()
		m.statusID++
		return m, clearStatusCmd(m.statusID, 4*time.Second)
	}
	return m, copyURLCmd(validURL, m.copyURLFn)
}

func (m Model) confirmPendingOpenRead() (tea.Model, tea.Cmd) {
	if m.pendingOpenReadEntryID == 0 || m.service == nil {
		m.status = "No pending mark-read action"
		return m, nil
	}
	entryID := m.pendingOpenReadEntryID
	m.pendingOpenReadEntryID = 0

	unread := m.entryUnreadState(entryID)
	if !unread {
		m.status = "Entry is already read"
		return m, nil
	}

	m.lastOpenReadEntryID = entryID
	m.lastOpenReadAt = m.nowFn()
	m.loading = true
	m.status = ""
	m.err = nil
	return m, toggleUnreadCmd(m.service, entryID, true)
}

func (m Model) entryUnreadState(entryID int64) bool {
	for _, entry := range m.entries {
		if entry.ID == entryID {
			return entry.IsUnread
		}
	}
	return true
}

func validateEntryURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("entry has no URL")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid URL format")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid URL host")
	}
	return trimmed, nil
}

func clearStatusCmd(id int, after time.Duration) tea.Cmd {
	return tea.Tick(after, func(time.Time) tea.Msg {
		return clearStatusMsg{id: id}
	})
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

func (m *Model) pageDownList() {
	if len(m.entries) == 0 {
		return
	}
	step := m.listPageStep()
	m.cursor += step
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
}

func (m *Model) pageUpList() {
	if len(m.entries) == 0 {
		return
	}
	step := m.listPageStep()
	m.cursor -= step
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) listPageStep() int {
	if m.height <= 0 {
		return 10
	}
	headerLines := 6
	if m.status != "" {
		headerLines += 2
	}
	step := m.height - headerLines
	if step < 3 {
		step = 3
	}
	return step
}

func (m Model) anchorEntryID() int64 {
	if m.selectedID != 0 {
		return m.selectedID
	}
	if len(m.entries) == 0 {
		return 0
	}
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return 0
	}
	return m.entries[m.cursor].ID
}

func (m *Model) restoreSelection(anchorID int64) {
	if len(m.entries) == 0 {
		m.cursor = 0
		m.selectedID = 0
		m.inDetail = false
		m.detailTop = 0
		return
	}

	if anchorID != 0 {
		for i, entry := range m.entries {
			if entry.ID == anchorID {
				m.cursor = i
				if m.selectedID != 0 {
					m.selectedID = anchorID
				}
				return
			}
		}
	}

	if m.selectedID != 0 {
		m.selectedID = 0
		m.inDetail = false
		m.detailTop = 0
	}
	m.clampCursor()
}

func (m Model) currentLimit() int {
	if m.page < 1 {
		return m.perPage
	}
	return m.page * m.perPage
}

func (m Model) footer() string {
	mode := "list"
	if m.inDetail {
		mode = "detail"
	}
	onOpen := "off"
	if m.markReadOnOpen {
		onOpen = "on"
	}
	confirm := "off"
	if m.confirmOpenRead {
		confirm = "on"
	}
	return fmt.Sprintf("Mode: %s | Filter: %s | Page: %d | Showing: %d | Last fetch: %d | Open->Read: %s | Confirm: %s", mode, m.filter, m.page, len(m.entries), m.lastFetchCount, onOpen, confirm)
}

func (m Model) messagePanel() string {
	status := "-"
	if m.status != "" {
		status = m.status
	}
	warning := "-"
	if m.err != nil {
		warning = m.err.Error()
	}
	state := "idle"
	if m.loading {
		state = "loading"
	}
	return fmt.Sprintf("Status: %s | Warning: %s | State: %s", status, warning, state)
}

func (m Model) helpView() string {
	lines := []string{
		"Navigation:",
		"  j/k or arrows move, g/G jump top/bottom, pgup/pgdown jump page",
		"Modes:",
		"  enter opens detail, esc/backspace returns to list",
		"Filters:",
		"  a all, u unread, * starred, n load next page",
		"Actions:",
		"  m toggle unread, s toggle starred, o open URL, y copy URL",
		"Options:",
		"  c compact mode, t mark-read-on-open, p confirm prompt, Shift+M confirm pending mark-read",
	}
	return strings.Join(lines, "\n")
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

	articleText := articleTextFromEntry(entry)
	if articleText != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(articleText, width)...)
	}

	imageURLs := imageURLsFromContent(entry.Content)
	if len(imageURLs) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Images:")
		for _, imageURL := range imageURLs {
			lines = append(lines, wrapText("- "+imageURL, width)...)
		}
	}

	return lines
}

func articleTextFromEntry(entry feedbin.Entry) string {
	content := strings.TrimSpace(entry.Content)
	if content != "" {
		if converted := htmlToText(content); converted != "" {
			return converted
		}
	}
	return strings.TrimSpace(entry.Summary)
}

func htmlToText(raw string) string {
	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n\n",
		"</div>", "\n\n",
		"</li>", "\n",
		"</h1>", "\n\n",
		"</h2>", "\n\n",
		"</h3>", "\n\n",
	)
	s := replacer.Replace(raw)

	reScriptStyle := regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	s = reScriptStyle.ReplaceAllString(s, "")

	reTags := regexp.MustCompile(`(?s)<[^>]+>`)
	s = reTags.ReplaceAllString(s, "")

	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed == "" {
			if len(out) > 0 && out[len(out)-1] == "" {
				continue
			}
			out = append(out, "")
			continue
		}
		out = append(out, trimmed)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

func imageURLsFromContent(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	reImgSrc := regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']?([^"'\s>]+)`)
	matches := reImgSrc.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(html.UnescapeString(m[1]))
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out
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

func loadMoreCmd(service Service, page, perPage int, filter string, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		entries, fetchedCount, err := service.LoadMore(ctx, page, perPage, filter, limit)
		if err != nil {
			return loadMoreErrorMsg{err: err}
		}
		return loadMoreSuccessMsg{page: page, fetchedCount: fetchedCount, entries: entries}
	}
}

func openURLCmd(entryID int64, unreadBefore bool, url string, openFn, copyFn func(string) error) tea.Cmd {
	return func() tea.Msg {
		if openFn != nil {
			if err := openFn(url); err == nil {
				return openURLSuccessMsg{status: "Opened URL in browser", entryID: entryID, unreadBefore: unreadBefore, opened: true}
			}
		}
		if copyFn != nil {
			if err := copyFn(url); err == nil {
				return openURLSuccessMsg{status: "Could not open browser, URL copied to clipboard", entryID: entryID, unreadBefore: unreadBefore, opened: false}
			}
		}
		return openURLErrorMsg{err: fmt.Errorf("could not open URL or copy to clipboard")}
	}
}

func copyURLCmd(url string, copyFn func(string) error) tea.Cmd {
	return func() tea.Msg {
		if copyFn != nil {
			if err := copyFn(url); err == nil {
				return openURLSuccessMsg{status: "URL copied to clipboard"}
			}
		}
		return openURLErrorMsg{err: fmt.Errorf("could not copy URL to clipboard")}
	}
}

func persistPreferencesCmd(saveFn func(Preferences) error, prefs Preferences) tea.Cmd {
	if saveFn == nil {
		return nil
	}
	return func() tea.Msg {
		if err := saveFn(prefs); err != nil {
			return preferenceSaveErrorMsg{err: err}
		}
		return nil
	}
}

func (m *Model) ensureInlineImagePreviewCmd() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	entry := m.entries[m.cursor]
	if strings.TrimSpace(entry.Content) == "" {
		return nil
	}
	imageURLs := imageURLsFromContent(entry.Content)
	if len(imageURLs) == 0 {
		return nil
	}
	if _, ok := m.imagePreview[entry.ID]; ok {
		return nil
	}
	if m.imagePreviewLoading[entry.ID] {
		return nil
	}
	m.imagePreviewLoading[entry.ID] = true
	return inlineImagePreviewCmd(entry.ID, imageURLs[0], m.contentWidth(), m.renderImageFn)
}

func inlineImagePreviewCmd(entryID int64, imageURL string, width int, renderFn func(string, int) (string, error)) tea.Cmd {
	if renderFn == nil {
		return nil
	}
	return func() tea.Msg {
		preview, err := renderFn(imageURL, width)
		if err != nil {
			return inlineImagePreviewErrorMsg{entryID: entryID, err: err}
		}
		return inlineImagePreviewSuccessMsg{entryID: entryID, preview: preview}
	}
}

func renderInlineImagePreview(imageURL string, width int) (string, error) {
	if width < 30 {
		width = 40
	}

	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return "", fmt.Errorf("chafa is not installed")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	imageData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	cmd := exec.Command(
		chafaPath,
		"--size", fmt.Sprintf("%dx18", width),
		"--format", preferredInlineImageFormat(),
		"-",
	)
	cmd.Stdin = bytes.NewReader(imageData)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("render image via chafa: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func preferredInlineImageFormat() string {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return "kitty"
	}
	if strings.EqualFold(os.Getenv("TERM_PROGRAM"), "iTerm.app") {
		return "iterm"
	}
	return "symbols"
}

func openURLInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Run()
}

func copyURLToClipboard(url string) error {
	commands := [][]string{
		{"pbcopy"},
		{"xclip", "-selection", "clipboard"},
		{"wl-copy"},
	}

	for _, c := range commands {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdin = bytes.NewBufferString(url)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("no clipboard command available")
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

func renderActiveListLine(active bool, line string) string {
	if !active {
		return line
	}
	return "\x1b[7m" + line + "\x1b[0m"
}

func (m *Model) ApplyPreferences(prefs Preferences) {
	m.compact = prefs.Compact
	m.markReadOnOpen = prefs.MarkReadOnOpen
	m.confirmOpenRead = prefs.ConfirmOpenRead
}

func (m *Model) SetPreferencesSaver(saveFn func(Preferences) error) {
	m.savePreferencesFn = saveFn
}

func (m Model) preferences() Preferences {
	return Preferences{
		Compact:         m.compact,
		MarkReadOnOpen:  m.markReadOnOpen,
		ConfirmOpenRead: m.confirmOpenRead,
	}
}
