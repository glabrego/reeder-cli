# reeder-cli

Terminal client for reading Feedbin RSS entries.

## Current Status

This project already supports:

- Feedbin API authentication via HTTP Basic Auth
- Fetching latest entries from Feedbin (including article HTML content when provided)
- Fetching subscriptions (feed metadata)
- Fetching unread and starred entry state
- Incremental sync of updated entries between page loads
- Caching entries and metadata locally in SQLite
- Persisting UI preferences in SQLite (`compact`, `article-numbering`, `time-format`, `mark-on-open`, `confirm-open-read`)
- Displaying entries in a terminal UI
- Neo-tree-inspired default list grouping with top-level sections: `Folders` and `Feeds`
- Active list-row highlight for the current cursor position
- Dedicated status/warnings/state panel near footer
- Startup metrics in message panel (cache load + initial refresh timing)
- Full-text-first detail rendering (falls back to summary)
- Image URL extraction in detail view
- Inline image previews in detail view (best effort via `chafa`)
- Refresh action in TUI (`r`)
- Local full-text search over cached entries (`/`)

## Tech Stack

- Go
- Bubble Tea (TUI)
- SQLite (`modernc.org/sqlite`)
- Go `testing` + `httptest`

## Prerequisites

- `asdf` configured with Go 1.22.5 or compatible
- Feedbin account credentials
- Optional for inline image previews: `chafa` installed and available in `PATH`

## Setup

```bash
asdf set golang 1.22.5
asdf exec go mod tidy
```

## Environment Variables

Required:

- `FEEDBIN_EMAIL`: your Feedbin login email
- `FEEDBIN_PASSWORD`: your Feedbin login password

Optional:

- `FEEDBIN_API_BASE_URL` (default: `https://api.feedbin.com/v2`)
- `FEEDBIN_DB_PATH` (default: `feedbin.db`)
- `FEEDBIN_SEARCH_MODE` (`like` by default, `fts` to prefer SQLite FTS5 with automatic fallback)
- `FEEDBIN_ARTICLE_STYLE_LINKS` (default: `true`; style rendered links in detail view)
- `FEEDBIN_ARTICLE_POSTPROCESS` (default: `true`; apply site-specific cleanup to article content)
- `FEEDBIN_ARTICLE_IMAGE_MODE` (default: `label`; valid: `label`, `none`)

## Run

```bash
export FEEDBIN_EMAIL="you@example.com"
export FEEDBIN_PASSWORD="your-password"
export FEEDBIN_DB_PATH="./feedbin.db" # optional

asdf exec go run ./cmd/reeder
```

CLI flags (override environment defaults):

- `--nerd` (show verbose toolbar/footer diagnostics)
- `--article-style-links=true|false`
- `--article-postprocess=true|false`
- `--article-image-mode=label|none`

Example:

```bash
asdf exec go run ./cmd/reeder --nerd --article-image-mode=none --article-style-links=false
```

For faster startup in day-to-day usage, build once and run the binary:

```bash
asdf exec go build -o ./bin/reeder ./cmd/reeder
./bin/reeder
```

## TUI Controls

- `j` / `k` or arrows: move cursor
- `[` / `]` (list mode): jump to previous / next top-level section
- `g` / `G`: jump to top / bottom
- `pgup` / `pgdown`: page navigation
- `left` / `h`: collapse current feed, then folder
- `right` / `l`: expand current folder/feed
- `enter`: open detail view when on an article; toggle collapse/expand when on a collection row
- `[` / `]`: previous / next entry (detail view)
- `esc` / `backspace`: back to list from detail
- `o`: open current entry URL (detail view)
- `a`: filter all
- `u`: filter unread
- `*`: filter starred
- `n`: load next page
- `/`: search cached entries (press `enter` to apply, empty query clears)
- `ctrl+l`: clear active search quickly
- `U`: toggle unread/read
- `S`: toggle star/unstar
- `y`: copy current entry URL
- `c`: toggle compact list mode
- `N`: toggle article numbering in list rows
- `d`: toggle list time format (relative/absolute)
- `t`: toggle mark-as-read when opening URL
- `p`: toggle confirmation prompt for mark-on-open
- `Shift+M`: confirm pending mark-as-read action
- `?`: show/hide in-app help
- `r` / `R` / `ctrl+r`: refresh entries from Feedbin
- `q`: quit
- `ctrl+c`: quit

## Test

```bash
asdf exec go test ./...
```

### Integration Test (Live Feedbin)

