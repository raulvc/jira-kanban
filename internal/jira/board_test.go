package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raulvc/jira-kanban/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── test helpers ────────────────────────────────────────────────────────────

// fakeJira sets up an httptest.Server that responds to the Jira endpoints
// used by the client.  The caller provides handler functions per path pattern;
// unmatched routes return 404.
type fakeJira struct {
	server *httptest.Server
	mux    *http.ServeMux
}

func newFakeJira() *fakeJira {
	f := &fakeJira{mux: http.NewServeMux()}
	f.server = httptest.NewServer(f.mux)
	return f
}

func (f *fakeJira) close()      { f.server.Close() }

func (f *fakeJira) client() *Client {
	return &Client{
		BaseURL:    f.server.URL,
		Email:      "test@test.com",
		APIToken:   "tok",
		HTTPClient: f.server.Client(),
	}
}

func (f *fakeJira) handle(pattern string, fn http.HandlerFunc) {
	f.mux.HandleFunc(pattern, fn)
}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func tmpCache(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

// boardConfig returns a standard 3-column board configuration response
// (Backlog, To Do, In Progress, Done).
func boardConfigJSON() boardConfigResponse {
	return boardConfigResponse{
		Name: "Test Board",
		ColumnConfig: struct {
			Columns []struct {
				Name     string `json:"name"`
				Statuses []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"statuses"`
			} `json:"columns"`
		}{
			Columns: []struct {
				Name     string `json:"name"`
				Statuses []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"statuses"`
			}{
				{Name: "Backlog", Statuses: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{{ID: "1", Name: "Backlog"}}},
				{Name: "To Do", Statuses: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{{ID: "2", Name: "To Do"}}},
				{Name: "In Progress", Statuses: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{{ID: "3", Name: "In Progress"}}},
				{Name: "Done", Statuses: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{{ID: "4", Name: "Done"}}},
			},
		},
	}
}

func makeIssue(key, summary, statusID, statusName, assignee string) issue {
	iss := issue{Key: key}
	iss.Fields.Summary = summary
	iss.Fields.Status.ID = statusID
	iss.Fields.Status.Name = statusName
	if assignee != "" {
		iss.Fields.Assignee = &struct {
			DisplayName string `json:"displayName"`
		}{DisplayName: assignee}
	}
	return iss
}

// ── buildBoard tests ────────────────────────────────────────────────────────

func TestBuildBoard_SkipsBacklog(t *testing.T) {
	mappings := []columnMapping{
		{Name: "Backlog", StatusIDs: []string{"1"}, StatusNames: []string{"Backlog"}},
		{Name: "To Do", StatusIDs: []string{"2"}, StatusNames: []string{"To Do"}},
		{Name: "Done", StatusIDs: []string{"3"}, StatusNames: []string{"Done"}},
	}
	issues := []issue{
		makeIssue("P-2", "todo task", "2", "To Do", ""),
	}
	board := buildBoard("B", mappings, issues)

	is := assert.New(t)
	is.Len(board.Columns, 2, "expected 2 columns (To Do, Done)")
	for _, col := range board.Columns {
		is.NotEqual("Backlog", col.Name, "Backlog column should be skipped from mappings")
	}
	is.Equal("To Do", board.Columns[0].Name)
}

func TestBuildBoard_LastColumnCapped(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"1"}, StatusNames: []string{"To Do"}},
		{Name: "Done", StatusIDs: []string{"2"}, StatusNames: []string{"Done"}},
	}
	issues := make([]issue, 0, 30)
	for i := range 30 {
		issues = append(issues, makeIssue("P-"+itoa(i+1), "task", "2", "Done", ""))
	}
	board := buildBoard("B", mappings, issues)
	doneCol := board.Columns[len(board.Columns)-1]
	assert.Len(t, doneCol.Issues, 20, "last column should be capped at 20")
}

func TestBuildBoard_PreservesRankOrder(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"1"}, StatusNames: []string{"To Do"}},
		{Name: "Done", StatusIDs: []string{"2"}, StatusNames: []string{"Done"}},
	}
	issues := []issue{
		makeIssue("P-3", "c", "1", "To Do", ""),
		makeIssue("P-1", "a", "1", "To Do", ""),
		makeIssue("P-2", "b", "1", "To Do", ""),
	}
	issues[0].Rank = "0|i0000z:"
	issues[1].Rank = "0|i0000a:"
	issues[2].Rank = "0|i0000m:"
	board := buildBoard("B", mappings, issues)
	todo := board.Columns[0]
	is := assert.New(t)
	is.Equal("P-1", todo.Issues[0].Key)
	is.Equal("P-2", todo.Issues[1].Key)
	is.Equal("P-3", todo.Issues[2].Key)
}

