# feedbin-cli

Terminal client for reading Feedbin RSS entries.

## Current Status

This project already supports:

- Feedbin API authentication via HTTP Basic Auth
- Fetching latest entries from Feedbin (including article HTML content when provided)
- Fetching subscriptions (feed metadata)
- Fetching unread and starred entry state
- Incremental sync of updated entries between page loads
- Caching entries and metadata locally in SQLite
- Persisting UI preferences in SQLite (`compact`, `time-format`, `mark-on-open`, `confirm-open-read`)
- Displaying entries in a terminal UI
- Neo-tree-inspired default list grouping by folder (URL host) and feed
- Active list-row highlight for the current cursor position
- Dedicated status/warnings/state panel near footer
- Startup metrics in message panel (cache load + initial refresh timing)
- Full-text-first detail rendering (falls back to summary)
- Image URL extraction in detail view
- Inline image previews in detail view (best effort via `chafa`)
- Refresh action in TUI (`r`)

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

## Run

```bash
export FEEDBIN_EMAIL="you@example.com"
export FEEDBIN_PASSWORD="your-password"
export FEEDBIN_DB_PATH="./feedbin.db" # optional

asdf exec go run ./cmd/feedbin
```

For faster startup in day-to-day usage, build once and run the binary:

```bash
asdf exec go build -o ./bin/feedbin ./cmd/feedbin
./bin/feedbin
```

## TUI Controls

- `j` / `k` or arrows: move cursor
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
- `U`: toggle unread/read
- `S`: toggle star/unstar
- `y`: copy current entry URL
- `c`: toggle compact list mode
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

Runs a real API flow (`refresh`, `toggle unread/starred`, `load more`) and is disabled by default.

```bash
FEEDBIN_INTEGRATION=1 asdf exec go test ./internal/app -run TestIntegration_RefreshToggleAndLoadMore -count=1
```

### TUI Workflow Tests

The unit suite now also covers open/confirm/debounce keyboard workflows and preference persistence behavior.

## Build

```bash
asdf exec go build ./cmd/feedbin
```

## Project Structure

- `cmd/feedbin`: CLI entrypoint
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
- Message panel reports startup timing:
  - cache load time and cached entry count
  - initial background refresh duration (or failure)
- Incremental sync cursor is persisted in SQLite app state and reused across restarts.
- UI preferences are loaded on startup and persisted whenever `c`, `d`, `t`, or `p` are toggled.
- Default list view is grouped as:
  - folder node: Feedbin folder/tag name (from `taggings`)
  - feed node: feed title
  - article rows under each feed
- Top collections are always visible:
  - folder collections are always rendered
  - feeds without Feedbin folder/tag are rendered as top-level collections
- Folder/feed rows show right-aligned unread counts in list view.
- Collections are part of navigation and receive the same active-row highlight as articles.
- Neo-tree-style collapsing:
  - `left`/`h` collapses the current feed, then folder.
  - `right`/`l` expands the current folder/feed, and can recover globally when all groups are collapsed.
- Inline image rendering behavior:
  - Uses first image in article HTML content when available.
  - Chooses `chafa` format automatically:
    - `kitty` when `KITTY_WINDOW_ID` is present.
    - `iterm` when `TERM_PROGRAM=iTerm.app`.
    - `symbols` fallback otherwise.
  - If `chafa` is not installed, detail view shows a non-fatal inline preview warning.
