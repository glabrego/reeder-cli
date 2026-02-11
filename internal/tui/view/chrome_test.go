package view

import (
	"regexp"
	"strings"
	"testing"

	tuitheme "github.com/glabrego/reeder-cli/internal/tui/theme"
)

var ansiStrip = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiStrip.ReplaceAllString(s, "")
}

func TestToolbar(t *testing.T) {
	if got := Toolbar(false, false); !strings.Contains(got, "j/k move") {
		t.Fatalf("unexpected compact list toolbar: %q", got)
	}
	if got := Toolbar(false, true); !strings.Contains(got, "j/k scroll") {
		t.Fatalf("unexpected compact detail toolbar: %q", got)
	}
	if got := Toolbar(true, false); !strings.Contains(got, "j/k/arrows") {
		t.Fatalf("unexpected nerd list toolbar: %q", got)
	}
	if got := Toolbar(true, true); !strings.Contains(got, "toggle unread") {
		t.Fatalf("unexpected nerd detail toolbar: %q", got)
	}
}

func TestCompactFooter(t *testing.T) {
	th := tuitheme.Default()
	got := stripANSI(CompactFooter("list", "all", 1, 42, "go", 3, th))
	for _, want := range []string{"mode list", "filter all", "page 1", "42 shown", `search "go" (3)`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in footer, got %q", want, got)
		}
	}
}

func TestNerdFooter(t *testing.T) {
	got := NerdFooter("detail", "unread", 2, 20, 10, "relative", "off", "on", "off", "", 0)
	if !strings.Contains(got, "Mode: detail | Filter: unread | Page: 2") {
		t.Fatalf("unexpected nerd footer: %q", got)
	}
}

func TestCompactMessage(t *testing.T) {
	th := tuitheme.Default()
	if got := stripANSI(CompactMessage(false, false, "", "", th)); !strings.Contains(got, "state: idle | Ready") {
		t.Fatalf("unexpected idle compact message: %q", got)
	}
	if got := stripANSI(CompactMessage(true, false, "", "", th)); !strings.Contains(got, "state: loading") {
		t.Fatalf("unexpected loading compact message: %q", got)
	}
	if got := stripANSI(CompactMessage(false, true, "", "boom", th)); !strings.Contains(got, "state: warning | boom") {
		t.Fatalf("unexpected warning compact message: %q", got)
	}
}

func TestNerdMessage(t *testing.T) {
	got := NerdMessage("ok", "-", "idle", "cache 10ms")
	if !strings.Contains(got, "Status: ok | Warning: - | State: idle | Startup: cache 10ms") {
		t.Fatalf("unexpected nerd message: %q", got)
	}
}
