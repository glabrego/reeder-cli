package view

import (
	"strings"
	"testing"
)

func TestSupportsKittyGraphics(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "ghostty")
	t.Setenv("TERM", "dumb")
	if !SupportsKittyGraphics() {
		t.Fatal("expected ghostty to support kitty graphics")
	}

	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-256color")
	if SupportsKittyGraphics() {
		t.Fatal("expected plain xterm-256color not to support kitty graphics")
	}
}

func TestKittyHelpers(t *testing.T) {
	if !ContainsKittyGraphicsEscape("\x1b_Ga=T,f=32\x1b\\") {
		t.Fatal("expected kitty graphics escape detection")
	}
	if ContainsKittyGraphicsEscape("plain text") {
		t.Fatal("did not expect escape detection for plain text")
	}
	if KittyRenderedLineCount("") != 0 {
		t.Fatal("expected zero line count for empty string")
	}
	if KittyRenderedLineCount("a\nb\nc") != 3 {
		t.Fatal("expected 3 rendered lines")
	}
}

func TestKittyPassthroughMode(t *testing.T) {
	t.Setenv("TMUX", "")
	if got := KittyPassthroughMode(); got != "none" {
		t.Fatalf("KittyPassthroughMode() = %q, want %q", got, "none")
	}

	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	if got := KittyPassthroughMode(); got != "screen" {
		t.Fatalf("KittyPassthroughMode() = %q, want %q", got, "screen")
	}
}

func TestClearKittyGraphicsSequence_TmuxWrapped(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	got := ClearKittyGraphicsSequence()
	if !strings.HasPrefix(got, "\x1bPtmux;\x1b") {
		t.Fatalf("expected tmux passthrough prefix, got %q", got)
	}
	if !strings.Contains(got, "\x1b\x1b_Ga=d,d=A") {
		t.Fatalf("expected escaped kitty delete command in tmux wrapper, got %q", got)
	}
	if !strings.HasSuffix(got, "\x1b\\") {
		t.Fatalf("expected tmux passthrough suffix, got %q", got)
	}
}
