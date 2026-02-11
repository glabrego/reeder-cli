package article

import (
	"strings"
	"testing"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

func FuzzContentLinesWithOptions(f *testing.F) {
	seeds := []string{
		"",
		"<p>Hello world</p>",
		"<article><h1>Title</h1><p>Paragraph</p></article>",
		"<div><img src='https://example.com/image.jpg' alt='Image'></div>",
		"<table><tr><th>a</th><th>b</th></tr><tr><td>1</td><td>2</td></tr></table>",
		"<blockquote><p>Quote</p><cite>Author</cite></blockquote>",
		"<<<<<<<<",
		"\x00\x01\x02<script>alert(1)</script>",
	}
	for _, s := range seeds {
		f.Add(s, "https://example.com/story")
	}

	f.Fuzz(func(t *testing.T, raw, articleURL string) {
		if len(raw) > 10_000 {
			raw = raw[:10_000]
		}
		if len(articleURL) > 512 {
			articleURL = articleURL[:512]
		}
		entry := feedbin.Entry{
			Content: raw,
			URL:     strings.TrimSpace(articleURL),
		}
		for _, width := range []int{1, 20, 72} {
			_ = ContentLinesWithOptions(entry, width, DefaultOptions)
			_ = ContentLinesWithOptions(entry, width, Options{
				StyleLinks:          false,
				ApplyPostprocessing: false,
				ImageMode:           ImageModeNone,
			})
			_ = TextFromEntryWithOptions(entry, DefaultOptions)
		}
	})
}
