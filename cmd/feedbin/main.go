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

	repo, err := storage.NewRepository(cfg.DBPath)
	if err != nil {
		log.Fatalf("storage init error: %v", err)
	}
	defer repo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := repo.Init(ctx); err != nil {
		log.Fatalf("storage schema error: %v", err)
	}

	client := feedbin.NewClient(cfg.APIBaseURL, cfg.Email, cfg.Password, nil)
	service := app.NewService(client, repo)

	entries, err := service.Refresh(ctx, 1, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh failed (%v), loading cached entries\n", err)
		entries, err = service.ListCached(ctx, 50)
		if err != nil {
			log.Fatalf("cannot load entries: %v", err)
		}
	}

	program := tea.NewProgram(tui.NewModel(service, entries), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Fatalf("tui error: %v", err)
	}
}
