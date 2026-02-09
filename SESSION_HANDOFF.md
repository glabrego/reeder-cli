# Feedbin CLI - Session Handoff

## 1. Project Snapshot

`feedbin-cli` is a terminal client (Go + Bubble Tea + SQLite) for reading Feedbin RSS entries.

Current status:
- Auth + startup health checks are implemented.
- Feed metadata/unread/starred state are synced and persisted locally.
- List + detail views are implemented with keyboard-driven workflows.
- Read/star actions are wired to Feedbin API and local cache.
- Filters, pagination, URL open/copy, and help overlay exist.
- Integration tests against live Feedbin are available (opt-in).

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

- `cmd/feedbin/main.go`
  - app startup, config load, health checks, initial refresh/cache fallback, launches TUI
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

2. Explicit startup health checks
- DB writable check before launch.
- Feedbin auth check with actionable error messages.
- API reachability warnings degrade gracefully to cache.

3. Incremental sync strategy
- Initial/full refresh syncs subscriptions + unread/starred sets.
- Subsequent load-more calls do incremental updated-entry sync using:
  - `GET /updated_entries.json?since=...`
  - `GET /entries.json?ids=...`
- Sync cursor (`updated_entries_since`) is persisted in SQLite `app_state`.

4. Keyboard-first UX in TUI
- No mouse assumptions.
- Detail and list workflows are fully keyboard-driven.

5. Safe open behavior
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
- [x] Filtered queries (`all` / `unread` / `starred`)

### TUI
- [x] List view + detail view
- [x] Entry detail wrapping + scroll
- [x] Footer state (`mode/filter/page/showing/last-fetch/open->read/confirm`)
- [x] Help panel (`?`)
- [x] List navigation:
  - `j/k`, arrows
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
  - `m` toggle unread/read
  - `s` toggle star/unstar
  - `o` open URL
  - `y` copy URL
- [x] Options:
  - `c` compact mode
  - `t` mark-read-on-open
  - `p` require confirm for mark-on-open
  - `Shift+M` confirm pending mark-read
- [x] Debounce for repeated open->mark-read

## 6. Important Runtime Behaviors

### Startup (`cmd/feedbin/main.go`)
1. Load env config.
2. Init SQLite and schema.
3. Run writable health check.
4. Run Feedbin auth check.
5. Refresh entries; if refresh fails, fallback to cached entries.
6. Start TUI.

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
```

What integration currently verifies:
- refresh from live API
- toggle unread
- toggle starred
- load more
- unread filter cache consistency

## 8. Current Keybindings (Quick Reference)

List mode:
- `j/k` or arrows: move
- `g/G`: top/bottom
- `pgup/pgdown`: page jump
- `enter`: open detail
- `a/u/*`: filter all/unread/starred
- `n`: load next page
- `m/s`: toggle unread/starred
- `y`: copy URL
- `c`: compact mode toggle
- `t`: mark-read-on-open toggle
- `p`: confirm mode toggle for mark-on-open
- `r`: refresh
- `?`: help
- `q` or `ctrl+c`: quit

Detail mode:
- `j/k`: scroll
- `[` / `]`: previous/next entry
- `o`: open URL
- `y`: copy URL
- `m/s`: toggle unread/starred
- `esc/backspace`: back to list
- `Shift+M`: confirm pending mark-read (when confirmation enabled)
- `?`: help
- `q` or `ctrl+c`: quit

## 9. Recent Commits (Most Relevant)

- `f05b526` Document help and mark-on-open confirmation keys
- `fc6e90a` Add help panel and detail navigation shortcuts
- `05090f0` Persist incremental sync cursor in SQLite
- `9cd4739` Document incremental sync and new controls
- `478768f` Enhance list navigation and open workflow options
- `105823b` Use incremental updated-entry sync between pages
- `5cd08b4` Add startup storage and auth health checks

## 10. Known Gaps / Follow-ups

1. CI pipeline is still missing.
- Add GitHub Actions for `go test ./...` and `go vet ./...`.

2. UI transient status capture in automated smoke tests.
- Behavior is tested in unit tests, but terminal snapshot tooling may miss short-lived status lines.

3. Potential future polish:
- add visual section for warnings/errors/status in fixed footer area
- persist user UI preferences (`compact`, `markReadOnOpen`, etc.) in DB
- add optional auto-clear timing config

## 11. How To Continue Next Session (Suggested Order)

1. Add CI workflow (`test + vet`).
2. Persist UI preferences in `app_state` (compact/confirm/mark-on-open).
3. Improve integration suite with dedicated tests for confirmation + debounce semantics.
4. Add stronger TUI golden tests for help/detail/list layout rendering.

## 12. Environment Notes

Required env vars:
- `FEEDBIN_EMAIL`
- `FEEDBIN_PASSWORD`

Optional:
- `FEEDBIN_API_BASE_URL` (default: `https://api.feedbin.com/v2`)
- `FEEDBIN_DB_PATH` (default: `feedbin.db`)
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
