package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := Config{
		BaseURL:  "https://example.atlassian.net",
		Email:    "user@example.com",
		APIToken: "secret-token",
		BoardID:  42,
		Theme:    "Kanagawa Light",
	}
	must := require.New(t)
	is := assert.New(t)
	must.NoError(Save(path, cfg))

	loaded, err := Load(path)
	must.NoError(err)
	is.Equal(cfg.BaseURL, loaded.BaseURL)
	is.Equal(cfg.Email, loaded.Email)
	is.Equal(cfg.APIToken, loaded.APIToken)
	is.Equal(42, loaded.BoardID)
	is.Equal("Kanagawa Light", loaded.Theme)
}

func TestLoad_NotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	assert.Error(t, err, "expected error for missing file")
}

func TestPath(t *testing.T) {
	must := require.New(t)
	is := assert.New(t)
	p, err := Path()
	must.NoError(err)
	is.NotEmpty(p)
	is.Equal("config.yml", filepath.Base(p))
}

func TestValidateURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid https", input: "https://example.atlassian.net", want: "https://example.atlassian.net"},
		{name: "valid http", input: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "trailing slash stripped", input: "https://example.com/", want: "https://example.com"},
		{name: "invalid scheme", input: "ftp://example.com", wantErr: true},
		{name: "no scheme", input: "example.com", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			got, err := validateURL(tt.input)
			if tt.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
			is.Equal(tt.want, got)
		})
	}
}

func TestValidateEmail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid email", input: "user@example.com"},
		{name: "no @", input: "userexample.com", wantErr: true},
		{name: "no dot after @", input: "user@example", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "subdomain", input: "a@b.co.uk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			_, err := validateEmail(tt.input)
			if tt.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
		})
	}
}