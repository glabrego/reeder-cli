package article

import (
	"testing"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func BenchmarkContentLinesWithOptions_ComplexArticle(b *testing.B) {
	entry := feedbin.Entry{
		URL: "https://example.com/story",
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
				<tr><td>Quality</td><td>High</td></tr>
			</table>
			<p><img src="https://example.com/image.jpg" alt="Cabin view"></p>
		</article>`,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ContentLinesWithOptions(entry, 72, DefaultOptions)
	}
}
