package article

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func TestContentLines_SourceGolden(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		url  string
	}{
		{name: "wikipedia", url: "https://en.wikipedia.org/wiki/Go_(programming_language)"},
		{name: "wired", url: "https://www.wired.com/story/example"},
		{name: "nytimes", url: "https://www.nytimes.com/2026/02/11/technology/example.html"},
		{name: "guardian", url: "https://www.theguardian.com/technology/2026/feb/11/example"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			htmlPath := filepath.Join("testdata", "sources", tc.name+".html")
			raw, err := os.ReadFile(htmlPath)
			if err != nil {
				t.Fatalf("read fixture %s: %v", htmlPath, err)
			}
			entry := feedbin.Entry{
				URL:     tc.url,
				Content: string(raw),
			}
			lines := ContentLinesWithOptions(entry, 72, DefaultOptions)
			got := stripANSIForTest.ReplaceAllString(strings.Join(lines, "\n"), "")
			assertGolden(t, filepath.Join("sources", tc.name+".golden"), got)
		})
	}
}
