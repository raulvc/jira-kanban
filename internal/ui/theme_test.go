package ui

import (
	"path/filepath"
	"testing"

	"github.com/raulvc/jira-kanban/internal/config"
)

func TestSetThemeByName(t *testing.T) {
	orig := currentTheme
	t.Cleanup(func() { currentTheme = orig })

	SetThemeByName("Kanagawa Light")
	if T().Name != "Kanagawa Light" {
		t.Fatalf("got %q, want %q", T().Name, "Kanagawa Light")
	}
	if ThemeName() != "Kanagawa Light" {
		t.Fatalf("ThemeName() = %q, want %q", ThemeName(), "Kanagawa Light")
	}

	SetThemeByName("nonexistent")
	if T().Name != "Kanagawa Dark" {
		t.Fatalf("fallback got %q, want %q", T().Name, "Kanagawa Dark")
	}

	SetThemeByName("")
	if T().Name != "Kanagawa Dark" {
		t.Fatalf("empty fallback got %q, want %q", T().Name, "Kanagawa Dark")
	}
}

func TestCycleTheme(t *testing.T) {
	orig := currentTheme
	t.Cleanup(func() { currentTheme = orig })

	SetThemeByName("Kanagawa Dark")
	name := cycleTheme()
	if name != "Kanagawa Light" {
		t.Fatalf("first cycle got %q, want %q", name, "Kanagawa Light")
	}
	for i := 0; i < len(themes)-1; i++ {
		cycleTheme()
	}
	if T().Name != "Kanagawa Dark" {
		t.Fatalf("wrap-around got %q, want %q", T().Name, "Kanagawa Dark")
	}
}

func TestThemeRoundTripViaConfig(t *testing.T) {
	orig := currentTheme
	t.Cleanup(func() { currentTheme = orig })

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := config.Config{
		BaseURL: "https://example.atlassian.net",
		Email:   "user@example.com",
		APIToken: "secret-token",
		BoardID: 42,
		Theme:   "Darcula",
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != "Darcula" {
		t.Fatalf("Theme: got %q, want %q", loaded.Theme, "Darcula")
	}

	SetThemeByName(loaded.Theme)
	if T().Name != "Darcula" {
		t.Fatalf("after SetThemeByName got %q, want %q", T().Name, "Darcula")
	}
}