func TestBuildBoard_LastColumnPreservesRankOrder(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"1"}, StatusNames: []string{"To Do"}},
		{Name: "Done", StatusIDs: []string{"2"}, StatusNames: []string{"Done"}},
	}
	issues := []issue{
		makeIssue("P-1", "a", "2", "Done", ""),
		makeIssue("P-3", "c", "2", "Done", ""),
		makeIssue("P-2", "b", "2", "Done", ""),
	}
	issues[0].Rank = "0|i0000m:"
	issues[1].Rank = "0|i0000a:"
	issues[2].Rank = "0|i0000z:"
	board := buildBoard("B", mappings, issues)
	done := board.Columns[len(board.Columns)-1]
	is := assert.New(t)
	is.Equal("P-3", done.Issues[0].Key)
	is.Equal("P-1", done.Issues[1].Key)
	is.Equal("P-2", done.Issues[2].Key)
}

func TestBuildBoard_ResolvesColumnByStatusID(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"10"}, StatusNames: []string{"To Do"}},
		{Name: "Done", StatusIDs: []string{"20"}, StatusNames: []string{"Done"}},
	}
	issues := []issue{
		makeIssue("P-1", "task", "10", "To Do", "Alice"),
	}
	board := buildBoard("B", mappings, issues)
	is := assert.New(t)
	is.Equal("P-1", board.Columns[0].Issues[0].Key)
	is.Equal("Alice", board.Columns[0].Issues[0].Assignee)
}

func TestBuildBoard_ResolvesColumnByStatusName(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"10"}, StatusNames: []string{"Open"}},
		{Name: "Done", StatusIDs: []string{"20"}, StatusNames: []string{"Closed"}},
	}
	issues := []issue{
		makeIssue("P-1", "task", "99", "Open", ""),
	}
	board := buildBoard("B", mappings, issues)
	is := assert.New(t)
	is.Equal("To Do", board.Columns[0].Name)
	is.Len(board.Columns[0].Issues, 1)
}

func TestBuildBoard_UnmappedStatusCreatesColumn(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"10"}, StatusNames: []string{"To Do"}},
	}
	issues := []issue{
		makeIssue("P-1", "task", "99", "Mystery", ""),
	}
	board := buildBoard("B", mappings, issues)
	is := assert.New(t)
	found := false
	for _, col := range board.Columns {
		if col.Name == "Mystery" {
			found = true
			is.Len(col.Issues, 1)
		}
	}
	is.True(found, "unmapped status should create a new column")
}

func TestVisibleKeys(t *testing.T) {
	board := Board{
		Name: "B",
		Columns: []Column{
			{Name: "A", Issues: []Card{{Key: "X-1"}, {Key: "X-2"}}},
			{Name: "B", Issues: []Card{{Key: "X-3"}}},
		},
	}
	keys := board.VisibleKeys()
	is := assert.New(t)
	is.Len(keys, 3)
	for _, k := range []string{"X-1", "X-2", "X-3"} {
		is.True(keys[k], "missing key %s", k)
	}
}

// ── pruneCache tests ────────────────────────────────────────────────────────

func TestPruneCache(t *testing.T) {
	store := cache.Store{BoardID: 1, Issues: map[string]cache.Entry{
		"P-1": {Key: "P-1"},
		"P-2": {Key: "P-2"},
		"P-3": {Key: "P-3"},
	}}
	board := Board{Columns: []Column{
		{Name: "To Do", Issues: []Card{{Key: "P-1"}}},
		{Name: "Done", Issues: []Card{{Key: "P-3"}}},
	}}
	pruneCache(&store, board)
	is := assert.New(t)
	is.Len(store.Issues, 2)
	is.NotContains(store.Issues, "P-2")
}

// ── visibleStatusIDs tests ──────────────────────────────────────────────────

func TestVisibleStatusIDs(t *testing.T) {
	cols := []columnMapping{
		{Name: "Backlog", StatusIDs: []string{"1"}},
		{Name: "To Do", StatusIDs: []string{"2", "3"}},
		{Name: "Done", StatusIDs: []string{"4"}},
	}
	ids := visibleStatusIDs(cols)
	assert.Len(t, ids, 3, "expected 3 visible status IDs")
}

// ── conversion tests ────────────────────────────────────────────────────────

func TestIssuesToEntriesRoundTrip(t *testing.T) {
	orig := []issue{
		makeIssue("P-1", "first", "10", "To Do", "Alice"),
		makeIssue("P-2", "second", "20", "Done", ""),
	}
	entries := issuesToEntries(orig)
	is := assert.New(t)
	is.Len(entries, 2)
	is.Equal("P-1", entries[0].Key)
	is.Equal("Alice", entries[0].Assignee)
	is.Equal("Unassigned", entries[1].Assignee)

	m := map[string]cache.Entry{}
	for _, e := range entries {
		m[e.Key] = e
	}
	back := entriesToIssues(m)
	is.Len(back, 2)
}

