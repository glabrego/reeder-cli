GO ?= go

.PHONY: test bench golden-update

test:
	$(GO) test ./...

bench:
	$(GO) test ./internal/render/article ./internal/tui/tree -run '^$$' -bench 'Benchmark(ContentLinesWithOptions_ComplexArticle|BuildRows_)' -benchmem

golden-update:
	$(GO) test ./internal/render/article -run TestContentLines_GoldenComplexArticle -update-article-golden
	$(GO) test ./internal/render/article -run TestContentLines_SourceGolden -update-article-golden
	$(GO) test ./internal/tui/view -run TestListRendering_Golden -update-view-golden
	$(GO) test ./internal/tui -run 'TestScreenGolden_ListAndDetail|TestScreenGolden_NerdModeListAndDetail' -update-tui-screen-golden
