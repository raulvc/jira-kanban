package ui

import (
	"testing"

	"github.com/raulvc/jira-kanban/internal/jira"
)

func TestNewFilterState(t *testing.T) {
	data := jira.Board{Columns: []jira.Column{
		{Name: "To Do", Issues: []jira.Card{
			{Key: "P-1", Assignee: "Alice"},
			{Key: "P-2", Assignee: "Bob"},
			{Key: "P-3", Assignee: "Alice"},
		}},
		{Name: "Done", Issues: []jira.Card{
			{Key: "P-4", Assignee: "Unassigned"},
			{Key: "P-5"},
		}},
	}}
	f := newFilterState(data)
	if len(f.members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(f.members))
	}
	if f.members[0] != "Alice" || f.members[1] != "Bob" {
		t.Fatalf("expected sorted [Alice, Bob], got %v", f.members)
	}
}

func TestFilterState_FilteredNoQuery(t *testing.T) {
	f := &filterState{members: []string{"Alice", "Bob", "Carol"}}
	out := f.filtered()
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}
}

func TestFilterState_FilteredSubstring(t *testing.T) {
	f := &filterState{members: []string{"Alice", "Bob", "Carol"}}
	f.query = "ali"
	out := f.filtered()
	if len(out) != 1 || out[0] != "Alice" {
		t.Fatalf("expected [Alice], got %v", out)
	}
}

func TestFilterState_FilteredNoMatch(t *testing.T) {
	f := &filterState{members: []string{"Alice", "Bob"}}
	f.query = "zzz"
	out := f.filtered()
	if len(out) != 0 {
		t.Fatalf("expected 0, got %d", len(out))
	}
}

func TestFilterState_TypeRune(t *testing.T) {
	f := &filterState{members: []string{"Alice", "Bob"}}
	f.typeRune('a')
	if f.query != "a" {
		t.Fatalf("expected 'a', got %q", f.query)
	}
	if f.selected != 0 {
		t.Fatal("selected should reset to 0")
	}
}

func TestFilterState_Backspace(t *testing.T) {
	f := &filterState{members: []string{"Alice"}, query: "ab", selected: 0}
	f.backspace()
	if f.query != "a" {
		t.Fatalf("expected 'a', got %q", f.query)
	}
}

func TestFilterState_BackspaceEmpty(t *testing.T) {
	f := &filterState{members: []string{"Alice"}}
	f.backspace()
	if f.query != "" {
		t.Fatalf("expected empty, got %q", f.query)
	}
}

func TestFilterState_MoveSelection(t *testing.T) {
	f := &filterState{members: []string{"A", "B", "C"}}
	f.moveSelection(1)
	if f.selected != 1 {
		t.Fatalf("expected 1, got %d", f.selected)
	}
	f.moveSelection(1)
	if f.selected != 2 {
		t.Fatalf("expected 2, got %d", f.selected)
	}
	f.moveSelection(1)
	if f.selected != 2 {
		t.Fatalf("should clamp at last, got %d", f.selected)
	}
	f.moveSelection(-1)
	if f.selected != 1 {
		t.Fatalf("expected 1, got %d", f.selected)
	}
}

func TestFilteredData_NoFilter(t *testing.T) {
	s := newBoardState(jira.Board{Columns: []jira.Column{
		{Name: "A", Issues: []jira.Card{{Key: "P-1", Assignee: "Alice"}}},
	}})
	fd := s.filteredData()
	if len(fd.Columns[0].Issues) != 1 {
		t.Fatal("should return all cards when no filter")
	}
}

func TestFilteredData_WithFilter(t *testing.T) {
	s := newBoardState(jira.Board{Columns: []jira.Column{
		{Name: "A", Issues: []jira.Card{
			{Key: "P-1", Assignee: "Alice"},
			{Key: "P-2", Assignee: "Bob"},
			{Key: "P-3", Assignee: "Alice"},
		}},
		{Name: "B", Issues: []jira.Card{
			{Key: "P-4", Assignee: "Bob"},
		}},
	}})
	s.memberFilter = "Alice"
	fd := s.filteredData()
	if len(fd.Columns[0].Issues) != 2 {
		t.Fatalf("expected 2 Alice cards in col A, got %d", len(fd.Columns[0].Issues))
	}
	if len(fd.Columns[1].Issues) != 0 {
		t.Fatalf("expected 0 Alice cards in col B, got %d", len(fd.Columns[1].Issues))
	}
}

func TestFilteredData_EmptyColumnsPreserved(t *testing.T) {
	s := newBoardState(jira.Board{Columns: []jira.Column{
		{Name: "A", Issues: []jira.Card{{Key: "P-1", Assignee: "Alice"}}},
		{Name: "B", Issues: []jira.Card{}},
	}})
	s.memberFilter = "Alice"
	fd := s.filteredData()
	if len(fd.Columns) != 2 {
		t.Fatal("all columns should be preserved even if empty")
	}
	if len(fd.Columns[1].Issues) != 0 {
		t.Fatal("column B should have no Alice cards")
	}
}