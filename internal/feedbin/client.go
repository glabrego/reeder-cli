package feedbin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Entry is the subset of Feedbin fields required by the app.
type Entry struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Author      string    `json:"author"`
	Summary     string    `json:"summary"`
	FeedID      int64     `json:"feed_id"`
	PublishedAt time.Time `json:"published"`

	FeedTitle string `json:"-"`
	IsUnread  bool   `json:"-"`
	IsStarred bool   `json:"-"`
}

// Subscription describes the subset of feed metadata used by the app.
type Subscription struct {
	ID      int64  `json:"feed_id"`
	Title   string `json:"title"`
	FeedURL string `json:"feed_url"`
	SiteURL string `json:"site_url"`
}

type Client struct {
	baseURL  string
	email    string
	password string
	http     *http.Client
}

func NewClient(baseURL, email, password string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		email:    email,
		password: password,
		http:     httpClient,
	}
}

func (c *Client) Authenticate(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/authentication.json", nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("authenticate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid credentials")
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("authenticate failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *Client) ListEntries(ctx context.Context, page, perPage int) ([]Entry, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	q := make(url.Values)
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))

	req, err := c.newRequest(ctx, http.MethodGet, "/entries.json?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list entries request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("list entries failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var entries []Entry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode entries response: %w", err)
	}
	return entries, nil
}

func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/subscriptions.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("list subscriptions failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var subscriptions []Subscription
	if err := json.NewDecoder(resp.Body).Decode(&subscriptions); err != nil {
		return nil, fmt.Errorf("decode subscriptions response: %w", err)
	}
	return subscriptions, nil
}

func (c *Client) ListUnreadEntryIDs(ctx context.Context) ([]int64, error) {
	return c.listEntryIDs(ctx, "/unread_entries.json", "unread entries")
}

func (c *Client) ListStarredEntryIDs(ctx context.Context) ([]int64, error) {
	return c.listEntryIDs(ctx, "/starred_entries.json", "starred entries")
}

func (c *Client) listEntryIDs(ctx context.Context, path, resource string) ([]int64, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list %s request failed: %w", resource, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("list %s failed with status %d: %s", resource, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", resource, err)
	}
	return ids, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(c.email, c.password)
	req.Header.Set("Accept", "application/json")
	return req, nil
}
