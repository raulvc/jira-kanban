// Package config handles loading, saving, and interactively prompting
// for the application configuration stored in ~/.config/jira-kanban/config.yml.
package config

import (
	"bufio"
	"fmt"
	"net/url"
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
	Theme   string `yaml:"theme"`
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
// the result when changes are made. If verify is non-nil, it is called
// before saving; if it returns an error the user is re-prompted.
func Ensure(cfg *Config, verify func() error) error {
	for {
		changed, err := promptFields(cfg)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		if verify != nil {
			fmt.Fprintln(os.Stderr, "  Verifying credentials…")
			if err := verify(); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ %s\n", err)
				fmt.Fprintln(os.Stderr, "  Please re-enter your configuration.")
				*cfg = Config{}
				continue
			}
			fmt.Fprintln(os.Stderr, "  ✓ Connection successful")
		}
		p, err := Path()
		if err != nil {
			return err
		}
		if err := Save(p, *cfg); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Config saved to %s\n", p)
		return nil
	}
}

func promptFields(cfg *Config) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	changed := false

	if strings.TrimSpace(cfg.BaseURL) == "" {
		v, err := promptValidated(reader, "Jira base URL", "https://your-company.atlassian.net", false, validateURL)
		if err != nil {
			return false, err
		}
		cfg.BaseURL = v
		changed = true
	}

	if strings.TrimSpace(cfg.Email) == "" {
		v, err := promptValidated(reader, "Jira email", "you@company.com", false, validateEmail)
		if err != nil {
			return false, err
		}
		cfg.Email = v
		changed = true
	}

	if strings.TrimSpace(cfg.Token) == "" {
		v, err := prompt(reader, "Jira API token (https://id.atlassian.com/manage-profile/security/api-tokens)", "", true)
		if err != nil {
			return false, err
		}
		cfg.Token = v
		changed = true
	}

	if cfg.BoardID <= 0 {
		v, err := prompt(reader, "Board ID", "e.g. 123", false)
		if err != nil {
			return false, err
		}
		id, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return false, fmt.Errorf("invalid board ID %q: %w", v, err)
		}
		cfg.BoardID = id
		changed = true
	}

	return changed, nil
}

func validateURL(s string) (string, error) {
	s = strings.TrimRight(s, "/")
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("URL must start with http:// or https://")
	}
	return s, nil
}

func validateEmail(s string) (string, error) {
	if !strings.Contains(s, "@") || !strings.Contains(s[strings.Index(s, "@")+1:], ".") {
		return "", fmt.Errorf("invalid email address")
	}
	return s, nil
}

func prompt(reader *bufio.Reader, label, suggestion string, secret bool) (string, error) {
	return promptValidated(reader, label, suggestion, secret, nil)
}

func promptValidated(reader *bufio.Reader, label, suggestion string, secret bool, validate func(string) (string, error)) (string, error) {
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

		if value == "" {
			fmt.Fprintln(os.Stderr, "  Value is required.")
			continue
		}

		if validate != nil {
			clean, err := validate(value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s\n", err)
				continue
			}
			return clean, nil
		}
		return value, nil
	}
}