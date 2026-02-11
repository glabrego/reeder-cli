package tree

import (
	"sort"
	"strings"

	"github.com/glabrego/reeder-cli/internal/feedbin"
)

type RowKind string

const (
	RowSection RowKind = "section"
	RowFolder  RowKind = "folder"
	RowFeed    RowKind = "feed"
	RowArticle RowKind = "article"
)

type Row struct {
	Kind       RowKind
	Label      string
	Folder     string
	Feed       string
	EntryIndex int
}

type BuildOptions struct {
	Compact           bool
	CollapsedFolders  map[string]bool
	CollapsedFeeds    map[string]bool
	CollapsedSections map[string]bool
}

type feedGroup struct {
	Name         string
	EntryIndices []int
}

type collection struct {
	Kind  string // "folder" or "top_feed"
	Key   string
	Label string
	Feeds []feedGroup
}

func SortEntries(entries []feedbin.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		ai := entries[i]
		aj := entries[j]
		fi, fkindi := topCollectionLabelForEntry(ai)
		fj, fkindj := topCollectionLabelForEntry(aj)
		if fkindi != fkindj {
			return fkindi < fkindj
		}
		if fi != fj {
			return fi < fj
		}
		ti := strings.ToLower(FeedName(ai))
		tj := strings.ToLower(FeedName(aj))
		if ti != tj {
			return ti < tj
		}
		if !ai.PublishedAt.Equal(aj.PublishedAt) {
			return ai.PublishedAt.After(aj.PublishedAt)
		}
		return false
	})
}

func FeedKey(folder, feed string) string {
	return folder + "\x00" + feed
}

func SplitFeedKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return key, key
	}
	return parts[0], parts[1]
}

func FolderName(entry feedbin.Entry) string {
	return strings.TrimSpace(entry.FeedFolder)
}

func FeedName(entry feedbin.Entry) string {
	name := strings.TrimSpace(entry.FeedTitle)
	if name == "" {
		return "unknown feed"
	}
	return name
}

func BuildRows(entries []feedbin.Entry, opts BuildOptions) []Row {
	if opts.Compact {
		indices := make([]int, 0, len(entries))
		for i := range entries {
			indices = append(indices, i)
		}
		sort.SliceStable(indices, func(i, j int) bool {
			ei := entries[indices[i]]
			ej := entries[indices[j]]
			if !ei.PublishedAt.Equal(ej.PublishedAt) {
				return ei.PublishedAt.After(ej.PublishedAt)
			}
			ti := strings.ToLower(strings.TrimSpace(ei.Title))
			tj := strings.ToLower(strings.TrimSpace(ej.Title))
			if ti != tj {
				return ti < tj
			}
			return ei.ID < ej.ID
		})

		rows := make([]Row, 0, len(indices))
		for _, idx := range indices {
			entry := entries[idx]
			rows = append(rows, Row{
				Kind:       RowArticle,
				Folder:     FolderName(entry),
				Feed:       FeedName(entry),
				EntryIndex: idx,
			})
		}
		return rows
	}

	tree := buildCollections(entries)
	folderCollections := make([]collection, 0, len(tree))
	topFeedCollections := make([]collection, 0, len(tree))
	for _, c := range tree {
		if c.Kind == "folder" {
			folderCollections = append(folderCollections, c)
			continue
		}
		topFeedCollections = append(topFeedCollections, c)
	}

	rows := make([]Row, 0, len(entries)+len(tree)*2+2)
	if len(folderCollections) > 0 {
		rows = append(rows, Row{Kind: RowSection, Label: "Folders"})
		if opts.CollapsedSections["Folders"] {
			goto topFeeds
		}
	}
	for _, c := range folderCollections {
		rows = append(rows, Row{
			Kind:   RowFolder,
			Label:  c.Label,
			Folder: c.Key,
		})
		if opts.CollapsedFolders[c.Key] {
			continue
		}
		for _, fg := range c.Feeds {
			rows = append(rows, Row{
				Kind:   RowFeed,
				Label:  fg.Name,
				Folder: c.Key,
				Feed:   fg.Name,
			})
			if opts.CollapsedFeeds[FeedKey(c.Key, fg.Name)] {
				continue
			}
			for _, idx := range fg.EntryIndices {
				rows = append(rows, Row{
					Kind:       RowArticle,
					Folder:     c.Key,
					Feed:       fg.Name,
					EntryIndex: idx,
				})
			}
		}
	}

topFeeds:
	if len(topFeedCollections) > 0 {
		rows = append(rows, Row{Kind: RowSection, Label: "Feeds"})
		if opts.CollapsedSections["Feeds"] {
			return rows
		}
	}
	for _, c := range topFeedCollections {
		rows = append(rows, Row{
			Kind:  RowFeed,
			Label: c.Label,
			Feed:  c.Label,
		})
		if opts.CollapsedFeeds[FeedKey("", c.Label)] {
			continue
		}
		if len(c.Feeds) == 0 {
			continue
		}
		for _, idx := range c.Feeds[0].EntryIndices {
			rows = append(rows, Row{
				Kind:       RowArticle,
				Feed:       c.Label,
				EntryIndex: idx,
			})
		}
	}
	return rows
}

