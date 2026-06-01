// Command jira-kanban displays a Jira board as an interactive terminal kanban.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/raulvc/jira-kanban/internal/config"
	"github.com/raulvc/jira-kanban/internal/jira"
	"github.com/raulvc/jira-kanban/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgFile, err := config.Path()
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config: %w", err)
	}

	parseBoardFlag(&cfg)

	if err := config.Ensure(&cfg, func() error {
		client := jira.NewClient(cfg.BaseURL, cfg.Email, cfg.Token)
		return client.Ping(cfg.BoardID)
	}); err != nil {
		return err
	}

	client := jira.NewClient(cfg.BaseURL, cfg.Email, cfg.Token)

	data, fromCache, err := client.FetchBoard(cfg.BoardID, printProgress)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return err
	}
	clearProgress()

	return ui.Run(client, cfg.BoardID, data, cfg.BaseURL, fromCache)
}

const barWidth = 30

func printProgress(p jira.Progress) {
	var label string
	if p.Cold {
		label = "First run — building cache"
	} else {
		label = "Syncing changes"
	}

	if p.Total == 0 {
		fmt.Fprintf(os.Stderr, "\r  %s…", label)
		return
	}

	pct := p.Fetched * 100 / p.Total
	filled := p.Fetched * barWidth / p.Total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Fprintf(os.Stderr, "\r  %s  [%s] %3d%%  %d/%d issues",
		label, bar, pct, p.Fetched, p.Total)
}

func clearProgress() {
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 80))
}

// parseBoardFlag applies --board or positional board ID overrides.
func parseBoardFlag(cfg *config.Config) {
	for i, arg := range os.Args[1:] {
		if arg == "--board" && i+1 < len(os.Args)-1 {
			if id, err := strconv.Atoi(os.Args[i+2]); err == nil {
				cfg.BoardID = id
			}
		} else if id, err := strconv.Atoi(arg); err == nil && id > 0 {
			cfg.BoardID = id
		}
	}
}
