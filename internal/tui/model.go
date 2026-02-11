package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/reeder-cli/internal/feedbin"
	article "github.com/glabrego/reeder-cli/internal/render/article"
	tuitheme "github.com/glabrego/reeder-cli/internal/tui/theme"
	tuitree "github.com/glabrego/reeder-cli/internal/tui/tree"
	tuiview "github.com/glabrego/reeder-cli/internal/tui/view"
)

type Service interface {
	Refresh(ctx context.Context, page, perPage int) ([]feedbin.Entry, error)
	ListCachedByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error)
	SearchCached(ctx context.Context, limit int, filter, query string) ([]feedbin.Entry, error)
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

type searchLoadSuccessMsg struct {
	filter  string
	query   string
	entries []feedbin.Entry
}

type searchLoadErrorMsg struct {
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
	RelativeTime    bool
	ShowNumbers     bool
}

var reANSICodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)
var uiTheme = tuitheme.Default()

const inlineImagePreviewAnchor = "__INLINE_IMAGE_PREVIEW_ANCHOR__"
const inlineImagePreviewRows = 18

type Model struct {
	service                Service
	entries                []feedbin.Entry
	cursor                 int
	selectedID             int64
	filter                 string
	searchQuery            string
	searchMatchCount       int
	searchInput            string
	searchInputMode        bool
	page                   int
	perPage                int
	lastFetchCount         int
	compact                bool
	showNumbers            bool
	markReadOnOpen         bool
	confirmOpenRead        bool
	relativeTime           bool
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
	articleOptions         article.Options
	inlineImagePreview     bool
	cacheLoadDuration      time.Duration
	cacheLoadedEntries     int
	initialRefreshDuration time.Duration
	initialRefreshDone     bool
	initialRefreshFailed   bool
	collapsedFolders       map[string]bool
	collapsedFeeds         map[string]bool
	collapsedSections      map[string]bool
	nerdIcons              bool
	nerdMode               bool
	treeCursor             int
}