func FirstArticleRow(rows []Row) int {
	for i, row := range rows {
		if row.Kind == RowArticle {
			return i
		}
	}
	return 0
}

func topCollectionLabelForEntry(entry feedbin.Entry) (label string, kind string) {
	if folder := FolderName(entry); folder != "" {
		return folder, "folder"
	}
	return FeedName(entry), "top_feed"
}

func buildCollections(entries []feedbin.Entry) []collection {
	collections := make([]collection, 0, 16)
	collectionIndex := make(map[string]int)
	feedIndexByCollection := make(map[string]map[string]int)

	for idx, entry := range entries {
		collectionLabel, collectionKind := topCollectionLabelForEntry(entry)
		collectionKey := collectionKind + "\x00" + collectionLabel
		ci, ok := collectionIndex[collectionKey]
		if !ok {
			collections = append(collections, collection{
				Kind:  collectionKind,
				Key:   collectionLabel,
				Label: collectionLabel,
				Feeds: make([]feedGroup, 0, 8),
			})
			ci = len(collections) - 1
			collectionIndex[collectionKey] = ci
			feedIndexByCollection[collectionKey] = make(map[string]int)
		}

		feedName := FeedName(entry)
		if collectionKind == "top_feed" {
			feedName = collectionLabel
		}
		fi, ok := feedIndexByCollection[collectionKey][feedName]
		if !ok {
			collections[ci].Feeds = append(collections[ci].Feeds, feedGroup{Name: feedName})
			fi = len(collections[ci].Feeds) - 1
			feedIndexByCollection[collectionKey][feedName] = fi
		}
		collections[ci].Feeds[fi].EntryIndices = append(collections[ci].Feeds[fi].EntryIndices, idx)
	}

	for i := range collections {
		for j := range collections[i].Feeds {
			sort.SliceStable(collections[i].Feeds[j].EntryIndices, func(a, b int) bool {
				ea := entries[collections[i].Feeds[j].EntryIndices[a]]
				eb := entries[collections[i].Feeds[j].EntryIndices[b]]
				if !ea.PublishedAt.Equal(eb.PublishedAt) {
					return ea.PublishedAt.After(eb.PublishedAt)
				}
				return strings.ToLower(strings.TrimSpace(ea.Title)) < strings.ToLower(strings.TrimSpace(eb.Title))
			})
		}

		sort.SliceStable(collections[i].Feeds, func(a, b int) bool {
			na := strings.ToLower(strings.TrimSpace(collections[i].Feeds[a].Name))
			nb := strings.ToLower(strings.TrimSpace(collections[i].Feeds[b].Name))
			if na != nb {
				return na < nb
			}
			return collections[i].Feeds[a].Name < collections[i].Feeds[b].Name
		})
	}

	sort.SliceStable(collections, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(collections[i].Label))
		lj := strings.ToLower(strings.TrimSpace(collections[j].Label))
		if li != lj {
			return li < lj
		}
		return collections[i].Label < collections[j].Label
	})

	return collections
}
