package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tmpCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

func TestLoadEmpty(t *testing.T) {
	tmpCacheDir(t)
	s, err := Load(999)
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsEmpty() {
		t.Fatal("expected empty store")
	}
	if s.BoardID != 999 {
		t.Fatalf("expected board ID 999, got %d", s.BoardID)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpCacheDir(t)
	s := Store{
		BoardID: 1,
		Issues:  map[string]Entry{},
	}
	now := time.Now().Truncate(time.Second)
	s.Merge([]Entry{
		{Key: "PROJ-1", Summary: "First", StatusID: "10", Status: "To Do", Assignee: "Alice", Labels: []string{"bug"}},
		{Key: "PROJ-2", Summary: "Second", StatusID: "20", Status: "Done", Assignee: "Unassigned"},
	}, now)

	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(1)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IsEmpty() {
		t.Fatal("loaded store should not be empty")
	}
	if len(loaded.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(loaded.Issues))
	}
	e := loaded.Issues["PROJ-1"]
	if e.Summary != "First" || e.StatusID != "10" || e.Assignee != "Alice" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if len(e.Labels) != 1 || e.Labels[0] != "bug" {
		t.Fatalf("expected labels [bug], got %v", e.Labels)
	}
}

func TestMergeOverwrites(t *testing.T) {
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "old", StatusID: "1", Status: "To Do"},
	}}
	s.Merge([]Entry{
		{Key: "X-1", Summary: "new", StatusID: "2", Status: "Done"},
	}, time.Now())

	if s.Issues["X-1"].Summary != "new" {
		t.Fatal("Merge should overwrite existing entry")
	}
	if s.Issues["X-1"].StatusID != "2" {
		t.Fatal("StatusID should be updated")
	}
}

func TestUpdateStatus(t *testing.T) {
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "task", StatusID: "1", Status: "To Do"},
	}}
	s.UpdateStatus("X-1", "2", "In Progress")
	e := s.Issues["X-1"]
	if e.StatusID != "2" || e.Status != "In Progress" {
		t.Fatalf("UpdateStatus failed: %+v", e)
	}
	if e.Summary != "task" {
		t.Fatal("UpdateStatus should preserve other fields")
	}
}

func TestUpdateStatusMissing(t *testing.T) {
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	s.UpdateStatus("X-99", "2", "Done")
	if len(s.Issues) != 0 {
		t.Fatal("UpdateStatus on missing key should be a no-op")
	}
}

func TestSinceDuration(t *testing.T) {
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	if s.SinceDuration() != 0 {
		t.Fatal("zero FetchedAt should return 0 duration")
	}
	s.FetchedAt = time.Now().Add(-5 * time.Minute)
	d := s.SinceDuration()
	if d < 4*time.Minute || d > 6*time.Minute {
		t.Fatalf("expected ~5m, got %s", d)
	}
}

func TestLoadCorruptCache(t *testing.T) {
	dir := tmpCacheDir(t)
	p := filepath.Join(dir, "jira-kanban")
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "board-1.json"), []byte("not json{{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(1)
	if err != nil {
		t.Fatal("corrupt cache should not error")
	}
	if !s.IsEmpty() {
		t.Fatal("corrupt cache should return empty store")
	}
}
