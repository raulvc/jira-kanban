package config

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := Config{
		BaseURL: "https://example.atlassian.net",
		Email:   "user@example.com",
		Token:   "secret-token",
		BoardID: 42,
		Theme:   "Kanagawa Light",
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.BaseURL != cfg.BaseURL {
		t.Fatalf("BaseURL: got %q, want %q", loaded.BaseURL, cfg.BaseURL)
	}
	if loaded.Email != cfg.Email {
		t.Fatalf("Email: got %q, want %q", loaded.Email, cfg.Email)
	}
	if loaded.Token != cfg.Token {
		t.Fatalf("Token mismatch")
	}
	if loaded.BoardID != 42 {
		t.Fatalf("BoardID: got %d, want 42", loaded.BoardID)
	}
	if loaded.Theme != "Kanagawa Light" {
		t.Fatalf("Theme: got %q, want %q", loaded.Theme, "Kanagawa Light")
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPath(t *testing.T) {
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if p == "" {
		t.Fatal("path should not be empty")
	}
	if filepath.Base(p) != "config.yml" {
		t.Fatalf("expected config.yml, got %s", filepath.Base(p))
	}
}
