package feedbin

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticate_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authentication.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("missing authorization header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
}

func TestAuthenticate_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "wrong", ts.Client())
	err := c.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListEntries_SendsBasicAuthAndParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/entries.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "2" {
			t.Fatalf("unexpected page query: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("per_page") != "5" {
			t.Fatalf("unexpected per_page query: %s", r.URL.RawQuery)
		}

		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("u@example.com:secret"))
		if got := r.Header.Get("Authorization"); got != expectedAuth {
			t.Fatalf("unexpected auth header: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"title":"First","url":"https://example.com/1","feed_id":10,"published":"2026-02-01T00:00:00Z"}]`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	entries, err := c.ListEntries(context.Background(), 2, 5)
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "First" {
		t.Fatalf("unexpected title: %s", entries[0].Title)
	}
}

func TestListSubscriptions_ParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriptions.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"feed_id":10,"title":"Example Feed","feed_url":"https://example.com/feed.xml","site_url":"https://example.com"}]`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	subs, err := c.ListSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("ListSubscriptions returned error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Title != "Example Feed" {
		t.Fatalf("unexpected title: %s", subs[0].Title)
	}
}

func TestListUnreadEntryIDs_ParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/unread_entries.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[1,2,3]`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	ids, err := c.ListUnreadEntryIDs(context.Background())
	if err != nil {
		t.Fatalf("ListUnreadEntryIDs returned error: %v", err)
	}
	if len(ids) != 3 || ids[2] != 3 {
		t.Fatalf("unexpected ids: %+v", ids)
	}
}

func TestListStarredEntryIDs_ParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/starred_entries.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[10]`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	ids, err := c.ListStarredEntryIDs(context.Background())
	if err != nil {
		t.Fatalf("ListStarredEntryIDs returned error: %v", err)
	}
	if len(ids) != 1 || ids[0] != 10 {
		t.Fatalf("unexpected ids: %+v", ids)
	}
}

func TestMarkEntriesUnread_SendsPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/unread_entries.json" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("unexpected content-type: %s", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"unread_entries":[1,2]`) {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	if err := c.MarkEntriesUnread(context.Background(), []int64{1, 2}); err != nil {
		t.Fatalf("MarkEntriesUnread returned error: %v", err)
	}
}

func TestMarkEntriesRead_SendsDeletePayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/unread_entries.json" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"unread_entries":[1]`) {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	if err := c.MarkEntriesRead(context.Background(), []int64{1}); err != nil {
		t.Fatalf("MarkEntriesRead returned error: %v", err)
	}
}

func TestStarAndUnstarEntries(t *testing.T) {
	requests := make([]string, 0, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.Path+" "+string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "u@example.com", "secret", ts.Client())
	if err := c.StarEntries(context.Background(), []int64{5}); err != nil {
		t.Fatalf("StarEntries returned error: %v", err)
	}
	if err := c.UnstarEntries(context.Background(), []int64{5}); err != nil {
		t.Fatalf("UnstarEntries returned error: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if !strings.Contains(requests[0], "POST /starred_entries.json") || !strings.Contains(requests[0], `"starred_entries":[5]`) {
		t.Fatalf("unexpected first request: %s", requests[0])
	}
	if !strings.Contains(requests[1], "DELETE /starred_entries.json") || !strings.Contains(requests[1], `"starred_entries":[5]`) {
		t.Fatalf("unexpected second request: %s", requests[1])
	}
}
