package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raulvc/jira-kanban/internal/cache"
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
		Token:      "tok",
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
	// Only include issues with visible statuses — backlog issues are
	// excluded by the JQL status filter before buildBoard is called.
	issues := []issue{
		makeIssue("P-2", "todo task", "2", "To Do", ""),
	}
	board := buildBoard("B", mappings, issues)

	if len(board.Columns) != 2 {
		t.Fatalf("expected 2 columns (To Do, Done), got %d", len(board.Columns))
	}
	for _, col := range board.Columns {
		if col.Name == "Backlog" {
			t.Fatal("Backlog column should be skipped from mappings")
		}
	}
	if board.Columns[0].Name != "To Do" {
		t.Fatalf("first column should be To Do, got %q", board.Columns[0].Name)
	}
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
	if len(doneCol.Issues) != 20 {
		t.Fatalf("last column should be capped at 20, got %d", len(doneCol.Issues))
	}
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
	if todo.Issues[0].Key != "P-1" || todo.Issues[1].Key != "P-2" || todo.Issues[2].Key != "P-3" {
		t.Fatalf("issues with rank should sort by rank: %v", []string{todo.Issues[0].Key, todo.Issues[1].Key, todo.Issues[2].Key})
	}
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
	if done.Issues[0].Key != "P-3" || done.Issues[1].Key != "P-1" || done.Issues[2].Key != "P-2" {
		t.Fatalf("last column with rank should sort by rank: %v", []string{done.Issues[0].Key, done.Issues[1].Key, done.Issues[2].Key})
	}
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
	if board.Columns[0].Issues[0].Key != "P-1" {
		t.Fatal("issue should be in To Do column via status ID mapping")
	}
	if board.Columns[0].Issues[0].Assignee != "Alice" {
		t.Fatal("assignee should be preserved")
	}
}

func TestBuildBoard_ResolvesColumnByStatusName(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"10"}, StatusNames: []string{"Open"}},
		{Name: "Done", StatusIDs: []string{"20"}, StatusNames: []string{"Closed"}},
	}
	// Issue has status ID "99" which doesn't match any column,
	// but status name "Open" should fall back to name-based mapping.
	issues := []issue{
		makeIssue("P-1", "task", "99", "Open", ""),
	}
	board := buildBoard("B", mappings, issues)
	if board.Columns[0].Name != "To Do" {
		t.Fatal("first column should be To Do")
	}
	if len(board.Columns[0].Issues) != 1 {
		t.Fatal("issue should be placed in To Do via status name fallback")
	}
}

func TestBuildBoard_UnmappedStatusCreatesColumn(t *testing.T) {
	mappings := []columnMapping{
		{Name: "To Do", StatusIDs: []string{"10"}, StatusNames: []string{"To Do"}},
	}
	// Issue with completely unknown status
	issues := []issue{
		makeIssue("P-1", "task", "99", "Mystery", ""),
	}
	board := buildBoard("B", mappings, issues)
	// Should create a new column named "Mystery"
	found := false
	for _, col := range board.Columns {
		if col.Name == "Mystery" {
			found = true
			if len(col.Issues) != 1 {
				t.Fatal("Mystery column should have 1 issue")
			}
		}
	}
	if !found {
		t.Fatal("unmapped status should create a new column")
	}
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
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	for _, k := range []string{"X-1", "X-2", "X-3"} {
		if !keys[k] {
			t.Fatalf("missing key %s", k)
		}
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
	if len(store.Issues) != 2 {
		t.Fatalf("expected 2 entries after prune, got %d", len(store.Issues))
	}
	if _, ok := store.Issues["P-2"]; ok {
		t.Fatal("P-2 should have been pruned")
	}
}

// ── visibleStatusIDs tests ──────────────────────────────────────────────────

func TestVisibleStatusIDs(t *testing.T) {
	cols := []columnMapping{
		{Name: "Backlog", StatusIDs: []string{"1"}},
		{Name: "To Do", StatusIDs: []string{"2", "3"}},
		{Name: "Done", StatusIDs: []string{"4"}},
	}
	ids := visibleStatusIDs(cols)
	if len(ids) != 3 {
		t.Fatalf("expected 3 visible status IDs, got %d", len(ids))
	}
}

// ── conversion tests ────────────────────────────────────────────────────────

func TestIssuesToEntriesRoundTrip(t *testing.T) {
	orig := []issue{
		makeIssue("P-1", "first", "10", "To Do", "Alice"),
		makeIssue("P-2", "second", "20", "Done", ""),
	}
	entries := issuesToEntries(orig)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Key != "P-1" || entries[0].Assignee != "Alice" {
		t.Fatalf("unexpected entry 0: %+v", entries[0])
	}
	if entries[1].Assignee != "Unassigned" {
		t.Fatalf("nil assignee should become Unassigned, got %q", entries[1].Assignee)
	}

	// Round-trip back
	m := map[string]cache.Entry{}
	for _, e := range entries {
		m[e.Key] = e
	}
	back := entriesToIssues(m)
	if len(back) != 2 {
		t.Fatalf("round-trip produced %d issues", len(back))
	}
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
	board, fromCache, err := c.FetchBoard(1, func(_ Progress) { progressCalls++ })
	if err != nil {
		t.Fatal(err)
	}
	if fromCache {
		t.Fatal("cold start should not be from cache")
	}
	if board.Name != "Test Board" {
		t.Fatalf("expected board name 'Test Board', got %q", board.Name)
	}
	// Backlog skipped → 3 columns (To Do, In Progress, Done)
	if len(board.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(board.Columns))
	}
	if progressCalls == 0 {
		t.Fatal("progress callback should have been called")
	}

	// Cache should now be populated
	store, _ := cache.Load(1)
	if store.IsEmpty() {
		t.Fatal("cache should be populated after cold start")
	}
}

