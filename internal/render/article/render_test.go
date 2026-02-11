package article

import (
	"regexp"
	"strings"
	"testing"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

var stripANSIForTest = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestTextFromEntry_FallsBackToSummary(t *testing.T) {
	entry := feedbin.Entry{
		Summary: "Only summary",
		Content: "",
	}
	got := TextFromEntry(entry)
	if got != "Only summary" {
		t.Fatalf("expected summary fallback, got %q", got)
	}
}

func TestImageURLsFromContent_OnlyHTTPAndDeduplicated(t *testing.T) {
	content := `<p><img src="https://example.com/a.jpg"><img src="https://example.com/a.jpg"><img src='http://example.com/b.png'><img src="data:image/png;base64,abc"></p>`
	got := ImageURLsFromContent(content)
	if len(got) != 2 {
		t.Fatalf("expected 2 URLs, got %d (%+v)", len(got), got)
	}
	if got[0] != "https://example.com/a.jpg" || got[1] != "http://example.com/b.png" {
		t.Fatalf("unexpected image URLs: %+v", got)
	}
}

func TestContentLines_ImagesFollowContentOrder(t *testing.T) {
	entry := feedbin.Entry{
		Content: `<p>First paragraph.</p><p><img src="https://example.com/one.jpg" alt="Figure one"></p><p>Second paragraph.</p><p><img src="https://example.com/two.jpg" alt="Figure two"></p>`,
	}

	lines := ContentLines(entry, 80)
	got := stripANSIForTest.ReplaceAllString(strings.Join(lines, "\n"), "")

	firstText := strings.Index(got, "First paragraph.")
	firstImage := strings.Index(got, "Image Figure one")
	secondText := strings.Index(got, "Second paragraph.")
	if firstText == -1 || firstImage == -1 || secondText == -1 {
		t.Fatalf("expected text and first image label in output, got %q", got)
	}
	if !(firstText < firstImage && firstImage < secondText) {
		t.Fatalf("expected content/image order preserved via image label placement, got %q", got)
	}
	if strings.Contains(got, "https://example.com/one.jpg") || strings.Contains(got, "https://example.com/two.jpg") {
		t.Fatalf("expected raw image URL lines hidden from article content, got %q", got)
	}
}

func TestContentLines_RendersCommonArticleElements(t *testing.T) {
	entry := feedbin.Entry{
		Content: `<article>
			<h1>Main Title</h1>
			<h2>Subtitle</h2>
			<p>Intro with a <a href="https://example.com/link">reference</a>.</p>
			<ul><li>First point</li><li>Second point</li></ul>
			<ol><li>Step one</li><li>Step two</li></ol>
			<blockquote><p>Quoted claim</p><cite>Jane Doe</cite></blockquote>
			<table>
				<tr><th>Metric</th><th>Value</th></tr>
				<tr><td>Speed</td><td>Fast</td></tr>
			</table>
		</article>`,
	}

	lines := ContentLines(entry, 80)
	got := stripANSIForTest.ReplaceAllString(strings.Join(lines, "\n"), "")

	for _, want := range []string{
		"▌ Main Title",
		"▌ Subtitle",
		"reference (https://example.com/link)",
		"• First point",
		"1. Step one",
		"│ Quoted claim",
		"│ Jane Doe",
		"| Metric | Value |",
		"| Speed | Fast |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in rendered output, got %q", want, got)
		}
	}
}

func TestContentLines_PostProcessingWikipediaStopsAtReferences(t *testing.T) {
	entry := feedbin.Entry{
		URL: "https://en.wikipedia.org/wiki/Go_(programming_language)",
		Content: `<article>
			<h2>Overview</h2>
			<p>Go is a statically typed language.</p>
			<h2>References [edit]</h2>
			<p>[1] Source</p>
		</article>`,
	}

	got := stripANSIForTest.ReplaceAllString(strings.Join(ContentLines(entry, 80), "\n"), "")
	if !strings.Contains(got, "Overview") || !strings.Contains(got, "Go is a statically typed language.") {
		t.Fatalf("expected leading content to be preserved, got %q", got)
	}
	if strings.Contains(strings.ToLower(got), "references") || strings.Contains(got, "[1] Source") {
		t.Fatalf("expected references section to be removed, got %q", got)
	}
}

func TestContentLines_PostProcessingWiredRemovesPromoTail(t *testing.T) {
	entry := feedbin.Entry{
		URL: "https://www.wired.com/story/example",
		Content: `<article>
			<p>Main article body.</p>
			<h2>More Great WIRED Stories</h2>
			<p>Subscribe now.</p>
		</article>`,
	}

	got := stripANSIForTest.ReplaceAllString(strings.Join(ContentLines(entry, 80), "\n"), "")
	if !strings.Contains(got, "Main article body.") {
		t.Fatalf("expected main body preserved, got %q", got)
	}
	if strings.Contains(got, "More Great WIRED Stories") || strings.Contains(got, "Subscribe now.") {
		t.Fatalf("expected promo tail removed, got %q", got)
	}
}
