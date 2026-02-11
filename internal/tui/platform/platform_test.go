package platform

import (
	"strings"
	"testing"
)

func TestValidateEntryURL(t *testing.T) {
	valid, err := ValidateEntryURL("https://example.com/path")
	if err != nil {
		t.Fatalf("unexpected error for valid URL: %v", err)
	}
	if valid != "https://example.com/path" {
		t.Fatalf("unexpected normalized URL: %q", valid)
	}

	_, err = ValidateEntryURL("ftp://example.com/path")
	if err == nil || !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}

	_, err = ValidateEntryURL("https://")
	if err == nil || !strings.Contains(err.Error(), "invalid URL host") {
		t.Fatalf("expected invalid host error, got %v", err)
	}
}
