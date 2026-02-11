package platform

import (
	"errors"
	"reflect"
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

func TestBrowserCommand(t *testing.T) {
	cases := []struct {
		goos string
		url  string
		name string
		args []string
	}{
		{goos: "darwin", url: "https://example.com", name: "open", args: []string{"https://example.com"}},
		{goos: "windows", url: "https://example.com", name: "rundll32", args: []string{"url.dll,FileProtocolHandler", "https://example.com"}},
		{goos: "linux", url: "https://example.com", name: "xdg-open", args: []string{"https://example.com"}},
	}
	for _, tc := range cases {
		gotName, gotArgs := browserCommand(tc.goos, tc.url)
		if gotName != tc.name || !reflect.DeepEqual(gotArgs, tc.args) {
			t.Fatalf("browserCommand(%q) = (%q, %v), want (%q, %v)", tc.goos, gotName, gotArgs, tc.name, tc.args)
		}
	}
}

func TestSelectClipboardCommand(t *testing.T) {
	lookup := func(bin string) (string, error) {
		if bin == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", errors.New("not found")
	}
	got, err := selectClipboardCommand(lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"xclip", "-selection", "clipboard"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected command: got=%v want=%v", got, want)
	}

	none := func(string) (string, error) { return "", errors.New("not found") }
	if _, err := selectClipboardCommand(none); err == nil {
		t.Fatal("expected error when no clipboard command is available")
	}
}