func TestFetchBoard_WarmStart(t *testing.T) {
	tmpCache(t)

	// Pre-populate cache
	store := cache.Store{BoardID: 1, Issues: map[string]cache.Entry{
		"P-1": {Key: "P-1", Summary: "cached", StatusID: "2", Status: "To Do", Assignee: "Unassigned"},
	}, FetchedAt: time.Now()}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardConfigJSON())
	})
	// The board issues endpoint should NOT be called on warm start.
	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("board issues endpoint should not be called on warm start")
		jsonResponse(w, boardIssuesResponse{})
	})

	c := fake.client()
	board, fromCache, err := c.FetchBoard(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !fromCache {
		t.Fatal("warm start should be from cache")
	}
	if board.Columns[0].Issues[0].Summary != "cached" {
		t.Fatal("should use cached data")
	}
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
	err := c.syncCacheFull(&store, 1, []string{"2", "3"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Issues) != 2 {
		t.Fatalf("expected 2 cached issues, got %d", len(store.Issues))
	}
}

func TestSyncCache_Incremental(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	// fetchChangedIssues endpoint — returns one changed issue
	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{
			Total: 1,
			Issues: []issue{
				makeIssue("P-1", "updated summary", "3", "In Progress", "Alice"),
			},
		})
	})

	// fetchKeyStatuses endpoint — both keys still exist
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
	err := c.syncCache(&store, 1, []string{"2", "3"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if store.Issues["P-1"].Summary != "updated summary" {
		t.Fatalf("P-1 should be updated, got %q", store.Issues["P-1"].Summary)
	}
	if store.Issues["P-1"].StatusID != "3" {
		t.Fatal("P-1 status should be updated")
	}
	if store.Issues["P-2"].Summary != "other" {
		t.Fatal("P-2 should be unchanged")
	}
}

func TestSyncCache_RemovesInvisibleIssues(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	// No changed issues
	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{Total: 0, Issues: nil})
	})

	// fetchKeyStatuses — P-2 moved to status "99" (not visible)
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
	err := c.syncCache(&store, 1, []string{"2", "3"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Issues["P-2"]; ok {
		t.Fatal("P-2 should be removed (status 99 not visible)")
	}
	if _, ok := store.Issues["P-1"]; !ok {
		t.Fatal("P-1 should remain")
	}
}

func TestSyncCache_RemovesDeletedIssues(t *testing.T) {
	tmpCache(t)
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/issue", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardIssuesResponse{Total: 0, Issues: nil})
	})

	// fetchKeyStatuses — P-2 no longer exists
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
	err := c.syncCache(&store, 1, []string{"2", "3"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Issues["P-2"]; ok {
		t.Fatal("P-2 should be removed (deleted)")
	}
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
	err := c.syncCache(&store, 1, []string{"2"}, func(p SyncProgress) {
		phases = append(phases, p.Phase)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) == 0 {
		t.Fatal("progress should have been reported")
	}
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

	// Pre-populate cache with timestamp
	store := cache.Store{
		BoardID:   1,
		FetchedAt: time.Now().Add(-10 * time.Minute),
		Issues:    map[string]cache.Entry{"P-1": {Key: "P-1", Summary: "old", StatusID: "2", Status: "To Do", Assignee: "Unassigned"}},
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	c := fake.client()
	board, err := c.RefreshBoard(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if board.Name != "Test Board" {
		t.Fatalf("wrong board name: %q", board.Name)
	}

	// Verify cache was pruned and saved
	saved, _ := cache.Load(1)
	if saved.IsEmpty() {
		t.Fatal("cache should not be empty after refresh")
	}
}

// ── UpdateCachedStatus test ─────────────────────────────────────────────────

func TestUpdateCachedStatus(t *testing.T) {
	tmpCache(t)

	store := cache.Store{
		BoardID: 1,
		Issues: map[string]cache.Entry{
			"P-1": {Key: "P-1", Summary: "task", StatusID: "2", Status: "To Do", Assignee: "Unassigned"},
		},
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	c := &Client{}
	c.UpdateCachedStatus(1, "P-1", "3", "In Progress")

	loaded, _ := cache.Load(1)
	e := loaded.Issues["P-1"]
	if e.StatusID != "3" || e.Status != "In Progress" {
		t.Fatalf("cache not updated: %+v", e)
	}
	if e.Summary != "task" {
		t.Fatal("other fields should be preserved")
	}
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
		if got := issueNum(tt.key); got != tt.want {
			t.Errorf("issueNum(%q) = %d, want %d", tt.key, got, tt.want)
		}
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
			if got := epicName(tt.iss); got != tt.want {
				t.Errorf("epicName() = %q, want %q", got, tt.want)
			}
		})
	}
}
