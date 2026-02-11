# Reeder CLI - Session Handoff

## 1. Project Snapshot

`reeder-cli` is a terminal client (Go + Bubble Tea + SQLite) for reading Feedbin RSS entries.

Current status:
- Startup is cache-first and non-blocking (UI opens from cache, then refreshes in background).
- Startup loads up to 1000 cached entries by default, then refreshes in background.
- Full refresh now returns up to `DefaultCacheLimit` cached entries (1000) to the UI, avoiding unread-count drops caused by replacing the in-memory list with only the latest page.
- Feed metadata/unread/starred state are synced and persisted locally.
- List + detail views are implemented with keyboard-driven workflows.
- Read/star actions are wired to Feedbin API and local cache.
- Filters, pagination, URL open/copy, and help overlay exist.
- Local cached search is available in list mode and combines with current filter (`all`/`unread`/`starred`).
- Search status/footer now includes active query match count, and `ctrl+l` clears active search quickly.
- Search backend supports optional `FEEDBIN_SEARCH_MODE=fts` with automatic fallback to LIKE if FTS is unavailable.
- SQLite indexes now cover core listing/filter paths (`published_at`, unread/starred+published, `feed_id`, feed title/folder) for better large-cache performance.
- UI preferences (`compact`, `article-numbering`, `time-format`, `mark-read-on-open`, `confirm-open-read`) are persisted in SQLite and restored on startup.
- A fixed message panel (status/warning/state) is rendered above the footer in all modes.
- Message panel also shows startup metrics (cache load duration/count + initial refresh timing/failure).
- Active list-row highlight is rendered for the current cursor to improve navigation visibility.
- Toolbar and status/footer were revamped for readability: default mode now shows compact, cleaner controls/state while verbose diagnostics are hidden.
- TUI color styling now follows a Catppuccin-inspired palette across list/detail surfaces, section headers, active row highlighting, links, quotes, and status metadata.
- List mode uses a Neo-tree-inspired grouped layout with two top sections: `Folders` and `Feeds`.
- Section headers are visually emphasized and include unread counters.
- Optional Nerd Font icons can be enabled for section headers via `FEEDBIN_NERD_ICONS=1` (safe fallback symbols by default).
- CLI startup now supports `--nerd` to enable verbose/diagnostic UI mode (full keybinding toolbar + detailed status/footer including startup metrics and toggles).
- Neo-tree-style collapse/expand in list mode via `left/right` and `h/l`.
- Collapse navigation is hierarchical: collapsing from an article focuses its parent feed; collapsing again focuses the parent folder.
- Expand navigation is hierarchical: expanding from a folder/feed moves focus down to the first child row.
- Section rows are collapsible/expandable and participate in navigation.
- Expand behavior includes global recovery path so fully-collapsed trees can be reopened.
- Top collections remain visible even when collapsed (folder collections and top-level feeds).
- Tree navigation is row-based: folders, feeds, and articles are all focusable/highlightable.
- List ordering is stable and status-agnostic: top collections and feeds are sorted alphabetically, while articles stay newest-first by publish date.
- List article rows render a right-aligned publish time in listing mode (non-compact), with title truncation to preserve alignment.
- List rendering is now viewport-clipped to terminal height (including pre-size fallback), avoiding startup layout breakage when cache contains many entries.
- Time format is user-toggleable (`relative`/`absolute`) and persisted as a UI preference.
- Article numbering in list rows is user-toggleable and disabled by default.
- Folder/feed rows render right-aligned unread counts only when count is greater than zero.
- Feedbin folder mapping is sourced from `GET /v2/taggings.json` and persisted to `feeds.folder_name`.
- Full refresh hydrates unread/starred entry payloads (`entries?ids=`) so unread/starred filters include items beyond the first page fetch.
- Added regression test coverage to guarantee full refresh keeps using `DefaultCacheLimit` for returned cached list size.
- Detail view prefers full article content (`content`) and falls back to summary when content is absent.
- Detail view preserves article content order for text/images and renders in-flow textual image labels (circumflex-style) using image `alt`/`title` when available.
- Detail view hides raw image URL rows in article flow.
- Detail view now uses a DOM-based article parser that renders common semantic structure (titles/subtitles, links, lists, tables, blockquotes/citations, captions, definition lists, and code blocks) instead of plain tag-stripped text.
- Detail/article rendering now applies readability-focused terminal styling (heading emphasis, quote bars, dimmed citations/URLs, highlighted links/table headers) inspired by modern Bubble Tea reader patterns.
- Detail/article styling now also adopts circumflex-like readability cues: depth-aware unordered bullet glyphs (`•/◦/▪/▫`), dim/italic quote bodies, and stronger heading markers with colored bars.
- Detail rendering now includes a lightweight domain-aware postprocessor (circumflex-inspired) that can skip boilerplate paragraphs or stop before promo/reference sections for known publishers (currently: Wikipedia, NYTimes, WIRED, The Guardian, Ars Technica, Axios).
- Optional inline image preview rendering (`FEEDBIN_INLINE_IMAGE_PREVIEW=1`) uses `chafa` with Ghostty/Kitty-aware behavior tuned to avoid overlap artifacts in TUI redraws.
- Integration tests against live Feedbin are available (opt-in).
- GitHub Actions CI now runs unit tests and vet on push/PR.

