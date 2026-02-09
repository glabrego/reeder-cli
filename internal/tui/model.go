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
	"sort"
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
	entries  []feedbin.Entry
	duration time.Duration
	source   string
}

type refreshErrorMsg struct {
	err      error
	duration time.Duration
	source   string
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
	cacheLoadDuration      time.Duration
	cacheLoadedEntries     int
	initialRefreshDuration time.Duration
	initialRefreshDone     bool
	initialRefreshFailed   bool
	collapsedFolders       map[string]bool
	collapsedFeeds         map[string]bool
	treeCursor             int
}

func NewModel(service Service, entries []feedbin.Entry) Model {
	seed := append([]feedbin.Entry(nil), entries...)
	sortEntriesForTree(seed)
	m := Model{
		service:             service,
		entries:             seed,
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
		collapsedFolders:    make(map[string]bool),
		collapsedFeeds:      make(map[string]bool),
	}
	rows := m.treeRows()
	m.treeCursor = firstArticleRow(rows)
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
	if len(rows) > 0 && rows[m.treeCursor].Kind == treeRowArticle {
		m.cursor = rows[m.treeCursor].EntryIndex
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if m.service == nil {
		return nil
	}
	return refreshCmd(m.service, m.perPage, "init")
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
			rows := m.treeRows()
			if len(rows) > 0 {
				m.treeCursor = 0
				m.syncCursorFromTree()
			}
			return m, nil
		case "G":
			rows := m.treeRows()
			if len(rows) > 0 {
				m.treeCursor = len(rows) - 1
				m.syncCursorFromTree()
			}
			return m, nil
		case "up", "k":
			m.moveCursorBy(-1)
			return m, nil
		case "down", "j":
			m.moveCursorBy(1)
			return m, nil
		case "enter":
			rows := m.treeRows()
			if len(rows) == 0 {
				return m, nil
			}
			m.ensureTreeCursorValid()
			row := rows[m.treeCursor]
			if row.Kind != treeRowArticle {
				m.toggleCurrentTreeNode()
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
			return m, refreshCmd(m.service, m.perPage, "manual")
		case "n":
			return m.loadMore()
		case "a":
			return m.switchFilter("all")
		case "u":
			if m.filter == "unread" {
				return m.switchFilter("all")
			}
			return m.switchFilter("unread")
		case "*":
			if m.filter == "starred" {
				return m.switchFilter("all")
			}
			return m.switchFilter("starred")
		case "m":
			m.ensureCursorVisible()
			if !m.currentTreeRowIsArticle() {
				return m, nil
			}
			return m.toggleUnreadCurrent()
		case "s":
			m.ensureCursorVisible()
			if !m.currentTreeRowIsArticle() {
				return m, nil
			}
			return m.toggleStarredCurrent()
		case "y":
			m.ensureCursorVisible()
			if !m.currentTreeRowIsArticle() {
				return m, nil
			}
			return m.copyCurrentURL()
		case "left", "h":
			m.collapseCurrentTreeNode()
			return m, nil
		case "right", "l":
			m.expandCurrentTreeNode()
			return m, nil
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
		if msg.source == "init" {
			m.initialRefreshDuration = msg.duration
			m.initialRefreshDone = true
			m.initialRefreshFailed = false
		}
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
		sortEntriesForTree(m.entries)
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
		if msg.source == "init" {
			m.initialRefreshDuration = msg.duration
			m.initialRefreshDone = true
			m.initialRefreshFailed = true
		}
		return m, nil
	case filterLoadSuccessMsg:
		anchorID := m.anchorEntryID()
		m.loading = false
		m.err = nil
		m.filter = msg.filter
		m.entries = msg.entries
		sortEntriesForTree(m.entries)
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
			rows := m.treeRows()
			m.ensureTreeCursorValid()
			visiblePos := 0
			for i, row := range rows {
				switch row.Kind {
				case treeRowFolder:
					prefix := "▾ "
					if m.collapsedFolders[row.Folder] {
						prefix = "▸ "
					}
					b.WriteString(renderActiveListLine(i == m.treeCursor, prefix+row.Label))
					b.WriteString("\n")
				case treeRowFeed:
					prefix := "  ▾ "
					if row.Folder == "" {
						prefix = "▾ "
					}
					if m.collapsedFeeds[treeFeedKey(row.Folder, row.Feed)] {
						if row.Folder == "" {
							prefix = "▸ "
						} else {
							prefix = "  ▸ "
						}
					}
					b.WriteString(renderActiveListLine(i == m.treeCursor, prefix+row.Label))
					b.WriteString("\n")
				case treeRowArticle:
					b.WriteString(m.renderEntryLine(row.EntryIndex, visiblePos, i == m.treeCursor))
					b.WriteString("\n")
					visiblePos++
				}
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

func refreshCmd(service Service, perPage int, source string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		start := time.Now()

		entries, err := service.Refresh(ctx, 1, perPage)
		if err != nil {
			return refreshErrorMsg{err: err, duration: time.Since(start), source: source}
		}
		return refreshSuccessMsg{entries: entries, duration: time.Since(start), source: source}
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
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	step := m.listPageStep()
	m.treeCursor += step
	if m.treeCursor >= len(rows) {
		m.treeCursor = len(rows) - 1
	}
	m.syncCursorFromTree()
}

func (m *Model) pageUpList() {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	step := m.listPageStep()
	m.treeCursor -= step
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
	m.syncCursorFromTree()
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
				m.setTreeCursorForEntry(i)
				m.ensureCursorVisible()
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
	m.setTreeCursorForEntry(m.cursor)
	m.ensureCursorVisible()
}

func (m *Model) setTreeCursorForEntry(entryIndex int) {
	rows := m.treeRows()
	for i, row := range rows {
		if row.Kind == treeRowArticle && row.EntryIndex == entryIndex {
			m.treeCursor = i
			return
		}
	}
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
	return fmt.Sprintf("Status: %s | Warning: %s | State: %s | Startup: %s", status, warning, state, m.startupMetrics())
}

func (m Model) startupMetrics() string {
	cachePart := "cache n/a"
	if m.cacheLoadDuration > 0 || m.cacheLoadedEntries > 0 {
		cachePart = fmt.Sprintf("cache %dms (%d entries)", m.cacheLoadDuration.Milliseconds(), m.cacheLoadedEntries)
	}
	refreshPart := "initial refresh pending"
	if m.initialRefreshDone {
		if m.initialRefreshFailed {
			refreshPart = fmt.Sprintf("initial refresh failed in %dms", m.initialRefreshDuration.Milliseconds())
		} else {
			refreshPart = fmt.Sprintf("initial refresh %dms", m.initialRefreshDuration.Milliseconds())
		}
	}
	return cachePart + ", " + refreshPart
}

func (m Model) helpView() string {
	lines := []string{
		"Navigation:",
		"  j/k or arrows move, g/G jump top/bottom, pgup/pgdown jump page",
		"Tree-style List:",
		"  default list is grouped by folder(host) and feed title",
		"  left/h collapses current feed/folder, right/l expands",
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
		sortEntriesForTree(m.entries)
		m.ensureCursorVisible()
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
	sortEntriesForTree(m.entries)
	m.ensureCursorVisible()
}

func folderNameForEntry(entry feedbin.Entry) string {
	return strings.TrimSpace(entry.FeedFolder)
}

func feedNameForEntry(entry feedbin.Entry) string {
	name := strings.TrimSpace(entry.FeedTitle)
	if name == "" {
		return "unknown feed"
	}
	return name
}

func sortEntriesForTree(entries []feedbin.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		ai := entries[i]
		aj := entries[j]
		fi, fkindi := topCollectionLabelForEntry(ai)
		fj, fkindj := topCollectionLabelForEntry(aj)
		if fkindi != fkindj {
			return fkindi < fkindj
		}
		if fi != fj {
			return fi < fj
		}
		ti := strings.ToLower(feedNameForEntry(ai))
		tj := strings.ToLower(feedNameForEntry(aj))
		if ti != tj {
			return ti < tj
		}
		if !ai.PublishedAt.Equal(aj.PublishedAt) {
			return ai.PublishedAt.After(aj.PublishedAt)
		}
		return false
	})
}

func treeFeedKey(folder, feed string) string {
	return folder + "\x00" + feed
}

func (m Model) visibleEntryIndices() []int {
	rows := m.treeRows()
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.Kind == treeRowArticle {
			out = append(out, row.EntryIndex)
		}
	}
	return out
}

func (m Model) renderEntryLine(idx, visiblePos int, active bool) string {
	entry := m.entries[idx]
	date := entry.PublishedAt.UTC().Format(time.DateOnly)
	cursorMarker := " "
	if active {
		cursorMarker = ">"
	}
	selectedMarker := " "
	if entry.ID == m.selectedID {
		selectedMarker = "*"
	}
	styledTitle := styleArticleTitle(entry, entry.Title)
	if m.compact {
		return renderActiveListLine(active, fmt.Sprintf("    %s%s%2d. %s %s%s", cursorMarker, selectedMarker, visiblePos+1, unreadMarker(entry), starredMarker(entry), styledTitle))
	}
	return renderActiveListLine(active, fmt.Sprintf("    %s%s%2d. [%s] %s %s%s", cursorMarker, selectedMarker, visiblePos+1, date, unreadMarker(entry), starredMarker(entry), styledTitle))
}

func styleArticleTitle(entry feedbin.Entry, title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return title
	}

	switch {
	case entry.IsUnread && entry.IsStarred:
		return "\x1b[1;3m" + title + "\x1b[0m"
	case entry.IsUnread:
		return "\x1b[1m" + title + "\x1b[0m"
	case entry.IsStarred:
		return "\x1b[3;90m" + title + "\x1b[0m"
	default:
		return "\x1b[90m" + title + "\x1b[0m"
	}
}

func (m *Model) ensureCursorVisible() {
	rows := m.treeRows()
	if len(rows) == 0 {
		m.treeCursor = 0
		m.cursor = 0
		return
	}
	m.ensureTreeCursorValid()
	m.syncCursorFromTree()
}

func (m *Model) ensureTreeCursorValid() {
	rows := m.treeRows()
	if len(rows) == 0 {
		m.treeCursor = 0
		return
	}
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
	if m.treeCursor >= len(rows) {
		m.treeCursor = len(rows) - 1
	}
}

func (m *Model) syncCursorFromTree() {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	if rows[m.treeCursor].Kind == treeRowArticle {
		m.cursor = rows[m.treeCursor].EntryIndex
		return
	}
	for i := m.treeCursor + 1; i < len(rows); i++ {
		if rows[i].Kind == treeRowArticle {
			m.cursor = rows[i].EntryIndex
			return
		}
	}
	for i := m.treeCursor - 1; i >= 0; i-- {
		if rows[i].Kind == treeRowArticle {
			m.cursor = rows[i].EntryIndex
			return
		}
	}
}

func (m *Model) moveCursorBy(delta int) {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	m.treeCursor += delta
	if m.treeCursor < 0 {
		m.treeCursor = 0
	}
	if m.treeCursor >= len(rows) {
		m.treeCursor = len(rows) - 1
	}
	m.syncCursorFromTree()
}

func (m Model) currentTreeRowIsArticle() bool {
	rows := m.treeRows()
	if len(rows) == 0 {
		return false
	}
	if m.treeCursor < 0 || m.treeCursor >= len(rows) {
		return false
	}
	return rows[m.treeCursor].Kind == treeRowArticle
}

func (m *Model) toggleCurrentTreeNode() {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	row := rows[m.treeCursor]
	switch row.Kind {
	case treeRowFolder:
		if m.collapsedFolders[row.Folder] {
			m.collapsedFolders[row.Folder] = false
			m.status = "Expanded folder: " + row.Folder
		} else {
			m.collapsedFolders[row.Folder] = true
			m.status = "Collapsed folder: " + row.Folder
		}
	case treeRowFeed:
		key := treeFeedKey(row.Folder, row.Feed)
		if m.collapsedFeeds[key] {
			m.collapsedFeeds[key] = false
			m.status = "Expanded feed: " + row.Feed
		} else {
			m.collapsedFeeds[key] = true
			m.status = "Collapsed feed: " + row.Feed
		}
	}
	m.ensureCursorVisible()
}

func (m *Model) collapseCurrentTreeNode() {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	row := rows[m.treeCursor]
	folder := row.Folder
	feed := row.Feed
	if row.Kind == treeRowArticle {
		entry := m.entries[row.EntryIndex]
		folder = folderNameForEntry(entry)
		feed = feedNameForEntry(entry)
	}
	feedKey := treeFeedKey(folder, feed)
	if feed != "" && !m.collapsedFeeds[feedKey] {
		m.collapsedFeeds[feedKey] = true
		m.status = "Collapsed feed: " + feed
		m.ensureCursorVisible()
		return
	}
	if folder != "" && !m.collapsedFolders[folder] {
		m.collapsedFolders[folder] = true
		m.status = "Collapsed folder: " + folder
		m.ensureCursorVisible()
	}
}

func (m *Model) expandCurrentTreeNode() {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}

	m.ensureTreeCursorValid()
	row := rows[m.treeCursor]
	folder := row.Folder
	feed := row.Feed
	if row.Kind == treeRowArticle {
		entry := m.entries[row.EntryIndex]
		folder = folderNameForEntry(entry)
		feed = feedNameForEntry(entry)
	}
	feedKey := treeFeedKey(folder, feed)
	if folder != "" && m.collapsedFolders[folder] {
		m.collapsedFolders[folder] = false
		m.status = "Expanded folder: " + folder
		m.ensureCursorVisible()
		return
	}
	if feed != "" && m.collapsedFeeds[feedKey] {
		m.collapsedFeeds[feedKey] = false
		m.status = "Expanded feed: " + feed
		m.ensureCursorVisible()
		return
	}
	if m.expandNextCollapsedFolder(folder) || m.expandNextCollapsedFeed(feedKey) {
		m.ensureCursorVisible()
	}
}

