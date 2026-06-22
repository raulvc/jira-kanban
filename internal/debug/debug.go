// Package debug provides file-based logging for jira-kanban.
// Logs are always written to $XDG_STATE_HOME/jira-kanban/debug.log
// (or the OS-equivalent).  With --debug, the log level is lowered to DEBUG
// and the path is printed on startup.
package debug

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

var logPath string

// Init sets up slog to write to a log file.  When verbose is true the
// log level is lowered to DEBUG and the log path is printed to stderr.
func Init(verbose bool) (string, error) {
	p, err := resolveLogPath()
	if err != nil {
		return "", err
	}
	logPath = filepath.Clean(p)

	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return "", fmt.Errorf("log dir: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", fmt.Errorf("log file: %w", err)
	}

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
	slog.Info("logging started", "path", logPath, "level", level)
	return logPath, nil
}

// LogPath returns the configured log file path.
func LogPath() string { return logPath }

// resolveLogPath returns the log file location using the XDG state directory.
//
//   - Linux/BSD: $XDG_STATE_HOME/jira-kanban/debug.log
//     (defaults to ~/.local/state/jira-kanban/debug.log)
//   - macOS: ~/Library/Logs/jira-kanban/debug.log
//   - Windows: %LOCALAPPDATA%/jira-kanban/debug.log
func resolveLogPath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "jira-kanban", "debug.log"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Logs", "jira-kanban", "debug.log"), nil
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "jira-kanban", "debug.log"), nil
		}
		return filepath.Join(home, "AppData", "Local", "jira-kanban", "debug.log"), nil
	default:
		return filepath.Join(home, ".local", "state", "jira-kanban", "debug.log"), nil
	}
}