## 2. Core Stack and Rationale

- Language: Go
  - fast CLI startup, simple deployment, strong stdlib for HTTP/testing
- TUI: Bubble Tea
  - state-machine approach works well for keyboard-first terminal UX
- Storage: SQLite (`modernc.org/sqlite`)
  - local persistent cache, structured filtering, sync cursor persistence
- Testing:
  - unit tests: Go `testing` + `httptest`
  - opt-in integration tests against live Feedbin

## 3. High-Level Architecture

- `cmd/reeder/main.go`
  - app startup, config load, storage health checks, cache-first boot, background refresh via TUI init
- `internal/config`
  - env-based config loading/validation
- `internal/feedbin`
  - Feedbin API client (read + write endpoints)
- `internal/storage`
  - SQLite repository for entries/feeds/state/app sync cursor
- `internal/app`
  - service orchestration: refresh, incremental sync, load-more, toggles
- `internal/tui`
  - Bubble Tea model: list/detail/help views, keybindings, state transitions

## 4. Key Product/Technical Decisions

1. Local-first rendering with SQLite cache
- Reason: responsive UI and resilience when API is unavailable.

2. Explicit startup/storage health checks
- DB writable check before launch.
- Removed standalone auth preflight to avoid extra startup round-trip.
- Startup reads cache immediately and defers network refresh to background.

3. Incremental sync strategy
- Initial/full refresh syncs subscriptions + unread/starred sets.
- Subsequent load-more calls do incremental updated-entry sync using:
  - `GET /updated_entries.json?since=...`
  - `GET /entries.json?ids=...`
- Sync cursor (`updated_entries_since`) is persisted in SQLite `app_state`.

4. Faster startup strategy
- cache-first boot from SQLite
- background refresh triggered by TUI init
- initial per-page reduced to 20
- full-state metadata calls (`subscriptions`, `unread`, `starred`) fetched in parallel
5. Keyboard-first UX in TUI
- No mouse assumptions.
- Detail and list workflows are fully keyboard-driven.

6. Safe open behavior
- URL validation before open/copy (requires `http`/`https`).
- `o` opens URL (browser fallback to clipboard copy).
- `y` copies URL directly.

## 5. What Is Implemented (Feature Checklist)

### Feed + Sync
- [x] Basic auth against Feedbin
- [x] Fetch entries page-wise
- [x] Fetch subscriptions
- [x] Fetch unread IDs and starred IDs
- [x] Write actions:
  - mark unread/read
  - star/unstar
- [x] Incremental updated-entry sync using persisted `since` cursor

### Storage
- [x] Entries table with unread/starred flags
- [x] Feeds table for metadata
- [x] App state table for sync cursor
- [x] App state keys for UI preferences
- [x] Entry content persisted in SQLite (`entries.content`)
- [x] Filtered queries (`all` / `unread` / `starred`)
- [x] Local text search query over cached entries (title/author/summary/content/url/feed/folder), with optional unread/starred filter
- [x] Optional FTS5 search mode (`FEEDBIN_SEARCH_MODE=fts`) with fallback to LIKE

### TUI
- [x] List view + detail view
- [x] Default grouped list layout by folder and feed
- [x] Two top-level sections in list view: `Folders` and `Feeds`
- [x] Entry detail wrapping + scroll
- [x] Footer state (`mode/filter/page/showing/last-fetch/open->read/confirm`)
- [x] Help panel (`?`)
- [x] List navigation:
  - `j/k`, arrows
  - `[` / `]` jump previous/next section
  - `g/G`
  - `pgup/pgdown`