func (m *Model) expandNextCollapsedFolder(preferred string) bool {
	candidates := make([]string, 0, len(m.collapsedFolders))
	for folder, collapsed := range m.collapsedFolders {
		if collapsed && folder != "" {
			candidates = append(candidates, folder)
		}
	}
	if len(candidates) == 0 {
		return false
	}
	sort.Strings(candidates)

	target := candidates[0]
	for _, folder := range candidates {
		if folder != preferred {
			target = folder
			break
		}
	}

	m.collapsedFolders[target] = false
	m.status = "Expanded folder: " + target
	return true
}

type treeFeedGroup struct {
	Name         string
	EntryIndices []int
}

type treeCollection struct {
	Kind  string // "folder" or "top_feed"
	Key   string
	Label string
	Feeds []treeFeedGroup
}

type treeRowKind string

const (
	treeRowFolder  treeRowKind = "folder"
	treeRowFeed    treeRowKind = "feed"
	treeRowArticle treeRowKind = "article"
)

type treeRow struct {
	Kind       treeRowKind
	Label      string
	Folder     string
	Feed       string
	EntryIndex int
}

func buildTreeCollections(entries []feedbin.Entry) []treeCollection {
	collections := make([]treeCollection, 0, 16)
	collectionIndex := make(map[string]int)
	feedIndexByCollection := make(map[string]map[string]int)

	for idx, entry := range entries {
		collectionLabel, collectionKind := topCollectionLabelForEntry(entry)
		collectionKey := collectionKind + "\x00" + collectionLabel
		ci, ok := collectionIndex[collectionKey]
		if !ok {
			collections = append(collections, treeCollection{
				Kind:  collectionKind,
				Key:   collectionLabel,
				Label: collectionLabel,
				Feeds: make([]treeFeedGroup, 0, 8),
			})
			ci = len(collections) - 1
			collectionIndex[collectionKey] = ci
			feedIndexByCollection[collectionKey] = make(map[string]int)
		}

		feedName := feedNameForEntry(entry)
		if collectionKind == "top_feed" {
			feedName = collectionLabel
		}
		fi, ok := feedIndexByCollection[collectionKey][feedName]
		if !ok {
			collections[ci].Feeds = append(collections[ci].Feeds, treeFeedGroup{Name: feedName})
			fi = len(collections[ci].Feeds) - 1
			feedIndexByCollection[collectionKey][feedName] = fi
		}
		collections[ci].Feeds[fi].EntryIndices = append(collections[ci].Feeds[fi].EntryIndices, idx)
	}

	return collections
}

