package view

import (
	"fmt"
	"strings"

	tuitheme "github.com/glabrego/reeder-cli/internal/tui/theme"
)

func Toolbar(nerdMode, inDetail bool) string {
	if nerdMode {
		if inDetail {
			return "j/k: scroll | [ ]: prev/next | o: open URL | y: copy URL | U: toggle unread | S: toggle star | esc/backspace: back | ?: help | q: quit"
		}
		return "j/k/arrows: move | [ ]: sections | g/G: top/bottom | pgup/pgdown: jump | c: compact | N: numbering | d: time format | t: mark-on-open | p: confirm prompt | /: search | ctrl+l: clear search | enter: details | a/u/*: filter | n: more | U/S: toggle | y: copy URL | ?: help | r: refresh | q: quit"
	}
	if inDetail {
		return "j/k scroll | [ ] prev/next | o open | y copy | U/S toggle | esc back | ? help"
	}
	return "j/k move | enter open | / search | a/u/* filter | n more | r refresh | ? help"
}

func CompactFooter(mode, filter string, page, shown int, searchQuery string, searchMatchCount int, th tuitheme.Theme) string {
	parts := []string{
		th.MetaLabel.Render("mode") + " " + th.MetaValue.Render(mode),
		th.MetaLabel.Render("filter") + " " + th.MetaValue.Render(filter),
		th.MetaLabel.Render("page") + " " + th.MetaValue.Render(fmt.Sprintf("%d", page)),
		th.MetaValue.Render(fmt.Sprintf("%d shown", shown)),
	}
	if searchQuery != "" {
		parts = append(parts, th.MetaLabel.Render("search")+" "+th.MetaValue.Render(fmt.Sprintf("%q (%d)", searchQuery, searchMatchCount)))
	}
	return strings.Join(parts, " • ")
}

func NerdFooter(mode, filter string, page, shown, lastFetch int, timeFormat, numbering, onOpen, confirm, searchQuery string, searchMatchCount int) string {
	footer := fmt.Sprintf("Mode: %s | Filter: %s | Page: %d | Showing: %d | Last fetch: %d | Time: %s | Nums: %s | Open->Read: %s | Confirm: %s", mode, filter, page, shown, lastFetch, timeFormat, numbering, onOpen, confirm)
	if searchQuery != "" {
		return fmt.Sprintf("%s | Search: %s (%d)", footer, searchQuery, searchMatchCount)
	}
	return footer
}

func CompactMessage(loading bool, hasWarning bool, status, warning string, th tuitheme.Theme) string {
	state := "idle"
	if loading {
		state = "loading"
	}
	if hasWarning {
		state = "warning"
	}
	main := "Ready"
	if status != "" {
		main = status
	} else if hasWarning {
		main = warning
	}
	stateLabel := th.StateIdle.Render("state")
	switch state {
	case "warning":
		stateLabel = th.StateWarn.Render("state")
	case "loading":
		stateLabel = th.StateLoad.Render("state")
	}
	return fmt.Sprintf("%s: %s | %s", stateLabel, state, th.MetaValue.Render(main))
}

func NerdMessage(status, warning, state, startup string) string {
	return fmt.Sprintf("Status: %s | Warning: %s | State: %s | Startup: %s", status, warning, state, startup)
}