Runs real API flows and is disabled by default.

```bash
FEEDBIN_INTEGRATION=1 asdf exec go test ./internal/app -run TestIntegration_RefreshToggleAndLoadMore -count=1
FEEDBIN_INTEGRATION=1 asdf exec go test ./internal/app -run TestIntegration_SearchCachedWithFilterAndClear -count=1
```

### Search Benchmarks

Compare local search backends (`like` vs `fts`) with synthetic cached data:

```bash
asdf exec go test ./internal/storage -run '^$' -bench 'BenchmarkRepositorySearch(Like|FTS)$' -benchmem
```

### Rendering Benchmarks

Track parser/tree performance as rendering complexity grows:

```bash
asdf exec go test ./internal/render/article ./internal/tui/tree -run '^$' -bench 'Benchmark(ContentLinesWithOptions_ComplexArticle|BuildRows_)' -benchmem
```

Current baseline (Apple M4, Go 1.22.5):

- `BenchmarkContentLinesWithOptions_ComplexArticle`: ~`74.2µs/op`, `127317 B/op`, `1771 allocs/op`
- `BenchmarkBuildRows_DefaultTree`: ~`101.9µs/op`, `166712 B/op`, `1826 allocs/op`
- `BenchmarkBuildRows_Compact`: ~`30.2µs/op`, `99903 B/op`, `4 allocs/op`

Performance budget guideline:

- Treat regressions above ~20% in `ns/op` or `allocs/op` as review blockers unless there is a clear quality/feature reason.

### TUI Workflow Tests

The unit suite now also covers open/confirm/debounce keyboard workflows and preference persistence behavior.

## Build

```bash
asdf exec go build ./cmd/reeder
```

## Project Structure

- `cmd/reeder`: CLI entrypoint
- `internal/config`: configuration loading/validation
- `internal/feedbin`: Feedbin API client
- `internal/storage`: SQLite repository/cache
- `internal/app`: application service/use-cases
- `internal/tui`: Bubble Tea model/view

## Notes

- Feedbin API v2 uses HTTP Basic Auth.
- Startup checks validate:
  - local SQLite path is writable
  - local cache is immediately loaded in UI
  - background refresh starts automatically after UI boot
- Startup no longer blocks on a separate auth preflight call.
- Full-state sync now fetches subscriptions/unread/starred in parallel.
- Full-state sync also hydrates unread/starred entry payloads by ID, so filtered unread/starred views include items not present in the initial page fetch.
- Default startup page size is 20 entries (faster first network refresh).
- Startup loads up to 1000 cached entries by default before background refresh.
- Message panel reports startup timing:
  - cache load time and cached entry count
  - initial background refresh duration (or failure)
- Incremental sync cursor is persisted in SQLite app state and reused across restarts.
- UI preferences are loaded on startup and persisted whenever `c`, `N`, `d`, `t`, or `p` are toggled.
- Search behavior:
  - `/` opens search input mode.
  - Search runs locally against cached data (title/author/summary/content/url/feed/folder).
  - Search combines with current filter (`all`, `unread`, `starred`).
  - Search status/footer show active query and match count.
  - `FEEDBIN_SEARCH_MODE=fts` enables FTS5-backed search when available (falls back to `LIKE` if unsupported).
- Default list view is grouped as:
  - top section: `Folders`
  - folder node: Feedbin folder/tag name (from `taggings`)
  - feed node: feed title
  - top section: `Feeds` (feeds without folder)
  - article rows under each feed
- Section headers are visually emphasized and now also show unread counters (when count > 0).
- Top collections are always visible:
  - folder collections are always rendered
  - feeds without Feedbin folder/tag are rendered as top-level collections
- Folder/feed rows show right-aligned unread counts in list view (only when count > 0).
- Article numbering is disabled by default; use `N` to enable it.
- Collections are part of navigation and receive the same active-row highlight as articles.
- Neo-tree-style collapsing:
  - `left`/`h` collapses the current section/feed/folder.
  - `right`/`l` expands the current section/folder/feed, and can recover globally when all groups are collapsed.
- Optional Nerd Font icons:
  - set `FEEDBIN_NERD_ICONS=1` to render section icons using Nerd Font glyphs.
  - defaults to built-in symbols when unset.
- Inline image rendering behavior:
  - Uses first image in article HTML content when available.
  - Delegates terminal capability detection to `chafa` itself (default auto-probing).
  - If `chafa` is not installed, detail view shows a non-fatal inline preview warning.
