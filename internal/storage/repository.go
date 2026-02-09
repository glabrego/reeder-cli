package storage

import (
	"context"
	"database/sql"
	"fmt"
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
CREATE TABLE IF NOT EXISTS entries (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  author TEXT,
  summary TEXT,
  feed_id INTEGER NOT NULL,
  published_at TEXT NOT NULL,
  fetched_at TEXT NOT NULL
);
`
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
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
INSERT INTO entries (id, title, url, author, summary, feed_id, published_at, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  title=excluded.title,
  url=excluded.url,
  author=excluded.author,
  summary=excluded.summary,
  feed_id=excluded.feed_id,
  published_at=excluded.published_at,
  fetched_at=excluded.fetched_at
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

func (r *Repository) ListEntries(ctx context.Context, limit int) ([]feedbin.Entry, error) {
	if limit < 1 {
		limit = 20
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, title, url, author, summary, feed_id, published_at
FROM entries
ORDER BY published_at DESC
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
		if err := rows.Scan(
			&entry.ID,
			&entry.Title,
			&entry.URL,
			&entry.Author,
			&entry.Summary,
			&entry.FeedID,
			&publishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}

		entry.PublishedAt, err = time.Parse(time.RFC3339Nano, publishedAt)
		if err != nil {
			return nil, fmt.Errorf("parse entry published_at %q: %w", publishedAt, err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return entries, nil
}
