package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glabrego/feedbin-cli/internal/app"
	"github.com/glabrego/feedbin-cli/internal/config"
	"github.com/glabrego/feedbin-cli/internal/feedbin"
	"github.com/glabrego/feedbin-cli/internal/storage"
	"github.com/glabrego/feedbin-cli/internal/tui"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	repo, err := storage.NewRepositoryWithSearch(cfg.DBPath, cfg.SearchMode)
	if err != nil {
		log.Fatalf("storage init error: %v", err)
	}
	defer repo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := repo.Init(ctx); err != nil {
		log.Fatalf("storage schema error: %v", err)
	}
	if err := repo.CheckWritable(ctx); err != nil {
		log.Fatalf("storage write check failed (%v). Verify FEEDBIN_DB_PATH is writable: %s", err, cfg.DBPath)
	}

	client := feedbin.NewClient(cfg.APIBaseURL, cfg.Email, cfg.Password, nil)
	service := app.NewService(client, repo)

	cacheLoadStart := time.Now()
	entries, err := service.ListCached(ctx, app.DefaultCacheLimit)
	if err != nil {
		log.Fatalf("cannot load cached entries: %v", err)
	}
	cacheLoadDuration := time.Since(cacheLoadStart)

	model := tui.NewModel(service, entries)
	model.SetStartupCacheStats(cacheLoadDuration, len(entries))

	prefCtx, prefCancel := context.WithTimeout(context.Background(), 5*time.Second)
	prefs, err := service.LoadUIPreferences(prefCtx)
	prefCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load UI preferences (%v), using defaults\n", err)
	} else {
		model.ApplyPreferences(tui.Preferences{
			Compact:         prefs.Compact,
			MarkReadOnOpen:  prefs.MarkReadOnOpen,
			ConfirmOpenRead: prefs.ConfirmOpenRead,
			RelativeTime:    prefs.RelativeTime,
			ShowNumbers:     prefs.ShowNumbers,
		})
	}

	model.SetPreferencesSaver(func(p tui.Preferences) error {
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		return service.SaveUIPreferences(saveCtx, app.UIPreferences{
			Compact:         p.Compact,
			MarkReadOnOpen:  p.MarkReadOnOpen,
			ConfirmOpenRead: p.ConfirmOpenRead,
			RelativeTime:    p.RelativeTime,
			ShowNumbers:     p.ShowNumbers,
		})
	})

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Fatalf("tui error: %v", err)
	}
}
