package article

import (
	"net/url"
	"strings"
)

func applyReaderPostprocessing(lines []string, articleURL string) []string {
	if len(lines) == 0 {
		return nil
	}
	rules := readerFilterRules(articleURL)
	if len(rules.replaceAll) > 0 {
		for i := range lines {
			for old, newVal := range rules.replaceAll {
				lines[i] = strings.ReplaceAll(lines[i], old, newVal)
			}
		}
	}
	paragraphs := paragraphsFromLines(lines)
	if len(paragraphs) == 0 {
		return nil
	}
	kept := make([][]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		plain := normalizeRuleText(strings.Join(paragraph, " "))
		if plain == "" {
			continue
		}
		if matchesAnyContains(plain, rules.endBeforeContains) || matchesAnyEquals(plain, rules.endBeforeEquals) {
			break
		}
		if matchesAnyContains(plain, rules.skipParagraphContains) || matchesAnyEquals(plain, rules.skipParagraphEquals) {
			continue
		}
		kept = append(kept, paragraph)
	}
	if len(kept) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	for i, p := range kept {
		out = append(out, p...)
		if i < len(kept)-1 {
			out = append(out, "")
		}
	}
	return trimBlankLines(out)
}

func readerFilterRules(articleURL string) readerFilterRuleSet {
	host := strings.ToLower(strings.TrimSpace(articleURL))
	if parsed, err := url.Parse(articleURL); err == nil && parsed.Host != "" {
		host = strings.ToLower(parsed.Hostname())
	}
	rules := readerFilterRuleSet{}
	switch {
	case strings.Contains(host, "wikipedia.org"):
		rules.replaceAll = map[string]string{"[edit]": ""}
		rules.endBeforeEquals = []string{"references", "footnotes", "see also", "notes"}
	case strings.Contains(host, "nytimes.com"):
		rules.skipParagraphContains = []string{"credit:", "this is a developing story. check back for updates."}
		rules.skipParagraphEquals = []string{"credit", "image"}
	case strings.Contains(host, "wired.com"), strings.Contains(host, "wired.co.uk"):
		rules.skipParagraphContains = []string{"read more:", "do you use social media regularly? take our short survey."}
		rules.endBeforeEquals = []string{"more great wired stories"}
	case strings.Contains(host, "theguardian.com"):
		rules.skipParagraphContains = []string{"photograph:"}
	case strings.Contains(host, "arstechnica.com"):
		rules.skipParagraphContains = []string{"enlarge/", "this story originally appeared on"}
	case strings.Contains(host, "axios.com"):
		rules.skipParagraphContains = []string{
			"sign up for our daily briefing",
			"download for free.",
			"sign up for free.",
			"axios on your phone",
		}
	}
	return rules
}

func paragraphsFromLines(lines []string) [][]string {
	paragraphs := make([][]string, 0, 8)
	current := make([]string, 0, 4)
	for _, line := range lines {
		if strings.TrimSpace(stripANSI(line)) == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, current)
				current = make([]string, 0, 4)
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}
	return paragraphs
}

func normalizeRuleText(s string) string {
	s = strings.ToLower(normalizeInlineText(stripANSI(s)))
	for _, prefix := range []string{"▌", "│", "•", "◦", "▪", "▫", "-", "—", ">", "#"} {
		s = strings.TrimLeft(s, " ")
		s = strings.TrimPrefix(s, prefix)
	}
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func matchesAnyContains(text string, needles []string) bool {
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(needle))) {
			return true
		}
	}
	return false
}

func matchesAnyEquals(text string, candidates []string) bool {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if text == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func styleDetailLinks(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = reHTTPURL.ReplaceAllStringFunc(line, func(m string) string {
			return detailLinkURL.Render(m)
		})
	}
	return out
}
