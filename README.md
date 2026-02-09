# feedbin-cli

Terminal client for reading Feedbin RSS entries.

## Current Status

This project already supports:

- Feedbin API authentication via HTTP Basic Auth
- Fetching latest entries from Feedbin
- Fetching subscriptions (feed metadata)
- Fetching unread and starred entry state
- Incremental sync of updated entries between page loads
- Caching entries and metadata locally in SQLite
- Displaying entries in a terminal UI
- Refresh action in TUI (`r`)

## Tech Stack

- Go
- Bubble Tea (TUI)
- SQLite (`modernc.org/sqlite`)
- Go `testing` + `httptest`

## Prerequisites

- `asdf` configured with Go 1.22.5 or compatible
- Feedbin account credentials

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

## TUI Controls

- `j` / `k` or arrows: move cursor
- `g` / `G`: jump to top / bottom
- `pgup` / `pgdown`: page navigation
- `enter`: open detail view
- `[` / `]`: previous / next entry (detail view)
- `esc` / `backspace`: back to list from detail
- `o`: open current entry URL (detail view)
- `a`: filter all
- `u`: filter unread
- `*`: filter starred
- `n`: load next page
- `m`: toggle unread/read
- `s`: toggle star/unstar
- `y`: copy current entry URL
- `c`: toggle compact list mode
- `t`: toggle mark-as-read when opening URL
- `p`: toggle confirmation prompt for mark-on-open
- `Shift+M`: confirm pending mark-as-read action
- `?`: show/hide in-app help
- `r`: refresh entries from Feedbin
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
  - Feedbin authentication credentials are valid
  - API reachability (warning + cache fallback on transient failures)
- If initial refresh fails at startup, the app attempts to load cached entries.
- Incremental sync cursor is persisted in SQLite app state and reused across restarts.
