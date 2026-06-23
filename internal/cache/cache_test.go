package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tmpCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

func TestLoadEmpty(t *testing.T) {
	tmpCacheDir(t)
	must := require.New(t)
	is := assert.New(t)
	s, err := Load(999)
	must.NoError(err)
	is.True(s.IsEmpty())
	is.Equal(999, s.BoardID)
}

func TestSaveAndLoad(t *testing.T) {
	tmpCacheDir(t)
	must := require.New(t)
	is := assert.New(t)
	s := Store{
		BoardID: 1,
		Issues:  map[string]Entry{},
	}
	now := time.Now().Truncate(time.Second)
	s.Merge([]Entry{
		{Key: "PROJ-1", Summary: "First", StatusID: "10", Status: "To Do", Assignee: "Alice", Labels: []string{"bug"}},
		{Key: "PROJ-2", Summary: "Second", StatusID: "20", Status: "Done", Assignee: "Unassigned"},
	}, now)

	must.NoError(s.Save())

	loaded, err := Load(1)
	must.NoError(err)
	is.False(loaded.IsEmpty())
	is.Len(loaded.Issues, 2)
	e := loaded.Issues["PROJ-1"]
	is.Equal("First", e.Summary)
	is.Equal("10", e.StatusID)
	is.Equal("Alice", e.Assignee)
	is.Len(e.Labels, 1)
	is.Equal("bug", e.Labels[0])
}

func TestMergeOverwrites(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "old", StatusID: "1", Status: "To Do"},
	}}
	s.Merge([]Entry{
		{Key: "X-1", Summary: "new", StatusID: "2", Status: "Done"},
	}, time.Now())

	is.Equal("new", s.Issues["X-1"].Summary)
	is.Equal("2", s.Issues["X-1"].StatusID)
}

func TestMergePreservesRank(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "old", Rank: "0|i0000a:"},
	}}
	s.Merge([]Entry{
		{Key: "X-1", Summary: "new", StatusID: "1", Status: "To Do"},
	}, time.Now())
	is.Equal("0|i0000a:", s.Issues["X-1"].Rank)
}

func TestMergePreservesAddedAt(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "old", AddedAt: "2024-01-01T00:00:00Z"},
	}}
	s.Merge([]Entry{
		{Key: "X-1", Summary: "new", StatusID: "1", Status: "To Do"},
	}, time.Now())
	is.Equal("2024-01-01T00:00:00Z", s.Issues["X-1"].AddedAt)
}

func TestMergeSetsAddedAtForNewEntry(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	s.Merge([]Entry{
		{Key: "X-1", Summary: "new", StatusID: "1", Status: "To Do"},
	}, time.Now())
	is.NotEmpty(s.Issues["X-1"].AddedAt)
}

func TestMergeNewRankOverridesOld(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "old", Rank: "0|i0000a:"},
	}}
	s.Merge([]Entry{
		{Key: "X-1", Summary: "new", StatusID: "1", Status: "To Do", Rank: "0|i0000z:"},
	}, time.Now())
	is.Equal("0|i0000z:", s.Issues["X-1"].Rank)
}

func TestUpdateStatus(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "task", StatusID: "1", Status: "To Do"},
	}}
	s.UpdateStatus("X-1", "2", "In Progress")
	e := s.Issues["X-1"]
	is.Equal("2", e.StatusID)
	is.Equal("In Progress", e.Status)
	is.Equal("task", e.Summary)
}

func TestUpdateStatusMissing(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	s.UpdateStatus("X-99", "2", "Done")
	is.Empty(s.Issues)
}

func TestUpdateAssignee(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{
		"X-1": {Key: "X-1", Summary: "task", StatusID: "1", Status: "To Do", Assignee: "Alice"},
	}}
	s.UpdateAssignee("X-1", "Bob")
	e := s.Issues["X-1"]
	is.Equal("Bob", e.Assignee)
	is.Equal("task", e.Summary)
}

func TestUpdateAssigneeMissing(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	s.UpdateAssignee("X-99", "Bob")
	is.Empty(s.Issues)
}

func TestUpsertEntry(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	s.UpsertEntry(Entry{Key: "X-1", Summary: "new", StatusID: "1", Status: "To Do"})
	is.Len(s.Issues, 1)
	is.Equal("new", s.Issues["X-1"].Summary)
	is.NotEmpty(s.Issues["X-1"].AddedAt)
}

func TestUpsertEntry_WithAddedAt(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	s.UpsertEntry(Entry{Key: "X-1", Summary: "new", AddedAt: "2024-01-01T00:00:00Z"})
	is.Equal("2024-01-01T00:00:00Z", s.Issues["X-1"].AddedAt)
}

func TestUpsertEntry_DoesNotAdvanceFetchedAt(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}, FetchedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	s.UpsertEntry(Entry{Key: "X-1", Summary: "new"})
	is.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), s.FetchedAt)
}

func TestSinceDuration(t *testing.T) {
	is := assert.New(t)
	s := Store{BoardID: 1, Issues: map[string]Entry{}}
	is.Equal(time.Duration(0), s.SinceDuration())
	s.FetchedAt = time.Now().Add(-5 * time.Minute)
	d := s.SinceDuration()
	is.True(d >= 4*time.Minute)
	is.True(d <= 6*time.Minute)
}

func TestLoadCorruptCache(t *testing.T) {
	must := require.New(t)
	is := assert.New(t)
	dir := tmpCacheDir(t)
	p := filepath.Join(dir, "jira-kanban")
	must.NoError(os.MkdirAll(p, 0o750))
	must.NoError(os.WriteFile(filepath.Join(p, "board-1.json"), []byte("not json{{{"), 0o600))
	s, err := Load(1)
	must.NoError(err, "corrupt cache should not error")
	is.True(s.IsEmpty(), "corrupt cache should return empty store")
}