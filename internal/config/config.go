package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultAPIBaseURL = "https://api.feedbin.com/v2"

// Config holds runtime settings for the CLI app.
type Config struct {
	Email      string
	Password   string
	APIBaseURL string
	DBPath     string
	SearchMode string

	ArticleStyleLinks   bool
	ArticlePostprocess  bool
	ArticleImageModeRaw string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		Email:              os.Getenv("FEEDBIN_EMAIL"),
		Password:           os.Getenv("FEEDBIN_PASSWORD"),
		APIBaseURL:         os.Getenv("FEEDBIN_API_BASE_URL"),
		DBPath:             os.Getenv("FEEDBIN_DB_PATH"),
		SearchMode:         os.Getenv("FEEDBIN_SEARCH_MODE"),
		ArticleStyleLinks:  parseEnvBoolWithDefault("FEEDBIN_ARTICLE_STYLE_LINKS", true),
		ArticlePostprocess: parseEnvBoolWithDefault("FEEDBIN_ARTICLE_POSTPROCESS", true),
		ArticleImageModeRaw: strings.ToLower(strings.TrimSpace(
			os.Getenv("FEEDBIN_ARTICLE_IMAGE_MODE"),
		)),
	}

	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "feedbin.db"
	}
	if cfg.SearchMode == "" {
		cfg.SearchMode = "like"
	}
	if cfg.ArticleImageModeRaw == "" {
		cfg.ArticleImageModeRaw = "label"
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Email == "" {
		return errors.New("FEEDBIN_EMAIL is required")
	}
	if c.Password == "" {
		return errors.New("FEEDBIN_PASSWORD is required")
	}
	if c.APIBaseURL == "" {
		return errors.New("APIBaseURL is required")
	}
	if c.DBPath == "" {
		return errors.New("DBPath is required")
	}
	if c.SearchMode != "like" && c.SearchMode != "fts" {
		return fmt.Errorf("SearchMode must be like or fts: %s", c.SearchMode)
	}
	if c.ArticleImageModeRaw != "label" && c.ArticleImageModeRaw != "none" {
		return fmt.Errorf("FEEDBIN_ARTICLE_IMAGE_MODE must be label or none: %s", c.ArticleImageModeRaw)
	}
	if c.APIBaseURL[len(c.APIBaseURL)-1] == '/' {
		return fmt.Errorf("APIBaseURL must not end with '/': %s", c.APIBaseURL)
	}
	return nil
}

func parseEnvBoolWithDefault(name string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	ok, err := strconv.ParseBool(v)
	if err != nil {
		return v == "1"
	}
	return ok
}
