package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/raulvc/jira-kanban/internal/fuzzy"
	"github.com/raulvc/jira-kanban/internal/jira"
)

func TestNewSearchState_Local(t *testing.T) {
	s := newSearchState(searchLocal)
	is := assert.New(t)
	is.Equal(searchLocal, s.mode)
	is.Equal("", s.query)
	is.Nil(s.results)
	is.Equal(-1, s.selected)
	is.False(s.loading)
}

func TestNewSearchState_Global(t *testing.T) {
	s := newSearchState(searchGlobal)
	is := assert.New(t)
	is.Equal(searchGlobal, s.mode)
}

func TestSearchState_TypeRune(t *testing.T) {
	s := newSearchState(searchLocal)
	s.typeRune('h')
	s.typeRune('i')
	is := assert.New(t)
	is.Equal("hi", s.query)
	is.Equal(0, s.selected, "typing should reset selection to 0")
}

func TestSearchState_Backspace(t *testing.T) {
	s := newSearchState(searchLocal)
	s.query = "hello"
	s.backspace()
	is := assert.New(t)
	is.Equal("hell", s.query)
	s.backspace()
	s.backspace()
	s.backspace()
	s.backspace()
	s.backspace()
	is.Equal("", s.query, "backspace on empty query should be a no-op")
}

func TestSearchState_MoveSelection(t *testing.T) {
	s := newSearchState(searchLocal)
	s.results = []fuzzy.Result{
		{Key: "P-1", Summary: "one"},
		{Key: "P-2", Summary: "two"},
		{Key: "P-3", Summary: "three"},
	}
	is := assert.New(t)

	s.selected = 0
	s.moveSelection(1)
	is.Equal(1, s.selected)

	s.moveSelection(1)
	is.Equal(2, s.selected)

	s.moveSelection(1)
	is.Equal(2, s.selected, "should clamp at bottom")

	s.moveSelection(-1)
	s.moveSelection(-1)
	s.moveSelection(-1)
	is.Equal(0, s.selected, "should clamp at top")
}

func TestSearchState_MoveSelectionNoResults(t *testing.T) {
	s := newSearchState(searchLocal)
	s.moveSelection(1)
	is := assert.New(t)
	is.Equal(-1, s.selected, "moveSelection with no results should be a no-op")
}

func TestSearchState_BackspaceResetsSelection(t *testing.T) {
	s := newSearchState(searchLocal)
	s.query = "test"
	s.selected = 3
	s.backspace()
	is := assert.New(t)
	is.Equal(0, s.selected, "backspace should reset selection to 0")
}

func TestSearchState_TypeRuneResetsSelection(t *testing.T) {
	s := newSearchState(searchLocal)
	s.selected = 5
	s.typeRune('x')
	is := assert.New(t)
	is.Equal(0, s.selected, "typing should reset selection to 0")
}

func TestSearchState_SwitchToGlobal(t *testing.T) {
	s := newSearchState(searchLocal)
	s.query = "test"
	s.results = []fuzzy.Result{{Key: "P-1"}}
	s.selected = 0

	ctx := &appContext{state: &boardState{search: s}}
	switchToGlobalSearch(ctx)

	is := assert.New(t)
	is.Equal(searchGlobal, s.mode)
	is.Equal("", s.query)
	is.Nil(s.results)
	is.Nil(s.gresults)
	is.Equal(-1, s.selected)
}

func TestSearchState_SwitchToLocalViaTab(t *testing.T) {
	s := newSearchState(searchGlobal)
	s.query = "jql test"
	s.gresults = []jira.SearchResult{{Key: "P-1"}}
	s.selected = 0

	// Simulate Tab key in global mode switches back to local
	s.mode = searchLocal
	s.query = ""
	s.gresults = nil
	s.results = nil
	s.selected = -1
	s.loading = false
	s.err = ""

	is := assert.New(t)
	is.Equal(searchLocal, s.mode)
	is.Equal("", s.query)
	is.Equal(-1, s.selected)
}

func TestSearchState_ResultCount_Local(t *testing.T) {
	s := newSearchState(searchLocal)
	s.results = []fuzzy.Result{{Key: "P-1"}, {Key: "P-2"}}
	is := assert.New(t)
	is.Equal(2, s.resultCount())
}

func TestSearchState_ResultCount_Global(t *testing.T) {
	s := newSearchState(searchGlobal)
	s.gresults = []jira.SearchResult{{Key: "P-1"}, {Key: "P-2"}, {Key: "P-3"}}
	is := assert.New(t)
	is.Equal(3, s.resultCount())
}

func TestExecuteLocalSearch_RespectsMemberFilter(t *testing.T) {
	board := jira.Board{
		Name: "Test",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "fix login", Assignee: "Alice"},
				{Key: "P-2", Summary: "fix login", Assignee: "Bob"},
			}},
		},
	}
	s := newBoardState(board)
	s.memberFilter = "Alice"
	s.search = newSearchState(searchLocal)
	s.search.query = "login"
	ctx := &appContext{state: s}

	executeLocalSearch(ctx)

	is := assert.New(t)
	is.Len(s.search.results, 1, "should only search filtered cards (Alice's)")
	is.Equal("P-1", s.search.results[0].Key)
}

func TestExecuteLocalSearch_RespectsEpicFilter(t *testing.T) {
	board := jira.Board{
		Name: "Test",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "fix login", Epic: "EpicA"},
				{Key: "P-2", Summary: "fix login", Epic: "EpicB"},
			}},
		},
	}
	s := newBoardState(board)
	s.epicFilterVal = "EpicA"
	s.search = newSearchState(searchLocal)
	s.search.query = "login"
	ctx := &appContext{state: s}

	executeLocalSearch(ctx)

	is := assert.New(t)
	is.Len(s.search.results, 1, "should only search epic-filtered cards")
	is.Equal("P-1", s.search.results[0].Key)
}

func TestExecuteLocalSearch_NoFilterSearchesAll(t *testing.T) {
	board := jira.Board{
		Name: "Test",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "fix login", Assignee: "Alice"},
				{Key: "P-2", Summary: "fix login", Assignee: "Bob"},
			}},
		},
	}
	s := newBoardState(board)
	s.search = newSearchState(searchLocal)
	s.search.query = "login"
	ctx := &appContext{state: s}

	executeLocalSearch(ctx)

	is := assert.New(t)
	is.Len(s.search.results, 2, "without filters should search all cards")
}
