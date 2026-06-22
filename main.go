// Command jira-kanban displays a Jira board as an interactive terminal kanban.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/raulvc/jira-kanban/internal/config"
	"github.com/raulvc/jira-kanban/internal/debug"
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
	debugFlag := flag.Bool("debug", false, "enable verbose debug logging")
	boardFlag := flag.Int("board", 0, "board ID override")
	flag.Parse()

	logPath, err := debug.Init(*debugFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	} else if *debugFlag {
		fmt.Fprintf(os.Stderr, "  Debug log: %s\n", logPath)
	}

	cfgFile, err := config.Path()
	if err != nil {
		return err
	}

	cfg, err := config.Load(cfgFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config: %w", err)
	}

	// Apply board flag overrides: --board flag takes priority, then positional arg.
	if *boardFlag > 0 {
		cfg.BoardID = *boardFlag
	}
	parseBoardFlag(&cfg)

	if err := config.Ensure(&cfg, func() error {
		client := jira.NewClient(cfg.BaseURL, cfg.Email, cfg.Token)
		return client.Ping(cfg.BoardID)
	}); err != nil {
		return err
	}

	client := jira.NewClient(cfg.BaseURL, cfg.Email, cfg.Token)

	// Eagerly fetch account ID so sync can query recent activity.
	if me, err := client.GetCurrentUser(); err == nil {
		client.AccountID = me.AccountID
	}

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

// parseBoardFlag applies positional board ID overrides (when --board was not set).
func parseBoardFlag(cfg *config.Config) {
	for _, arg := range os.Args[1:] {
		if arg == "--board" || arg == "-board" {
			continue
		}
		if id, err := parsePositiveInt(arg); err == nil && id > 0 {
			cfg.BoardID = id
		}
	}
}

func parsePositiveInt(s string) (int, error) {
	// Skip flag-like args
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("not a positive int: %s", s)
	}
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a positive int: %s", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}