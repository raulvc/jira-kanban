package ui

import (
	"testing"

	"github.com/raulvc/jira-kanban/internal/jira"
)

func testModal() *modalState {
	return &modalState{
		issueKey: "P-1",
		allTransitions: []jira.Transition{
			{ID: "1", Name: "Start Progress → In Progress", ToStatusID: "3", ToStatus: "In Progress"},
			{ID: "2", Name: "Done", ToStatusID: "4", ToStatus: "Done"},
			{ID: "3", Name: "Reopen → To Do", ToStatusID: "2", ToStatus: "To Do"},
		},
	}
}

func TestFiltered_NoQuery(t *testing.T) {
	m := testModal()
	items := m.filtered()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestFiltered_MatchesSubstring(t *testing.T) {
	m := testModal()
	m.query = "done"
	items := m.filtered()
	if len(items) != 1 {
		t.Fatalf("expected 1 match for 'done', got %d", len(items))
	}
	if items[0].ID != "2" {
		t.Fatalf("expected Done transition, got %q", items[0].Name)
	}
}

func TestFiltered_CaseInsensitive(t *testing.T) {
	m := testModal()
	m.query = "PROGRESS"
	items := m.filtered()
	if len(items) != 1 {
		t.Fatalf("expected 1 match for 'PROGRESS', got %d", len(items))
	}
}

func TestFiltered_NoMatch(t *testing.T) {
	m := testModal()
	m.query = "zzzzz"
	items := m.filtered()
	if len(items) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(items))
	}
}

func TestFiltered_PartialMatch(t *testing.T) {
	m := testModal()
	m.query = "to"
	items := m.filtered()
	// Should match "Reopen → To Do"
	if len(items) != 1 {
		t.Fatalf("expected 1 match for 'to', got %d", len(items))
	}
}

func TestTypeRune(t *testing.T) {
	m := testModal()
	m.typeRune('d')
	if m.query != "d" {
		t.Fatalf("expected 'd', got %q", m.query)
	}
	m.typeRune('o')
	if m.query != "do" {
		t.Fatalf("expected 'do', got %q", m.query)
	}
	if m.selected != 0 {
		t.Fatal("typeRune should reset selection to 0")
	}
}

func TestBackspace(t *testing.T) {
	m := testModal()
	m.query = "don"
	m.backspace()
	if m.query != "do" {
		t.Fatalf("expected 'do', got %q", m.query)
	}
	m.backspace()
	m.backspace()
	if m.query != "" {
		t.Fatalf("expected empty, got %q", m.query)
	}
	m.backspace() // should be a no-op
	if m.query != "" {
		t.Fatal("backspace on empty should be no-op")
	}
}

func TestBackspace_AdjustsSelection(t *testing.T) {
	m := testModal()
	m.query = "zzz"
	m.selected = 5 // out of bounds for filtered (0 items)
	m.backspace()   // "zz" → still 0 matches
	// selected should remain unchanged since no items
	m.query = ""
	m.selected = 5
	m.backspace() // no-op, already empty
}

func TestMoveSelection(t *testing.T) {
	m := testModal()
	m.moveSelection(1)
	if m.selected != 1 {
		t.Fatal("should move to 1")
	}
	m.moveSelection(1)
	if m.selected != 2 {
		t.Fatal("should move to 2")
	}
	m.moveSelection(1) // past end
	if m.selected != 2 {
		t.Fatal("should not go past end")
	}
	m.moveSelection(-1)
	if m.selected != 1 {
		t.Fatal("should go back to 1")
	}
	m.moveSelection(-5) // past start
	if m.selected != 0 {
		t.Fatal("should clamp to 0")
	}
}

func TestMoveSelection_EmptyFiltered(t *testing.T) {
	m := testModal()
	m.query = "zzz"
	m.moveSelection(1) // no items — should be no-op
}

func TestSelectedTransition(t *testing.T) {
	m := testModal()
	tr := m.selectedTransition()
	if tr == nil || tr.ID != "1" {
		t.Fatal("should return first transition")
	}
	m.selected = 2
	tr = m.selectedTransition()
	if tr == nil || tr.ID != "3" {
		t.Fatal("should return third transition")
	}
}

func TestSelectedTransition_OutOfBounds(t *testing.T) {
	m := testModal()
	m.selected = 99
	if m.selectedTransition() != nil {
		t.Fatal("out of bounds should return nil")
	}
	m.selected = -1
	if m.selectedTransition() != nil {
		t.Fatal("negative should return nil")
	}
}

func TestSelectedTransition_WithFilter(t *testing.T) {
	m := testModal()
	m.query = "done"
	m.selected = 0
	tr := m.selectedTransition()
	if tr == nil || tr.ID != "2" {
		t.Fatal("filtered selection should return the matching transition")
	}
}