- [x] Detail navigation:
  - `[` / `]` previous/next entry
- [x] Filters:
  - `a`, `u`, `*`
- [x] Pagination:
  - `n` load next page
  - no-more handling when fetch count is zero
- [x] Actions:
  - `U` toggle unread/read
  - `S` toggle star/unstar
  - `o` open URL
  - `y` copy URL
- [x] Options:
  - `c` compact mode
  - `N` toggle article numbering
  - `d` toggle relative/absolute list time
  - `t` mark-read-on-open
  - `p` require confirm for mark-on-open
  - `Shift+M` confirm pending mark-read
- [x] Debounce for repeated open->mark-read
- [x] Fixed message panel for status/warning/loading state
- [x] Startup timing metrics in message panel
- [x] Full-text-first detail rendering with HTML-to-text conversion
- [x] Semantic HTML detail rendering for common article elements (headings/lists/tables/links/citations)
- [x] Ordered image-aware detail flow (text/images in article order, no raw image URL rows)
- [x] Circumflex-style textual image labels in detail view (with optional `chafa` inline previews via env flag)

## 6. Important Runtime Behaviors

### Startup (`cmd/reeder/main.go`)
1. Load env config.
2. Init SQLite and schema.
3. Run writable health check.
4. Load cached entries from SQLite immediately.
5. Start TUI.
6. TUI `Init()` triggers background refresh (`page=1`, `per_page=20`).

### Sync Cursor Persistence
- Key: `updated_entries_since`
- Table: `app_state`
- On service startup/load-more, cursor is loaded from DB if in-memory cursor is empty.
- Cursor is updated and persisted after full or incremental sync.

## 7. Testing and Validation

### Unit Tests (default)
```bash
asdf exec go test ./...
```

### Live Integration Test (opt-in)
```bash
FEEDBIN_INTEGRATION=1 asdf exec go test ./internal/app -run TestIntegration_RefreshToggleAndLoadMore -count=1
FEEDBIN_INTEGRATION=1 asdf exec go test ./internal/app -run TestIntegration_SearchCachedWithFilterAndClear -count=1
```

What integration currently verifies:
- refresh from live API
- toggle unread
- toggle starred
- load more
- unread filter cache consistency
- search + unread filter behavior after refresh/load-more
- empty-query search returns the same result set as filtered cache (service-layer clear-search behavior)

Additional workflow coverage in unit tests:
- confirm mode for open->mark-read (`o` + `Shift+M`)
- debounce behavior to prevent repeated auto mark-read
- preference persistence command path on `c/N/d/t/p`
- storage benchmarks for search backends (`BenchmarkRepositorySearchLike`, `BenchmarkRepositorySearchFTS`)

## 8. Current Keybindings (Quick Reference)

List mode:
- `j/k` or arrows: move
- `[` / `]`: jump previous/next section
- `g/G`: top/bottom
- `pgup/pgdown`: page jump
- `left/h`: collapse current section/feed/folder
- `right/l`: expand current section/folder/feed
- `enter`: open detail
- `a/u/*`: filter all/unread/starred
- `/`: search cached entries (press `enter` to apply; empty query clears)
- `ctrl+l`: clear active search
- `n`: load next page
- `U/S`: toggle unread/starred
- `y`: copy URL
- `c`: compact mode toggle
- `N`: toggle article numbering
- `d`: toggle list time format (relative/absolute)
- `t`: mark-read-on-open toggle
- `p`: confirm mode toggle for mark-on-open
- `r`: refresh
- `R` or `ctrl+r`: refresh
- `?`: help
- `q` or `ctrl+c`: quit

Detail mode:
- `j/k`: scroll
- `[` / `]`: previous/next entry
- `o`: open URL
- `y`: copy URL
- `U/S`: toggle unread/starred
- `esc/backspace`: back to list
- `Shift+M`: confirm pending mark-read (when confirmation enabled)
- `?`: help
- `q` or `ctrl+c`: quit

## 9. Recent Commits (Most Relevant)