// ── FetchBoard tests (with fake server) ─────────────────────────────────────

func TestFetchBoard_ColdStart(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardConfigJSON())
	})
	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{
			Total: 2,
			Issues: []issue{
				makeIssue("P-1", "first", "2", "To Do", "Alice"),
				makeIssue("P-2", "second", "3", "In Progress", "Bob"),
			},
		})
	})

	var progressCalls int
	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	board, fromCache, err := c.FetchBoard(1, func(_ Progress) { progressCalls++ })
	must.NoError(err)
	is.False(fromCache, "cold start should not be from cache")
	is.Equal("Test Board", board.Name)
	is.Len(board.Columns, 3, "expected 3 columns (Backlog skipped)")
	is.True(progressCalls > 0, "progress callback should have been called")

	store, _ := cache.Load(1)
	is.False(store.IsEmpty(), "cache should be populated after cold start")
}

func TestFetchBoard_WarmStart(t *testing.T) {
	tmpCache(t)

	must := require.New(t)
	store := cache.Store{BoardID: 1, Issues: map[string]cache.Entry{
		"P-1": {Key: "P-1", Summary: "cached", StatusID: "2", Status: "To Do", Assignee: "Unassigned"},
	}, FetchedAt: time.Now()}
	must.NoError(store.Save())

	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardConfigJSON())
	})
	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("board issues endpoint should not be called on warm start")
		jsonResponse(w, boardIssuesResponse{})
	})

	c := fake.client()
	is := assert.New(t)
	board, fromCache, err := c.FetchBoard(1, nil)
	must.NoError(err)
	is.True(fromCache, "warm start should be from cache")
	is.Equal("cached", board.Columns[0].Issues[0].Summary)
}

// ── syncCache tests ─────────────────────────────────────────────────────────

func TestSyncCacheFull_NoTimestamp(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{
			Total: 2,
			Issues: []issue{
				makeIssue("P-1", "a", "2", "To Do", ""),
				makeIssue("P-2", "b", "3", "In Progress", ""),
			},
		})
	})
	fake.handle("POST /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				makeIssue("P-1", "a", "2", "To Do", ""),
				makeIssue("P-2", "b", "3", "In Progress", ""),
			},
		})
	})

	store := cache.Store{BoardID: 1, Issues: map[string]cache.Entry{}}
	c := fake.client()
	must := require.New(t)
	must.NoError(c.syncCacheFull(&store, 1, []string{"2", "3"}, nil))
	assert.Len(t, store.Issues, 2, "expected 2 cached issues")
}

func TestSyncCache_Incremental(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{
			Total: 1,
			Issues: []issue{
				makeIssue("P-1", "updated summary", "3", "In Progress", "Alice"),
			},
		})
	})

	fake.handle("POST /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				makeIssue("P-1", "", "3", "", ""),
				makeIssue("P-2", "", "2", "", ""),
			},
		})
	})

	store := cache.Store{
		BoardID:   1,
		FetchedAt: time.Now().Add(-10 * time.Minute),
		Issues: map[string]cache.Entry{
			"P-1": {Key: "P-1", Summary: "old summary", StatusID: "2", Status: "To Do"},
			"P-2": {Key: "P-2", Summary: "other", StatusID: "2", Status: "To Do"},
		},
	}
	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	must.NoError(c.syncCache(&store, 1, []string{"2", "3"}, nil))
	is.Equal("updated summary", store.Issues["P-1"].Summary)
	is.Equal("3", store.Issues["P-1"].StatusID)
	is.Equal("other", store.Issues["P-2"].Summary)
}

func TestSyncCache_RemovesInvisibleIssues(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{Total: 0, Issues: nil})
	})

	fake.handle("POST /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				makeIssue("P-1", "", "2", "", ""),
				makeIssue("P-2", "", "99", "", ""),
			},
		})
	})

	store := cache.Store{
		BoardID:   1,
		FetchedAt: time.Now().Add(-5 * time.Minute),
		Issues: map[string]cache.Entry{
			"P-1": {Key: "P-1", StatusID: "2"},
			"P-2": {Key: "P-2", StatusID: "2"},
		},
	}
	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	must.NoError(c.syncCache(&store, 1, []string{"2", "3"}, nil))
	is.NotContains(store.Issues, "P-2", "P-2 should be removed (status 99 not visible)")
	is.Contains(store.Issues, "P-1")
}