func NewModel(service Service, entries []feedbin.Entry) Model {
	seed := append([]feedbin.Entry(nil), entries...)
	sortEntriesForTree(seed)
	initialPerPage := defaultPerPageFromEnv()
	seed = limitEntries(seed, initialPerPage)
	m := Model{
		service:             service,
		entries:             seed,
		filter:              "all",
		page:                1,
		perPage:             initialPerPage,
		openURLFn:           openURLInBrowser,
		copyURLFn:           copyURLToClipboard,
		nowFn:               time.Now,
		autoReadDebounce:    5 * time.Second,
		relativeTime:        true,
		renderImageFn:       renderInlineImagePreview,
		imagePreview:        make(map[int64]string),
		imagePreviewErr:     make(map[int64]string),
		imagePreviewLoading: make(map[int64]bool),
		articleOptions:      article.DefaultOptions,
		inlineImagePreview:  parseEnvBool("FEEDBIN_INLINE_IMAGE_PREVIEW"),
		collapsedFolders:    make(map[string]bool),
		collapsedFeeds:      make(map[string]bool),
		collapsedSections:   make(map[string]bool),
		nerdIcons:           parseEnvBool("FEEDBIN_NERD_ICONS"),
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

func (m *Model) SetNerdMode(nerd bool) {
	m.nerdMode = nerd
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
		if m.page == 1 && !m.initialRefreshDone {
			m.perPage = m.maxArticlesPerPage()
			m.entries = limitEntries(m.entries, m.currentLimit())
			m.ensureCursorVisible()
		}
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

		if m.searchInputMode {
			switch msg.String() {
			case "enter":
				return m.applySearchInput()
			case "ctrl+l":
				m.searchInput = ""
				return m, nil
			case "esc":
				m.searchInputMode = false
				m.searchInput = ""
				m.status = "Search canceled"
				m.statusID++
				return m, clearStatusCmd(m.statusID, 3*time.Second)
			case "ctrl+c":
				return m, tea.Quit
			case "backspace", "ctrl+h":
				if len(m.searchInput) > 0 {
					_, size := utf8.DecodeLastRuneInString(m.searchInput)
					m.searchInput = m.searchInput[:len(m.searchInput)-size]
				}
				return m, nil
			}
			if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
				m.searchInput += string(msg.Runes)
			}
			return m, nil
		}

		if m.inDetail {
			switch msg.String() {
			case "esc", "backspace":
				m.inDetail = false
				m.detailTop = 0
				return m, tea.ClearScreen
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
				lines := m.detailLines(entry)
				maxTop := 0
				if max := len(lines) - m.detailBodyHeight(); max > 0 {
					maxTop = max
				}
				if m.detailTop < maxTop {
					m.detailTop++
				}
				return m, nil
			case "U":
				return m.toggleUnreadCurrent()
			case "S":
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
		case "ctrl+l":
			return m.clearSearch()
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
		case "[":
			m.jumpToSection(-1)
			return m, nil
		case "]":
			m.jumpToSection(1)
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
		case "r", "R", "ctrl+r":
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
		case "/":
			m.searchInputMode = true
			m.searchInput = m.searchQuery
			m.status = "Search mode: type query and press enter"
			m.err = nil
			return m, nil
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
		case "U":
			m.ensureCursorVisible()
			if !m.currentTreeRowIsArticle() {
				return m, nil
			}
			return m.toggleUnreadCurrent()
		case "S":
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
		case "N":
			m.showNumbers = !m.showNumbers
			m.err = nil
			if m.showNumbers {
				m.status = "Article numbering: on"
			} else {
				m.status = "Article numbering: off"
			}
			return m, persistPreferencesCmd(m.savePreferencesFn, m.preferences())
		case "d":
			m.relativeTime = !m.relativeTime
			m.err = nil
			if m.relativeTime {
				m.status = "Time format: relative"
			} else {
				m.status = "Time format: absolute"
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
		m.entries = limitEntries(msg.entries, m.currentLimit())
		m.applyCurrentFilter()
		if m.searchQuery != "" {
			m.searchMatchCount = len(m.entries)
		}
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
		m.applyCurrentFilter()
		if m.searchQuery != "" {
			m.searchMatchCount = len(m.entries)
		}
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
	case searchLoadSuccessMsg:
		anchorID := m.anchorEntryID()
		m.loading = false
		m.err = nil
		m.filter = msg.filter
		m.searchQuery = strings.TrimSpace(msg.query)
		m.entries = msg.entries
		m.searchMatchCount = len(msg.entries)
		sortEntriesForTree(m.entries)
		m.restoreSelection(anchorID)
		if m.searchQuery == "" {
			m.status = "Search cleared"
			m.searchMatchCount = 0
		} else {
			m.status = fmt.Sprintf("Search: %s (%d matches)", m.searchQuery, m.searchMatchCount)
		}
		return m, nil
	case searchLoadErrorMsg:
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
		return m, tea.ClearScreen
	case inlineImagePreviewErrorMsg:
		delete(m.imagePreviewLoading, msg.entryID)
		m.imagePreviewErr[msg.entryID] = msg.err.Error()
		return m, tea.ClearScreen
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	if !m.inDetail && supportsKittyGraphics() {
		b.WriteString(clearKittyGraphicsSequence())
	}
	b.WriteString(m.titleBar())
	b.WriteString("\n")
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
	b.WriteString(m.toolbar())
	b.WriteString("\n\n")
	if m.inDetail {
		b.WriteString(m.detailView())
		b.WriteString("\n")
		b.WriteString(m.messagePanel())
		b.WriteString("\n")
		b.WriteString(m.footer())
		b.WriteString("\n")
		return b.String()
	}
	if m.searchInputMode {
		b.WriteString(fmt.Sprintf("Search> %s\n\n", m.searchInput))
	} else if m.searchQuery != "" {
		b.WriteString(fmt.Sprintf("Search: %s\n\n", m.searchQuery))
	}

	if m.loading {
		b.WriteString("Loading entries...\n")
	} else {
		if len(m.entries) == 0 {
			b.WriteString("No entries available.\n")
		} else {
			rows := m.treeRows()
			sectionUnreadCounts := m.unreadCountsBySection()
			folderUnreadCounts, feedUnreadCounts := m.unreadCountsByTreeNode()
			m.ensureTreeCursorValid()
			start, end, visiblePos := m.listWindow(rows)
			for i := start; i < end; i++ {
				row := rows[i]
				switch row.Kind {
				case treeRowSection:
					b.WriteString(m.renderSectionLine(row.Label, sectionUnreadCounts[row.Label], i == m.treeCursor))
					b.WriteString("\n")
				case treeRowFolder:
					prefix := "▾ "
					if m.collapsedFolders[row.Folder] {
						prefix = "▸ "
					}
					b.WriteString(m.renderTreeNodeLine(prefix+row.Label, folderUnreadCounts[row.Folder], i == m.treeCursor))
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
					b.WriteString(m.renderTreeNodeLine(prefix+row.Label, feedUnreadCounts[treeFeedKey(row.Folder, row.Feed)], i == m.treeCursor))
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

func (m Model) titleBar() string {
	label := uiTheme.Title.Render("Reeder CLI")
	if m.nerdMode {
		return label
	}
	mode := "List"
	if m.inDetail {
		mode = "Detail"
	}
	modePill := uiTheme.ModePill.Render(mode)
	return label + "  " + modePill
}

func (m Model) toolbar() string {
	return tuiview.Toolbar(m.nerdMode, m.inDetail)
}

func (m Model) detailView() string {
	if len(m.entries) == 0 {
		return "No entry selected.\n"
	}

	entry := m.entries[m.cursor]
	lines := m.detailLines(entry)
	return renderDetailLines(lines, m.detailTop, m.detailBodyHeight())
}

func (m Model) detailLines(entry feedbin.Entry) []string {
	lines := buildDetailLines(entry, m.detailContentWidth(), m.articleOptions)
	lines = m.appendInlineImagePreview(lines, entry.ID)
	return leftPadLines(lines, m.detailHorizontalMargin())
}

func (m Model) appendInlineImagePreview(lines []string, entryID int64) []string {
	if !m.inlineImagePreview {
		return lines
	}
	previewLines := make([]string, 0, 3)
	if m.imagePreviewLoading[entryID] {
		previewLines = append(previewLines, "Loading image preview...")
	}
	if len(previewLines) == 0 {
		if previewRaw := m.imagePreview[entryID]; strings.TrimSpace(previewRaw) != "" {
			if containsKittyGraphicsEscape(previewRaw) {
				previewLines = append(previewLines, strings.TrimRight(previewRaw, "\r\n"))
			} else {
				previewSplit := strings.Split(strings.TrimRight(previewRaw, "\r\n"), "\n")
				previewLines = centerLines(previewSplit, m.detailContentWidth())
			}
		}
	}
	if len(previewLines) == 0 {
		if errMsg := strings.TrimSpace(m.imagePreviewErr[entryID]); errMsg != "" {
			previewLines = append(previewLines, "Image preview unavailable: "+errMsg)
		}
	}

	anchored := false
	out := make([]string, 0, len(lines)+len(previewLines)+1)
	for _, line := range lines {
		if line != inlineImagePreviewAnchor {
			out = append(out, line)
			continue
		}
		anchored = true
		if len(previewLines) > 0 {
			out = append(out, previewLines...)
		}
	}
	if anchored {
		return out
	}
	if len(previewLines) > 0 {
		out := append([]string{}, lines...)
		out = append(out, "")
		out = append(out, previewLines...)
		return out
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
	if m.searchQuery != "" {
		return m, loadSearchCmd(m.service, filter, m.searchQuery, m.currentLimit())
	}
	return m, loadFilterCmd(m.service, filter, m.currentLimit())
}

func (m Model) applySearchInput() (tea.Model, tea.Cmd) {
	if m.service == nil {
		return m, nil
	}
	query := strings.TrimSpace(m.searchInput)
	m.searchInputMode = false
	m.searchInput = query
	m.loading = true
	m.status = ""
	m.err = nil
	return m, loadSearchCmd(m.service, m.filter, query, m.currentLimit())
}

func (m Model) clearSearch() (tea.Model, tea.Cmd) {
	if m.service == nil {
		return m, nil
	}
	if strings.TrimSpace(m.searchQuery) == "" && strings.TrimSpace(m.searchInput) == "" && !m.searchInputMode {
		return m, nil
	}
	m.searchQuery = ""
	m.searchInput = ""
	m.searchInputMode = false
	m.searchMatchCount = 0
	m.loading = true
	m.status = ""
	m.err = nil
	return m, loadFilterCmd(m.service, m.filter, m.currentLimit())
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
	if !m.nerdMode {
		mode := "list"
		if m.inDetail {
			mode = "detail"
		}
		return tuiview.CompactFooter(
			mode,
			m.filter,
			m.page,
			len(m.entries),
			m.searchQuery,
			m.searchMatchCount,
			uiTheme,
		)
	}
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
	timeFormat := "absolute"
	if m.relativeTime {
		timeFormat = "relative"
	}
	numbering := "off"
	if m.showNumbers {
		numbering = "on"
	}
	return tuiview.NerdFooter(
		mode,
		m.filter,
		m.page,
		len(m.entries),
		m.lastFetchCount,
		timeFormat,
		numbering,
		onOpen,
		confirm,
		m.searchQuery,
		m.searchMatchCount,
	)
}

func (m Model) messagePanel() string {
	if !m.nerdMode {
		return tuiview.CompactMessage(
			m.loading,
			m.err != nil,
			m.status,
			warningText(m.err),
			uiTheme,
		)
	}
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
	return tuiview.NerdMessage(status, warning, state, m.startupMetrics())
}

func warningText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
		"  j/k or arrows move, [ ] jump between sections, g/G jump top/bottom, pgup/pgdown jump page",
		"Tree-style List:",
		"  default list has Folders and Feeds sections",
		"  left/h collapses current feed/folder, right/l expands",
		"  Section legend: ▦/■ section, ▾/▸ expandable group, indented rows are feeds/articles",
		"Modes:",
		"  enter opens detail, esc/backspace returns to list",
		"Filters:",
		"  a all, u unread, * starred, / search, n load next page",
		"Actions:",
		"  U toggle unread, S toggle starred, o open URL, y copy URL, r/R/ctrl+r refresh",
		"Options:",
		"  c compact mode, N numbering, d time format, t mark-read-on-open, p confirm prompt, ctrl+l clear search, Shift+M confirm pending mark-read",
	}
	return strings.Join(lines, "\n")
}

func (m *Model) applyCurrentFilter() {
	if m.filter == "all" && m.searchQuery == "" {
		sortEntriesForTree(m.entries)
		m.ensureCursorVisible()
		return
	}
	searchQuery := strings.ToLower(strings.TrimSpace(m.searchQuery))
	filtered := make([]feedbin.Entry, 0, len(m.entries))
	for _, entry := range m.entries {
		if m.filter == "unread" && !entry.IsUnread {
			continue
		}
		if m.filter == "starred" && !entry.IsStarred {
			continue
		}
		if searchQuery != "" && !entryMatchesSearch(entry, searchQuery) {
			continue
		}
		filtered = append(filtered, entry)
	}
	m.entries = filtered
	sortEntriesForTree(m.entries)
	m.ensureCursorVisible()
}

func entryMatchesSearch(entry feedbin.Entry, query string) bool {
	if query == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		entry.Title,
		entry.Author,
		entry.Summary,
		entry.Content,
		entry.URL,
		entry.FeedTitle,
		entry.FeedFolder,
	}, " "))
	return strings.Contains(haystack, query)
}

func folderNameForEntry(entry feedbin.Entry) string {
	return tuitree.FolderName(entry)
}

func feedNameForEntry(entry feedbin.Entry) string {
	return tuitree.FeedName(entry)
}

func compactEntryLabel(entry feedbin.Entry) string {
	return tuiview.CompactEntryLabel(entry)
}

func sortEntriesForTree(entries []feedbin.Entry) {
	tuitree.SortEntries(entries)
}

func treeFeedKey(folder, feed string) string {
	return tuitree.FeedKey(folder, feed)
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
	now := time.Now()
	if m.nowFn != nil {
		now = m.nowFn()
	}
	return tuiview.RenderEntryLine(tuiview.EntryLineParams{
		Entry:        entry,
		Now:          now,
		RelativeTime: m.relativeTime,
		Compact:      m.compact,
		ShowNumbers:  m.showNumbers,
		VisiblePos:   visiblePos,
		Active:       active,
		Selected:     entry.ID == m.selectedID,
		Width:        m.contentWidth(),
	}, uiTheme)
}

func (m Model) renderTreeNodeLine(left string, unreadCount int, active bool) string {
	return tuiview.RenderTreeNodeLine(left, unreadCount, m.contentWidth(), active, uiTheme)
}

func (m Model) renderSectionLine(label string, unreadCount int, active bool) string {
	return tuiview.RenderSectionLine(label, unreadCount, m.contentWidth(), active, m.nerdIcons, uiTheme)
}

func (m Model) unreadCountsBySection() map[string]int {
	sectionCounts := map[string]int{
		"Folders": 0,
		"Feeds":   0,
	}
	for _, entry := range m.entries {
		if !entry.IsUnread {
			continue
		}
		if folderNameForEntry(entry) != "" {
			sectionCounts["Folders"]++
			continue
		}
		sectionCounts["Feeds"]++
	}
	return sectionCounts
}

func (m Model) unreadCountsByTreeNode() (map[string]int, map[string]int) {
	folderCounts := make(map[string]int)
	feedCounts := make(map[string]int)
	for _, entry := range m.entries {
		if !entry.IsUnread {
			continue
		}
		folder := folderNameForEntry(entry)
		feed := feedNameForEntry(entry)
		if folder != "" {
			folderCounts[folder]++
		}
		feedCounts[treeFeedKey(folder, feed)]++
	}
	return folderCounts, feedCounts
}

func relativeTimeLabel(now, then time.Time) string {
	return tuiview.RelativeTimeLabel(now, then)
}

func styleArticleTitle(entry feedbin.Entry, title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return title
	}
	return uiTheme.StyleArticleTitle(entry, title)
}

func visibleLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func stripANSI(s string) string {
	return reANSICodes.ReplaceAllString(s, "")
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

func (m *Model) jumpToSection(direction int) {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	if direction == 0 {
		return
	}
	if direction > 0 {
		for i := m.treeCursor + 1; i < len(rows); i++ {
			if rows[i].Kind == treeRowSection {
				m.treeCursor = i
				m.syncCursorFromTree()
				return
			}
		}
		return
	}
	for i := m.treeCursor - 1; i >= 0; i-- {
		if rows[i].Kind == treeRowSection {
			m.treeCursor = i
			m.syncCursorFromTree()
			return
		}
	}
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
	if row.Kind == treeRowSection {
		if m.collapsedSections[row.Label] {
			m.collapsedSections[row.Label] = false
			m.status = "Expanded section: " + row.Label
		} else {
			m.collapsedSections[row.Label] = true
			m.status = "Collapsed section: " + row.Label
		}
		m.ensureCursorVisible()
		return
	}
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
	if m.compact {
		return
	}
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	m.ensureTreeCursorValid()
	row := rows[m.treeCursor]
	if row.Kind == treeRowSection {
		if !m.collapsedSections[row.Label] {
			m.collapsedSections[row.Label] = true
			m.status = "Collapsed section: " + row.Label
		}
		return
	}
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
		m.setTreeCursorToFeed(folder, feed)
		m.ensureCursorVisible()
		return
	}
	if folder != "" && !m.collapsedFolders[folder] {
		m.collapsedFolders[folder] = true
		m.status = "Collapsed folder: " + folder
		m.setTreeCursorToFolder(folder)
		m.ensureCursorVisible()
	}
}

func (m *Model) setTreeCursorToFeed(folder, feed string) {
	rows := m.treeRows()
	for i, row := range rows {
		if row.Kind != treeRowFeed || row.Folder != folder {
			continue
		}
		if feed != "" && row.Feed != feed {
			continue
		}
		if feed == "" && row.Feed == "" {
			continue
		}
		if row.Feed == "" {
			continue
		}
		if row.Feed != "" {
			m.treeCursor = i
			return
		}
	}
}

func (m *Model) setTreeCursorToFolder(folder string) {
	rows := m.treeRows()
	for i, row := range rows {
		if row.Kind == treeRowFolder && row.Folder == folder {
			m.treeCursor = i
			return
		}
	}
}

func (m *Model) setTreeCursorToFirstArticle(folder, feed string) {
	rows := m.treeRows()
	for i, row := range rows {
		if row.Kind == treeRowArticle && row.Folder == folder && row.Feed == feed {
			m.treeCursor = i
			return
		}
	}
}

func (m *Model) expandCurrentTreeNode() {
	if m.compact {
		return
	}
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}

	m.ensureTreeCursorValid()
	row := rows[m.treeCursor]
	if row.Kind == treeRowSection {
		if m.collapsedSections[row.Label] {
			m.collapsedSections[row.Label] = false
			m.status = "Expanded section: " + row.Label
		}
		m.ensureCursorVisible()
		return
	}
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
		m.setTreeCursorToFeed(folder, "")
		m.ensureCursorVisible()
		return
	}
	if feed != "" && m.collapsedFeeds[feedKey] {
		m.collapsedFeeds[feedKey] = false
		m.status = "Expanded feed: " + feed
		m.setTreeCursorToFirstArticle(folder, feed)
		m.ensureCursorVisible()
		return
	}
	if row.Kind == treeRowFolder {
		m.setTreeCursorToFeed(folder, "")
		m.ensureCursorVisible()
		return
	}
	if row.Kind == treeRowFeed {
		m.setTreeCursorToFirstArticle(folder, feed)
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

type treeRow = tuitree.Row
type treeRowKind = tuitree.RowKind

const (
	treeRowSection treeRowKind = tuitree.RowSection
	treeRowFolder  treeRowKind = tuitree.RowFolder
	treeRowFeed    treeRowKind = tuitree.RowFeed
	treeRowArticle treeRowKind = tuitree.RowArticle
)

func (m Model) treeRows() []treeRow {
	return tuitree.BuildRows(m.entries, tuitree.BuildOptions{
		Compact:           m.compact,
		CollapsedFolders:  m.collapsedFolders,
		CollapsedFeeds:    m.collapsedFeeds,
		CollapsedSections: m.collapsedSections,
	})
}

func firstArticleRow(rows []treeRow) int {
	return tuitree.FirstArticleRow(rows)
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
	return tuitree.SplitFeedKey(key)
}

func (m Model) contentWidth() int {
	if m.width > 0 {
		return m.width - 1
	}
	return 100
}

func (m Model) detailHorizontalMargin() int {
	if m.width > 0 && m.width <= 60 {
		return 3
	}
	return 6
}

func (m Model) detailContentWidth() int {
	width := m.contentWidth() - (2 * m.detailHorizontalMargin())
	if width < 20 {
		return 20
	}
	return width
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

func (m Model) listBodyHeight() int {
	usedByHeader := 6
	if m.searchInputMode || m.searchQuery != "" {
		usedByHeader += 2
	}
	if m.height > 0 {
		if h := m.height - usedByHeader; h > 3 {
			return h
		}
		return 3
	}
	return 18
}

func (m Model) maxArticlesPerPage() int {
	h := m.listBodyHeight()
	if h < 1 {
		return 1
	}
	return h
}

func defaultPerPageFromEnv() int {
	linesValue := strings.TrimSpace(os.Getenv("LINES"))
	if linesValue == "" {
		return 20
	}

	lines, err := strconv.Atoi(linesValue)
	if err != nil || lines <= 0 {
		return 20
	}

	// Keep this aligned with listBodyHeight() defaults.
	const usedByHeader = 6
	if h := lines - usedByHeader; h > 3 {
		return h
	}
	return 3
}

func limitEntries(entries []feedbin.Entry, limit int) []feedbin.Entry {
	if limit <= 0 || len(entries) <= limit {
		return entries
	}
	return append([]feedbin.Entry(nil), entries[:limit]...)
}

func leftPadLines(lines []string, padding int) []string {
	if padding <= 0 || len(lines) == 0 {
		return lines
	}
	prefix := strings.Repeat(" ", padding)
	out := make([]string, len(lines))
	for i, line := range lines {
		if containsKittyGraphicsEscape(line) {
			out[i] = line
			continue
		}
		out[i] = prefix + line
	}
	return out
}

func centerLines(lines []string, width int) []string {
	if width <= 0 || len(lines) == 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		visible := visibleLen(line)
		if visible >= width {
			out[i] = line
			continue
		}
		pad := (width - visible) / 2
		if pad < 0 {
			pad = 0
		}
		out[i] = strings.Repeat(" ", pad) + line
	}
	return out
}

func (m Model) listWindow(rows []treeRow) (int, int, int) {
	if len(rows) == 0 {
		return 0, 0, 0
	}

	height := m.listBodyHeight()
	if height <= 0 || len(rows) <= height {
		return 0, len(rows), 0
	}

	start := m.treeCursor - height/2
	if start < 0 {
		start = 0
	}
	maxStart := len(rows) - height
	if start > maxStart {
		start = maxStart
	}
	end := start + height

	visiblePos := 0
	for i := 0; i < start; i++ {
		if rows[i].Kind == treeRowArticle {
			visiblePos++
		}
	}
	return start, end, visiblePos
}

func buildDetailLines(entry feedbin.Entry, width int, opts article.Options) []string {
	lines := tuiview.DetailMetaLines(entry, width, wrapText)
	contentLines := article.ContentLinesWithOptions(entry, width, opts)
	if len(contentLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, contentLines...)
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

func loadSearchCmd(service Service, filter, query string, limit int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, err := service.SearchCached(ctx, limit, filter, query)
		if err != nil {
			return searchLoadErrorMsg{err: err}
		}
		return searchLoadSuccessMsg{filter: filter, query: query, entries: entries}
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
	if !m.inlineImagePreview {
		return nil
	}
	if len(m.entries) == 0 {
		return nil
	}
	entry := m.entries[m.cursor]
	if strings.TrimSpace(entry.Content) == "" {
		return nil
	}
	imageURLs := article.ImageURLsFromContent(entry.Content)
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
	return inlineImagePreviewCmd(entry.ID, imageURLs[0], m.detailContentWidth(), m.renderImageFn)
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

	args := []string{
		"--size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
		"--view-size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
		"--align", "top,center",
		"--format", "symbols",
		"-",
	}
	if supportsKittyGraphics() {
		args = []string{
			"--size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
			"--view-size", fmt.Sprintf("%dx%d", width, inlineImagePreviewRows),
			"--align", "top,center",
			"--format", "kitty",
			"--passthrough", kittyPassthroughMode(),
			"--relative", "on",
			"-",
		}
	}
	cmd := exec.Command(chafaPath, args...)
	cmd.Stdin = bytes.NewReader(imageData)
	output, err := cmd.CombinedOutput()
	raw := string(output)
	trimmed := strings.TrimSpace(raw)

	if err != nil {
		return "", fmt.Errorf("render image via chafa: %w: %s", err, trimmed)
	}
	if supportsKittyGraphics() && containsKittyGraphicsEscape(raw) {
		return strings.TrimRight(raw, "\r\n"), nil
	}
	if trimmed == "" {
		return "", fmt.Errorf("empty output")
	}
	return trimmed, nil
}

func supportsKittyGraphics() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	termProgram := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if strings.Contains(termProgram, "ghostty") || strings.Contains(termProgram, "kitty") {
		return true
	}
	term := strings.ToLower(strings.TrimSpace(os.Getenv("TERM")))
	return strings.Contains(term, "xterm-kitty") || strings.Contains(term, "ghostty")
}

func containsKittyGraphicsEscape(s string) bool {
	return strings.Contains(s, "\x1b_G")
}

func kittyRenderedLineCount(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func clearKittyGraphicsSequence() string {
	base := "\x1b_Ga=d,d=A\x1b\\"
	if os.Getenv("TMUX") == "" {
		return base
	}
	escaped := strings.ReplaceAll(base, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;\x1b" + escaped + "\x1b\\"
}

func kittyPassthroughMode() string {
	if os.Getenv("TMUX") != "" {
		return "screen"
	}
	return "none"
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

func parseEnvBool(name string) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return false
	}
	ok, err := strconv.ParseBool(v)
	if err != nil {
		return v == "1"
	}
	return ok
}

func (m *Model) ApplyPreferences(prefs Preferences) {
	m.compact = prefs.Compact
	m.markReadOnOpen = prefs.MarkReadOnOpen
	m.confirmOpenRead = prefs.ConfirmOpenRead
	m.relativeTime = prefs.RelativeTime
	m.showNumbers = prefs.ShowNumbers
}

func (m *Model) SetPreferencesSaver(saveFn func(Preferences) error) {
	m.savePreferencesFn = saveFn
}

func (m *Model) SetArticleOptions(opts article.Options) {
	m.articleOptions = opts
}

func (m Model) preferences() Preferences {
	return Preferences{
		Compact:         m.compact,
		MarkReadOnOpen:  m.markReadOnOpen,
		ConfirmOpenRead: m.confirmOpenRead,
		RelativeTime:    m.relativeTime,
		ShowNumbers:     m.showNumbers,
	}
}

func (m *Model) SetStartupCacheStats(duration time.Duration, entries int) {
	m.cacheLoadDuration = duration
	m.cacheLoadedEntries = entries
}
