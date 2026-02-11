package view

import (
	"strings"

	article "github.com/glabrego/reeder-cli/internal/render/article"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

const inlineImagePreviewAnchor = "__INLINE_IMAGE_PREVIEW_ANCHOR__"

type InlineImagePreviewState struct {
	Enabled bool
	Loading bool
	Raw     string
	Err     string
}

func DetailLines(
	entry feedbin.Entry,
	contentWidth int,
	horizontalMargin int,
	opts article.Options,
	wrap WrapFunc,
	preview InlineImagePreviewState,
) []string {
	lines := detailBaseLines(entry, contentWidth, opts, wrap)
	lines = appendInlineImagePreview(lines, preview, contentWidth)
	return leftPadLines(lines, horizontalMargin)
}

func DetailMaxTop(linesLen, bodyHeight int) int {
	maxTop := linesLen - bodyHeight
	if maxTop < 0 {
		return 0
	}
	return maxTop
}

func RenderDetailLines(lines []string, top, maxLines int) string {
	if len(lines) == 0 {
		return ""
	}
	if top < 0 {
		top = 0
	}
	if top > len(lines)-1 {
		top = len(lines) - 1
	}
	end := len(lines)
	if maxLines > 0 && top+maxLines < end {
		end = top + maxLines
	}
	return strings.Join(lines[top:end], "\n") + "\n"
}

func detailBaseLines(entry feedbin.Entry, width int, opts article.Options, wrap WrapFunc) []string {
	lines := DetailMetaLines(entry, width, wrap)
	contentLines := article.ContentLinesWithOptions(entry, width, opts)
	if len(contentLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, contentLines...)
	}
	return lines
}

func appendInlineImagePreview(lines []string, preview InlineImagePreviewState, contentWidth int) []string {
	if !preview.Enabled {
		return lines
	}
	previewLines := make([]string, 0, 3)
	if preview.Loading {
		previewLines = append(previewLines, "Loading image preview...")
	}
	if len(previewLines) == 0 {
		if previewRaw := strings.TrimSpace(preview.Raw); previewRaw != "" {
			if containsKittyGraphicsEscape(preview.Raw) {
				previewLines = append(previewLines, strings.TrimRight(preview.Raw, "\r\n"))
			} else {
				previewSplit := strings.Split(strings.TrimRight(preview.Raw, "\r\n"), "\n")
				previewLines = centerLines(previewSplit, contentWidth)
			}
		}
	}
	if len(previewLines) == 0 {
		if errMsg := strings.TrimSpace(preview.Err); errMsg != "" {
			previewLines = append(previewLines, "Image preview unavailable: "+errMsg)
		}
	}

	anchored := false
	out := make([]string, 0, len(lines)+len(previewLines)+1)
	for _, line := range lines {
		if line != inlineImagePreviewAnchor {
			out = append(out, line)
			continue
		}
		anchored = true
		if len(previewLines) > 0 {
			out = append(out, previewLines...)
		}
	}
	if anchored {
		return out
	}
	if len(previewLines) == 0 {
		return lines
	}
	out = append(out, "")
	out = append(out, previewLines...)
	return out
}

func leftPadLines(lines []string, padding int) []string {
	if padding <= 0 || len(lines) == 0 {
		return lines
	}
	prefix := strings.Repeat(" ", padding)
	out := make([]string, len(lines))
	for i, line := range lines {
		if containsKittyGraphicsEscape(line) {
			out[i] = line
			continue
		}
		out[i] = prefix + line
	}
	return out
}

func centerLines(lines []string, width int) []string {
	if width <= 0 || len(lines) == 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		visible := visibleLen(line)
		if visible >= width {
			out[i] = line
			continue
		}
		pad := (width - visible) / 2
		if pad < 0 {
			pad = 0
		}
		out[i] = strings.Repeat(" ", pad) + line
	}
	return out
}

func containsKittyGraphicsEscape(s string) bool {
	return strings.Contains(s, "\x1b_G")
}
