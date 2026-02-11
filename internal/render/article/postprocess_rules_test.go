package article

import (
	"reflect"
	"testing"
)

func TestReaderFilterRules_BySource(t *testing.T) {
	cases := []struct {
		name  string
		url   string
		check func(t *testing.T, rules readerFilterRuleSet)
	}{
		{
			name: "wikipedia",
			url:  "https://en.wikipedia.org/wiki/Go_(programming_language)",
			check: func(t *testing.T, rules readerFilterRuleSet) {
				t.Helper()
				if got := rules.replaceAll["[edit]"]; got != "" {
					t.Fatalf("expected wikipedia [edit] replacement, got %q", got)
				}
				want := []string{"references", "footnotes", "see also", "notes"}
				if !reflect.DeepEqual(rules.endBeforeEquals, want) {
					t.Fatalf("unexpected wikipedia endBeforeEquals: got=%v want=%v", rules.endBeforeEquals, want)
				}
			},
		},
		{
			name: "nytimes",
			url:  "https://www.nytimes.com/2026/02/11/technology/example.html",
			check: func(t *testing.T, rules readerFilterRuleSet) {
				t.Helper()
				wantContains := []string{"credit:", "this is a developing story. check back for updates."}
				wantEquals := []string{"credit", "image"}
				if !reflect.DeepEqual(rules.skipParagraphContains, wantContains) {
					t.Fatalf("unexpected nytimes skipParagraphContains: got=%v want=%v", rules.skipParagraphContains, wantContains)
				}
				if !reflect.DeepEqual(rules.skipParagraphEquals, wantEquals) {
					t.Fatalf("unexpected nytimes skipParagraphEquals: got=%v want=%v", rules.skipParagraphEquals, wantEquals)
				}
			},
		},
		{
			name: "wired",
			url:  "https://www.wired.com/story/example",
			check: func(t *testing.T, rules readerFilterRuleSet) {
				t.Helper()
				wantContains := []string{"read more:", "do you use social media regularly? take our short survey."}
				wantEnd := []string{"more great wired stories"}
				if !reflect.DeepEqual(rules.skipParagraphContains, wantContains) {
					t.Fatalf("unexpected wired skipParagraphContains: got=%v want=%v", rules.skipParagraphContains, wantContains)
				}
				if !reflect.DeepEqual(rules.endBeforeEquals, wantEnd) {
					t.Fatalf("unexpected wired endBeforeEquals: got=%v want=%v", rules.endBeforeEquals, wantEnd)
				}
			},
		},
		{
			name: "guardian",
			url:  "https://www.theguardian.com/technology/2026/feb/11/example",
			check: func(t *testing.T, rules readerFilterRuleSet) {
				t.Helper()
				wantContains := []string{"photograph:"}
				if !reflect.DeepEqual(rules.skipParagraphContains, wantContains) {
					t.Fatalf("unexpected guardian skipParagraphContains: got=%v want=%v", rules.skipParagraphContains, wantContains)
				}
			},
		},
		{
			name: "axios",
			url:  "https://www.axios.com/2026/02/11/example",
			check: func(t *testing.T, rules readerFilterRuleSet) {
				t.Helper()
				wantContains := []string{"sign up for our daily briefing", "download for free.", "sign up for free.", "axios on your phone"}
				if !reflect.DeepEqual(rules.skipParagraphContains, wantContains) {
					t.Fatalf("unexpected axios skipParagraphContains: got=%v want=%v", rules.skipParagraphContains, wantContains)
				}
			},
		},
		{
			name: "default",
			url:  "https://example.com/article",
			check: func(t *testing.T, rules readerFilterRuleSet) {
				t.Helper()
				if len(rules.skipParagraphContains) != 0 || len(rules.skipParagraphEquals) != 0 || len(rules.endBeforeEquals) != 0 || len(rules.endBeforeContains) != 0 || len(rules.replaceAll) != 0 {
					t.Fatalf("expected empty default rules, got %+v", rules)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rules := readerFilterRules(tc.url)
			tc.check(t, rules)
		})
	}
}

func TestNormalizeRuleText(t *testing.T) {
	got := normalizeRuleText("  ▌  References [edit]  ")
	if got != "references [edit]" {
		t.Fatalf("unexpected normalized rule text: %q", got)
	}
}
