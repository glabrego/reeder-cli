package config

import (
	"os"
	"testing"
)

func TestLoadFromEnv_UsesDefaults(t *testing.T) {
	t.Setenv("FEEDBIN_EMAIL", "user@example.com")
	t.Setenv("FEEDBIN_PASSWORD", "secret")
	t.Setenv("FEEDBIN_API_BASE_URL", "")
	t.Setenv("FEEDBIN_DB_PATH", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	if cfg.APIBaseURL != defaultAPIBaseURL {
		t.Fatalf("unexpected API base URL: %s", cfg.APIBaseURL)
	}
	if cfg.DBPath != "feedbin.db" {
		t.Fatalf("unexpected DB path: %s", cfg.DBPath)
	}
	if cfg.SearchMode != "like" {
		t.Fatalf("unexpected search mode: %s", cfg.SearchMode)
	}
}

func TestLoadFromEnv_MissingEmail(t *testing.T) {
	t.Setenv("FEEDBIN_EMAIL", "")
	t.Setenv("FEEDBIN_PASSWORD", "secret")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing email")
	}
}

func TestValidate_APIBaseURLTrailingSlash(t *testing.T) {
	cfg := Config{
		Email:      "user@example.com",
		Password:   "secret",
		APIBaseURL: "https://api.feedbin.com/v2/",
		DBPath:     "feedbin.db",
		SearchMode: "like",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidate_SearchMode(t *testing.T) {
	cfg := Config{
		Email:      "user@example.com",
		Password:   "secret",
		APIBaseURL: "https://api.feedbin.com/v2",
		DBPath:     "feedbin.db",
		SearchMode: "nope",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for search mode")
	}
}

func TestLoadFromEnv_IsolatedFromHostEnvironment(t *testing.T) {
	t.Setenv("FEEDBIN_EMAIL", "")
	t.Setenv("FEEDBIN_PASSWORD", "")
	os.Unsetenv("FEEDBIN_API_BASE_URL")
	os.Unsetenv("FEEDBIN_DB_PATH")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error when required credentials are missing")
	}
}
