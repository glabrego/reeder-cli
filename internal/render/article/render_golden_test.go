package article

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var updateArticleGolden = flag.Bool("update-article-golden", false, "update article golden files")

func TestContentLines_GoldenComplexArticle(t *testing.T) {
	entry := feedbin.Entry{
		URL: "https://example.com/story",
		Content: `<article>
			<h1>Main Title</h1>
			<h2>Subtitle</h2>
			<p>Intro paragraph with a <a href="https://example.com/link">reference link</a>.</p>
			<ul><li>First point</li><li>Second point</li></ul>
			<ol><li>Step one</li><li>Step two</li></ol>
			<blockquote><p>Quoted claim</p><cite>Jane Doe</cite></blockquote>
			<table>
				<tr><th>Metric</th><th>Value</th></tr>
				<tr><td>Speed</td><td>Fast</td></tr>
				<tr><td>Quality</td><td>High</td></tr>
			</table>
			<p><img src="https://example.com/image.jpg" alt="Cabin view"></p>
		</article>`,
	}

	lines := ContentLinesWithOptions(entry, 72, DefaultOptions)
	got := stripANSIForTest.ReplaceAllString(strings.Join(lines, "\n"), "")
	assertGolden(t, "complex_article.golden", got)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateArticleGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := strings.TrimRight(string(wantBytes), "\n")
	got = strings.TrimRight(got, "\n")
	if got != want {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
