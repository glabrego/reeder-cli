package storage

import (
	"context"
	"database/sql"
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
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entries (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  author TEXT,
  summary TEXT,
  feed_id INTEGER NOT NULL,
  published_at TEXT NOT NULL,
  fetched_at TEXT NOT NULL,
  is_unread INTEGER NOT NULL DEFAULT 0,
  is_starred INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(feed_id) REFERENCES feeds(id)
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
INSERT INTO feeds (id, title, feed_url, site_url, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title=excluded.title,
  feed_url=excluded.feed_url,
  site_url=excluded.site_url,
  updated_at=excluded.updated_at
`)
	if err != nil {
		return fmt.Errorf("prepare save feeds statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, sub := range subscriptions {
		_, err := stmt.ExecContext(ctx, sub.ID, sub.Title, sub.FeedURL, sub.SiteURL, now)
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
INSERT INTO entries (id, title, url, author, summary, feed_id, published_at, fetched_at, is_unread, is_starred)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title=excluded.title,
  url=excluded.url,
  author=excluded.author,
  summary=excluded.summary,
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

func (r *Repository) ListEntries(ctx context.Context, limit int) ([]feedbin.Entry, error) {
	if limit < 1 {
		limit = 20
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT e.id, e.title, e.url, e.author, e.summary, e.feed_id, e.published_at, e.is_unread, e.is_starred, COALESCE(f.title, '')
FROM entries e
LEFT JOIN feeds f ON f.id = e.feed_id
ORDER BY e.published_at DESC
LIMIT ?
`, limit)
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
			&entry.FeedID,
			&publishedAt,
			&isUnread,
			&isStarred,
			&entry.FeedTitle,
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

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func intToBool(v int) bool {
	return v != 0
}