func (m Model) treeRows() []treeRow {
	tree := buildTreeCollections(m.entries)
	rows := make([]treeRow, 0, len(m.entries)+len(tree)*2)
	for _, collection := range tree {
		if collection.Kind == "folder" {
			rows = append(rows, treeRow{
				Kind:   treeRowFolder,
				Label:  collection.Label,
				Folder: collection.Key,
			})
			if m.collapsedFolders[collection.Key] {
				continue
			}
			for _, fg := range collection.Feeds {
				rows = append(rows, treeRow{
					Kind:   treeRowFeed,
					Label:  fg.Name,
					Folder: collection.Key,
					Feed:   fg.Name,
				})
				if m.collapsedFeeds[treeFeedKey(collection.Key, fg.Name)] {
					continue
				}
				for _, idx := range fg.EntryIndices {
					rows = append(rows, treeRow{
						Kind:       treeRowArticle,
						Folder:     collection.Key,
						Feed:       fg.Name,
						EntryIndex: idx,
					})
				}
			}
			continue
		}

		rows = append(rows, treeRow{
			Kind:  treeRowFeed,
			Label: collection.Label,
			Feed:  collection.Label,
		})
		if m.collapsedFeeds[treeFeedKey("", collection.Label)] {
			continue
		}
		if len(collection.Feeds) == 0 {
			continue
		}
		for _, idx := range collection.Feeds[0].EntryIndices {
			rows = append(rows, treeRow{
				Kind:       treeRowArticle,
				Feed:       collection.Label,
				EntryIndex: idx,
			})
		}
	}
	return rows
}

func firstArticleRow(rows []treeRow) int {
	for i, row := range rows {
		if row.Kind == treeRowArticle {
			return i
		}
	}
	return 0
}

func topCollectionLabelForEntry(entry feedbin.Entry) (label string, kind string) {
	if folder := folderNameForEntry(entry); folder != "" {
		return folder, "folder"
	}
	return feedNameForEntry(entry), "top_feed"
}

func (m *Model) expandNextCollapsedFeed(preferred string) bool {
	candidates := make([]string, 0, len(m.collapsedFeeds))
	for key, collapsed := range m.collapsedFeeds {
		if collapsed {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return false
	}
	sort.Strings(candidates)

	target := candidates[0]
	for _, key := range candidates {
		if key != preferred {
			target = key
			break
		}
	}

	m.collapsedFeeds[target] = false
	_, feed := splitTreeFeedKey(target)
	m.status = "Expanded feed: " + feed
	return true
}

func splitTreeFeedKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return key, key
	}
	return parts[0], parts[1]
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

func (m *Model) SetStartupCacheStats(duration time.Duration, entries int) {
	m.cacheLoadDuration = duration
	m.cacheLoadedEntries = entries
}