func TestSyncCache_RemovesDeletedIssues(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{Total: 0, Issues: nil})
	})

	fake.handle("POST /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				makeIssue("P-1", "", "2", "", ""),
			},
		})
	})

	store := cache.Store{
		BoardID:   1,
		FetchedAt: time.Now().Add(-5 * time.Minute),
		Issues: map[string]cache.Entry{
			"P-1": {Key: "P-1", StatusID: "2"},
			"P-2": {Key: "P-2", StatusID: "2"},
		},
	}
	c := fake.client()
	must := require.New(t)
	must.NoError(c.syncCache(&store, 1, []string{"2", "3"}, nil))
	assert.NotContains(t, store.Issues, "P-2", "P-2 should be removed (deleted)")
}

func TestSyncCache_ProgressReported(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{
			Total:  1,
			Issues: []issue{makeIssue("P-1", "x", "2", "To Do", "")},
		})
	})
	fake.handle("POST /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{makeIssue("P-1", "", "2", "", "")},
		})
	})

	store := cache.Store{
		BoardID:   1,
		FetchedAt: time.Now().Add(-5 * time.Minute),
		Issues:    map[string]cache.Entry{"P-1": {Key: "P-1", StatusID: "2"}},
	}

	var phases []string
	c := fake.client()
	must := require.New(t)
	must.NoError(c.syncCache(&store, 1, []string{"2"}, func(p SyncProgress) {
		phases = append(phases, p.Phase)
	}))
	assert.NotEmpty(t, phases, "progress should have been reported")
}

// ── RefreshBoard integration test ───────────────────────────────────────────

func TestRefreshBoard(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardConfigJSON())
	})
	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{
			Total: 1,
			Issues: []issue{
				makeIssue("P-1", "refreshed", "2", "To Do", ""),
			},
		})
	})
	fake.handle("POST /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{makeIssue("P-1", "", "2", "", "")},
		})
	})

	must := require.New(t)
	store := cache.Store{
		BoardID:   1,
		FetchedAt: time.Now().Add(-10 * time.Minute),
		Issues:    map[string]cache.Entry{"P-1": {Key: "P-1", Summary: "old", StatusID: "2", Status: "To Do", Assignee: "Unassigned"}},
	}
	must.NoError(store.Save())

	c := fake.client()
	is := assert.New(t)
	board, err := c.RefreshBoard(1, nil)
	must.NoError(err)
	is.Equal("Test Board", board.Name)

	saved, _ := cache.Load(1)
	is.False(saved.IsEmpty(), "cache should not be empty after refresh")
}

// ── UpdateCachedStatus test ─────────────────────────────────────────────────

func TestUpdateCachedStatus(t *testing.T) {
	tmpCache(t)

	must := require.New(t)
	is := assert.New(t)
	store := cache.Store{
		BoardID: 1,
		Issues: map[string]cache.Entry{
			"P-1": {Key: "P-1", Summary: "task", StatusID: "2", Status: "To Do", Assignee: "Unassigned"},
		},
	}
	must.NoError(store.Save())

	c := &Client{}
	c.UpdateCachedStatus(1, "P-1", "3", "In Progress")

	loaded, _ := cache.Load(1)
	e := loaded.Issues["P-1"]
	is.Equal("3", e.StatusID)
	is.Equal("In Progress", e.Status)
	is.Equal("task", e.Summary)
}

// ── helper ──────────────────────────────────────────────────────────────────

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func TestIssueNum(t *testing.T) {
	tests := []struct {
		key  string
		want int
	}{
		{"PROJ-1234", 1234},
		{"X-1", 1},
		{"noprefix", 0},
	}
	for _, tt := range tests {
		is := assert.New(t)
		is.Equal(tt.want, issueNum(tt.key))
	}
}

func TestEpicName(t *testing.T) {
	tests := []struct {
		name string
		iss  issue
		want string
	}{
		{
			"epic via Epic field",
			func() issue {
				iss := issue{Key: "P-1"}
				iss.Fields.Epic = &issueEpic{Key: "EP-1", Summary: "Platform Auth"}
				return iss
			}(),
			"Platform Auth",
		},
		{
			"epic via Parent field",
			func() issue {
				iss := issue{Key: "P-2"}
				iss.Fields.Parent = &issueParent{Key: "EP-2", Fields: struct{ Summary string `json:"summary"` }{Summary: "Migration"}}
				return iss
			}(),
			"Migration",
		},
		{
			"no epic",
			issue{Key: "P-3"},
			"",
		},
		{
			"epic Name fallback",
			func() issue {
				iss := issue{Key: "P-4"}
				iss.Fields.Epic = &issueEpic{Name: "Epic Name Only"}
				return iss
			}(),
			"Epic Name Only",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			is.Equal(tt.want, epicName(tt.iss))
		})
	}
}