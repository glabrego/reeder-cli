package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/glabrego/feedbin-cli/internal/feedbin"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) Init(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS feeds (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  feed_url TEXT,
  site_url TEXT,
  folder_name TEXT,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entries (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  author TEXT,
  summary TEXT,
  content TEXT,
  feed_id INTEGER NOT NULL,
  published_at TEXT NOT NULL,
  fetched_at TEXT NOT NULL,
  is_unread INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(feed_id) REFERENCES feeds(id)
);

CREATE TABLE IF NOT EXISTS app_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	if err := r.addColumnIfMissing(ctx, "entries", "is_unread", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := r.addColumnIfMissing(ctx, "entries", "is_starred", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := r.addColumnIfMissing(ctx, "entries", "content", "TEXT"); err != nil {
		return err
	}
	if err := r.addColumnIfMissing(ctx, "feeds", "folder_name", "TEXT"); err != nil {
		return err
	}

	return nil
}

func (r *Repository) addColumnIfMissing(ctx context.Context, table, column, declaration string) error {
	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, declaration)
	_, err := r.db.ExecContext(ctx, query)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return nil
	}
	return fmt.Errorf("ensure column %s.%s: %w", table, column, err)
}

func (r *Repository) SaveSubscriptions(ctx context.Context, subscriptions []feedbin.Subscription) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO feeds (id, title, feed_url, site_url, folder_name, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title=excluded.title,
  feed_url=excluded.feed_url,
  site_url=excluded.site_url,
  folder_name=excluded.folder_name,
  updated_at=excluded.updated_at
`)
	if err != nil {
		return fmt.Errorf("prepare save feeds statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, sub := range subscriptions {
		_, err := stmt.ExecContext(ctx, sub.ID, sub.Title, sub.FeedURL, sub.SiteURL, sub.Folder, now)
		if err != nil {
			return fmt.Errorf("save subscription %d: %w", sub.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *Repository) SaveEntries(ctx context.Context, entries []feedbin.Entry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO entries (id, title, url, author, summary, content, feed_id, published_at, fetched_at, is_unread, is_starred)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title=excluded.title,
  url=excluded.url,
  author=excluded.author,
  summary=excluded.summary,
  content=excluded.content,
  feed_id=excluded.feed_id,
  published_at=excluded.published_at,
  fetched_at=excluded.fetched_at,
  is_unread=excluded.is_unread,
  is_starred=excluded.is_starred
`)
	if err != nil {
		return fmt.Errorf("prepare save statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, entry := range entries {
		_, err := stmt.ExecContext(
			ctx,
			entry.ID,
			entry.Title,
			entry.URL,
			entry.Author,
			entry.Summary,
			entry.Content,
			entry.FeedID,
			entry.PublishedAt.UTC().Format(time.RFC3339Nano),
			now,
			boolToInt(entry.IsUnread),
			boolToInt(entry.IsStarred),
		)
		if err != nil {
			return fmt.Errorf("save entry %d: %w", entry.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *Repository) SaveEntryStates(ctx context.Context, unreadIDs, starredIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE entries SET is_unread = 0, is_starred = 0`); err != nil {
		return fmt.Errorf("reset entry states: %w", err)
	}

	for _, id := range unreadIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE entries SET is_unread = 1 WHERE id = ?`, id); err != nil {
			return fmt.Errorf("mark entry unread %d: %w", id, err)
		}
	}

	for _, id := range starredIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE entries SET is_starred = 1 WHERE id = ?`, id); err != nil {
			return fmt.Errorf("mark entry starred %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *Repository) SetEntryUnread(ctx context.Context, entryID int64, unread bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entries SET is_unread = ? WHERE id = ?`, boolToInt(unread), entryID)
	if err != nil {
		return fmt.Errorf("set entry unread state for %d: %w", entryID, err)
	}
	return nil
}

func (r *Repository) SetEntryStarred(ctx context.Context, entryID int64, starred bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE entries SET is_starred = ? WHERE id = ?`, boolToInt(starred), entryID)
	if err != nil {
		return fmt.Errorf("set entry starred state for %d: %w", entryID, err)
	}
	return nil
}

func (r *Repository) CheckWritable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS healthcheck (id INTEGER PRIMARY KEY, touched_at TEXT NOT NULL)`)
	if err != nil {
		return fmt.Errorf("create healthcheck table: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO healthcheck (touched_at) VALUES (?)`, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert healthcheck row: %w", err)
	}
	return nil
}

func (r *Repository) GetSyncCursor(ctx context.Context, key string) (time.Time, error) {
	value, err := r.GetAppState(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("load sync cursor %q: %w", key, err)
	}

	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse sync cursor %q value %q: %w", key, value, err)
	}
	return ts, nil
}

func (r *Repository) SetSyncCursor(ctx context.Context, key string, value time.Time) error {
	if err := r.SetAppState(ctx, key, value.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("save sync cursor %q: %w", key, err)
	}
	return nil
}