- `HEAD` Add search-path indexes, search benchmarks, and live search integration test
- `69c284b` Add search UX polish, search-flow tests, and optional FTS mode
- `5beec86` Add local cached search in TUI and storage
- `768cf71` Add GitHub Actions CI for test and vet
- `efa8782` Keep refresh list at cache limit and add regression test
- `b3dc546` Add inline image previews in detail view
- `e43d2d2` Add full-text detail rendering and image URL extraction
- `1899f07` Highlight active list row in TUI
- `43a3c6d` Document persisted preferences and message panel
- `0bcdf16` Add fixed message panel and preference save hooks
- `c4fc404` Persist UI preferences in app state
- `1b784db` Add comprehensive session handoff and coding practices

## 10. Known Gaps / Follow-ups

1. Potential future polish:
- add optional auto-clear timing config for message panel
- persist additional UI state (`filter`, `cursor`/last-selected id) across restarts
- add richer inline image strategy (multi-image pagination, configurable preview size)
- make image preview timeout/size configurable per user preference

## 11. How To Continue Next Session (Suggested Order)

1. Add stronger TUI golden tests for help/detail/list/message panel rendering.
2. Expand live integration suite only where API-backed behavior is critical.
3. Add optional message auto-clear timing config.

## 12. Environment Notes

Required env vars:
- `FEEDBIN_EMAIL`
- `FEEDBIN_PASSWORD`

Optional:
- `FEEDBIN_API_BASE_URL` (default: `https://api.feedbin.com/v2`)
- `FEEDBIN_DB_PATH` (default: `feedbin.db`)
- `FEEDBIN_SEARCH_MODE` (`like` default, `fts` optional)
- `FEEDBIN_INLINE_IMAGE_PREVIEW` (`0` default for circumflex-style image labels; set `1` to enable `chafa` inline previews)
- `FEEDBIN_INTEGRATION=1` to enable live integration tests

## 13. Coding Best Practices (Project-Specific)

### Test Discipline

1. Run unit tests after every meaningful change set:
```bash
asdf exec go test ./...
```

2. When touching sync/auth/app orchestration, also run live integration tests:
```bash
FEEDBIN_INTEGRATION=1 asdf exec go test ./internal/app -run TestIntegration_RefreshToggleAndLoadMore -count=1
```

3. Treat failing tests as blockers.
- Do not ship code with broken unit tests.
- Prefer fixing tests and implementation in the same focused change.

### Integration Test Safety

1. Integration tests are opt-in by design.
- Never assume they are enabled in CI/local by default.
- Always gate with `FEEDBIN_INTEGRATION=1`.

2. Keep tests idempotent and restorative.
- If a test toggles unread/starred state, restore original state before exit when possible.

3. Keep integration tests narrow.
- Validate behavior-critical API flows only; avoid broad, flaky end-to-end scripts.

### Documentation Hygiene

1. Update docs in the same PR/commit set when behavior changes:
- `README.md` for user-visible controls, setup, commands.
- `SESSION_HANDOFF.md` for architecture/flow/decision updates and next-session context.

2. Continuously update `SESSION_HANDOFF.md` during development.
- Assume sessions can stop at any moment.
- After each meaningful project update, add/adjust handoff notes immediately (not only at the end).
- The file should always reflect current reality for the next AI session.

3. Do not leave stale keybindings or commands in docs.
- If TUI keys change, update key sections immediately.

4. Keep docs operational.
- Every command in docs should be runnable as written.

### Commit Quality

1. Keep commits atomic and concise.
- One commit per logical change unit (e.g., storage, app orchestration, TUI behavior, docs).

2. Prefer commit sequences like:
- implementation + tests together
- docs update as a separate final commit (if large), or same commit for small behavior changes

3. Recommended workflow pattern:
- edit -> `gofmt` -> `go test` -> commit
- repeat for each logical unit

4. Commit message style:
- imperative, specific, short
- include a brief solution detail (what changed and why) in the commit body when useful
- examples:
  - `Persist incremental sync cursor in SQLite`
  - `Add help panel and detail navigation shortcuts`

### Review/Session Handoff Expectations

1. Before ending a session:
- run `asdf exec go test ./...`
- run integration test if relevant changes touched app sync/actions
- ensure `git status` is clean (or explicitly list uncommitted files)

2. Summarize:
- what changed
- why it changed
- what was validated
- what remains (if anything)

---

This file is intended as the primary AI handoff context for subsequent coding sessions.
