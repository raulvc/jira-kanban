// Package config handles loading, saving, and interactively prompting
// for the application configuration stored in ~/.config/jira-kanban/config.yml.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// Config holds the credentials and board selector needed to talk to Jira.
type Config struct {
	BaseURL string `yaml:"base_url"`
	Email   string `yaml:"email"`
	Token   string `yaml:"api_token"` //nolint:gosec // field name, not a hardcoded secret
	BoardID int    `yaml:"board_id"`
}

// Path returns the default configuration file location.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jira-kanban", "config.yml"), nil
}

// Load reads and unmarshals the YAML configuration at path.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path) //nolint:gosec // path is always derived from user home
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	return cfg, err
}

// Save marshals cfg and writes it to path, creating parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(&cfg) //nolint:gosec // Token is the field name, not a literal
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Ensure interactively prompts for any missing fields in cfg and persists
// the result when changes are made.
func Ensure(cfg *Config) error {
	reader := bufio.NewReader(os.Stdin)
	changed := false

	if strings.TrimSpace(cfg.BaseURL) == "" {
		v, err := prompt(reader, "Jira base URL", "https://your-company.atlassian.net", false)
		if err != nil {
			return err
		}
		cfg.BaseURL = strings.TrimRight(v, "/")
		changed = true
	}

	if strings.TrimSpace(cfg.Email) == "" {
		v, err := prompt(reader, "Jira email", "you@company.com", false)
		if err != nil {
			return err
		}
		cfg.Email = v
		changed = true
	}

	if strings.TrimSpace(cfg.Token) == "" {
		v, err := prompt(reader, "Jira API token (https://id.atlassian.com/manage-profile/security/api-tokens)", "", true)
		if err != nil {
			return err
		}
		cfg.Token = v
		changed = true
	}

	if cfg.BoardID <= 0 {
		v, err := prompt(reader, "Board ID", "e.g. 123", false)
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid board ID %q: %w", v, err)
		}
		cfg.BoardID = id
		changed = true
	}

	if changed {
		p, err := Path()
		if err != nil {
			return err
		}
		if err := Save(p, *cfg); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Config saved to %s\n", p)
	}
	return nil
}

func prompt(reader *bufio.Reader, label, suggestion string, secret bool) (string, error) {
	for {
		p := label
		if suggestion != "" {
			p += " (" + suggestion + ")"
		}
		fmt.Fprintf(os.Stderr, "%s: ", p)

		var value string
		if secret {
			raw, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // fd conversion is safe on all targets
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return "", err
			}
			value = strings.TrimSpace(string(raw))
		} else {
			v, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			value = strings.TrimSpace(v)
		}

		if value != "" {
			return value, nil
		}
		fmt.Fprintln(os.Stderr, "  Value is required.")
	}
}
