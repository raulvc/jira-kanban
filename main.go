// Command jira-kanban displays a Jira board as an interactive terminal kanban.
package main

import (
	"fmt"
	"os"
	"strconv"

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

	if err := config.Ensure(&cfg); err != nil {
		return err
	}

	client := jira.NewClient(cfg.BaseURL, cfg.Email, cfg.Token)

	fmt.Fprintln(os.Stderr, "Loading board…")
	data, err := client.FetchBoard(cfg.BoardID)
	if err != nil {
		return err
	}

	return ui.Run(client, cfg.BoardID, data, cfg.BaseURL)
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