func (r *Repository) GetAppState(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", sql.ErrNoRows
		}
		return "", fmt.Errorf("load app_state key %q: %w", key, err)
	}
	return value, nil
}

func (r *Repository) SetAppState(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO app_state (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
  value=excluded.value,
  updated_at=excluded.updated_at
`, key, value, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save app_state key %q: %w", key, err)
	}
	return nil
}

func (r *Repository) ListEntries(ctx context.Context, limit int) ([]feedbin.Entry, error) {
	return r.ListEntriesByFilter(ctx, limit, "all")
}

func (r *Repository) ListEntriesByFilter(ctx context.Context, limit int, filter string) ([]feedbin.Entry, error) {
	if limit < 1 {
		limit = 1000
	}

	whereClause := ""
	switch filter {
	case "unread":
		whereClause = "WHERE e.is_unread = 1"
	case "starred":
		whereClause = "WHERE e.is_starred = 1"
	}

	query := fmt.Sprintf(`
SELECT e.id, e.title, e.url, e.author, e.summary, COALESCE(e.content, ''), e.feed_id, e.published_at, e.is_unread, e.is_starred, COALESCE(f.title, ''), COALESCE(f.folder_name, '')
FROM entries e
LEFT JOIN feeds f ON f.id = e.feed_id
%s
ORDER BY e.published_at DESC
LIMIT ?
`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	entries := make([]feedbin.Entry, 0, limit)
	for rows.Next() {
		var entry feedbin.Entry
		var publishedAt string
		var isUnread int
		var isStarred int
		if err := rows.Scan(
			&entry.ID,
			&entry.Title,
			&entry.URL,
			&entry.Author,
			&entry.Summary,
			&entry.Content,
			&entry.FeedID,
			&publishedAt,
			&isUnread,
			&isStarred,
			&entry.FeedTitle,
			&entry.FeedFolder,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}

		entry.PublishedAt, err = time.Parse(time.RFC3339Nano, publishedAt)
		if err != nil {
			return nil, fmt.Errorf("parse entry published_at %q: %w", publishedAt, err)
		}
		entry.IsUnread = intToBool(isUnread)
		entry.IsStarred = intToBool(isStarred)
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return entries, nil
}

func (r *Repository) SearchEntriesByFilter(ctx context.Context, limit int, filter, query string) ([]feedbin.Entry, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return r.ListEntriesByFilter(ctx, limit, filter)
	}
	if limit < 1 {
		limit = 1000
	}

	pattern := "%" + strings.ToLower(trimmedQuery) + "%"
	whereParts := make([]string, 0, 2)
	args := make([]any, 0, 3)

	switch filter {
	case "unread":
		whereParts = append(whereParts, "e.is_unread = 1")
	case "starred":
		whereParts = append(whereParts, "e.is_starred = 1")
	}
	whereParts = append(whereParts, `(LOWER(e.title) LIKE ? OR LOWER(COALESCE(e.author, '')) LIKE ? OR LOWER(COALESCE(e.summary, '')) LIKE ? OR LOWER(COALESCE(e.content, '')) LIKE ? OR LOWER(e.url) LIKE ? OR LOWER(COALESCE(f.title, '')) LIKE ? OR LOWER(COALESCE(f.folder_name, '')) LIKE ?)`)
	for i := 0; i < 7; i++ {
		args = append(args, pattern)
	}

	querySQL := fmt.Sprintf(`
SELECT e.id, e.title, e.url, e.author, e.summary, COALESCE(e.content, ''), e.feed_id, e.published_at, e.is_unread, e.is_starred, COALESCE(f.title, ''), COALESCE(f.folder_name, '')
FROM entries e
LEFT JOIN feeds f ON f.id = e.feed_id
WHERE %s
ORDER BY e.published_at DESC
LIMIT ?
`, strings.Join(whereParts, " AND "))
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}
	defer rows.Close()

	entries := make([]feedbin.Entry, 0, limit)
	for rows.Next() {
		var entry feedbin.Entry
		var publishedAt string
		var isUnread int
		var isStarred int
		if err := rows.Scan(
			&entry.ID,
			&entry.Title,
			&entry.URL,
			&entry.Author,
			&entry.Summary,
			&entry.Content,
			&entry.FeedID,
			&publishedAt,
			&isUnread,
			&isStarred,
			&entry.FeedTitle,
			&entry.FeedFolder,
		); err != nil {
			return nil, fmt.Errorf("scan search entry: %w", err)
		}
		entry.PublishedAt, err = time.Parse(time.RFC3339Nano, publishedAt)
		if err != nil {
			return nil, fmt.Errorf("parse entry published_at %q: %w", publishedAt, err)
		}
		entry.IsUnread = intToBool(isUnread)
		entry.IsStarred = intToBool(isStarred)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search rows iteration: %w", err)
	}

	return entries, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func intToBool(v int) bool {
	return v != 0
}
