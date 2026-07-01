package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/raulvc/jira-kanban/internal/jira"
)

func testBoard() jira.Board {
	return jira.Board{
		Name: "Test",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "first", Status: "To Do"},
				{Key: "P-2", Summary: "second", Status: "To Do"},
			}},
			{Name: "In Progress", Issues: []jira.Card{
				{Key: "P-3", Summary: "third", Status: "In Progress"},
			}},
			{Name: "Done", Issues: []jira.Card{
				{Key: "P-4", Summary: "fourth", Status: "Done"},
			}},
		},
	}
}

func TestNewBoardState(t *testing.T) {
	s := newBoardState(testBoard())
	if s.colIdx != 0 {
		t.Fatal("initial column should be 0")
	}
	if len(s.cardIdx) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(s.cardIdx))
	}
}

func TestMoveColumn(t *testing.T) {
	s := newBoardState(testBoard())
	s.moveColumn(1)
	if s.colIdx != 1 {
		t.Fatalf("expected column 1, got %d", s.colIdx)
	}
	s.moveColumn(1)
	if s.colIdx != 2 {
		t.Fatalf("expected column 2, got %d", s.colIdx)
	}
	s.moveColumn(1) // should not go past last
	if s.colIdx != 2 {
		t.Fatal("should not go past last column")
	}
	s.moveColumn(-1)
	if s.colIdx != 1 {
		t.Fatal("should go back")
	}
}

func TestMoveCard(t *testing.T) {
	s := newBoardState(testBoard())
	s.moveCard(1)
	if s.cardIdx[0] != 1 {
		t.Fatal("should move to card 1")
	}
	s.moveCard(1) // should not go past last
	if s.cardIdx[0] != 1 {
		t.Fatal("should not go past last card")
	}
	s.moveCard(-1)
	if s.cardIdx[0] != 0 {
		t.Fatal("should go back to 0")
	}
	s.moveCard(-1) // should not go negative
	if s.cardIdx[0] != 0 {
		t.Fatal("should not go negative")
	}
}

func TestJumpCard(t *testing.T) {
	s := newBoardState(testBoard())
	s.jumpCard(true) // jump to end
	if s.cardIdx[0] != 1 {
		t.Fatalf("expected last card (1), got %d", s.cardIdx[0])
	}
	s.jumpCard(false) // jump to start
	if s.cardIdx[0] != 0 {
		t.Fatal("expected first card (0)")
	}
}

func TestSelectedCard(t *testing.T) {
	s := newBoardState(testBoard())
	card := s.selectedCard()
	if card == nil {
		t.Fatal("should return a card")
	}
	if card.Key != "P-1" {
		t.Fatalf("expected P-1, got %s", card.Key)
	}
	s.moveColumn(1)
	card = s.selectedCard()
	if card.Key != "P-3" {
		t.Fatalf("expected P-3, got %s", card.Key)
	}
}

func TestSelectedCard_EmptyColumn(t *testing.T) {
	board := jira.Board{
		Name:    "B",
		Columns: []jira.Column{{Name: "Empty", Issues: nil}},
	}
	s := newBoardState(board)
	if s.selectedCard() != nil {
		t.Fatal("empty column should return nil")
	}
}

func TestMoveIssueToStatus(t *testing.T) {
	s := newBoardState(testBoard())

	s.moveIssueToStatus("P-1", "In Progress")

	// P-1 should no longer be in To Do
	for _, c := range s.data.Columns[0].Issues {
		if c.Key == "P-1" {
			t.Fatal("P-1 should have been removed from To Do")
		}
	}
	// P-1 should be in In Progress
	found := false
	for _, c := range s.data.Columns[1].Issues {
		if c.Key == "P-1" {
			found = true
			if c.Status != "In Progress" {
				t.Fatalf("P-1 status should be In Progress, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("P-1 should be in In Progress column")
	}
	// To Do should have 1 card left
	if len(s.data.Columns[0].Issues) != 1 {
		t.Fatalf("To Do should have 1 card, got %d", len(s.data.Columns[0].Issues))
	}
}

func TestMoveIssueToStatus_NotFound(t *testing.T) {
	s := newBoardState(testBoard())
	s.moveIssueToStatus("P-99", "Done")
	// Should be a no-op
	if len(s.data.Columns[0].Issues) != 2 {
		t.Fatal("should be unchanged")
	}
}

func TestMoveIssueToStatus_SameColumn(t *testing.T) {
	s := newBoardState(testBoard())
	s.moveIssueToStatus("P-1", "To Do")
	// Same column — no-op
	if len(s.data.Columns[0].Issues) != 2 {
		t.Fatal("same column move should be a no-op")
	}
}

func TestMoveIssueToStatus_UnknownTarget(t *testing.T) {
	s := newBoardState(testBoard())
	s.moveIssueToStatus("P-1", "NonExistentStatus")
	// Unknown target — no-op
	if len(s.data.Columns[0].Issues) != 2 {
		t.Fatal("unknown target should be a no-op")
	}
}

func TestMoveIssueToStatus_AdjustsCardIdx(t *testing.T) {
	s := newBoardState(testBoard())
	// Select the last card in To Do (P-2, index 1)
	s.cardIdx[0] = 1
	s.moveIssueToStatus("P-2", "Done")
	// cardIdx should be adjusted since we removed the last card
	if s.cardIdx[0] != 0 {
		t.Fatalf("cardIdx should be adjusted to 0, got %d", s.cardIdx[0])
	}
}

func TestReload(t *testing.T) {
	s := newBoardState(testBoard())
	s.colIdx = 2
	s.statusMsg = "hello"
	s.syncing = true

	newBoard := jira.Board{
		Name:    "New",
		Columns: []jira.Column{{Name: "X", Issues: nil}},
	}
	s.reload(newBoard)

	if s.colIdx != 0 {
		t.Fatal("reload should reset colIdx if out of bounds")
	}
	if s.syncing {
		t.Fatal("reload should clear syncing")
	}
	if s.statusMsg != "" {
		t.Fatal("reload should clear statusMsg")
	}
	if len(s.cardIdx) != 1 {
		t.Fatal("cardIdx should match new column count")
	}
}

func TestFilteredData_HideEmpty(t *testing.T) {
	board := jira.Board{
		Name: "B",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{{Key: "P-1", Summary: "task", Status: "To Do"}}},
			{Name: "In Progress", Issues: []jira.Card{}},
			{Name: "Done", Issues: []jira.Card{{Key: "P-2", Summary: "done", Status: "Done"}}},
		},
	}

	s := newBoardState(board)
	is := assert.New(t)

	fd := s.filteredData()
	is.Len(fd.Columns, 3, "without hideEmpty, all columns present")

	s.hideEmpty = true
	fd = s.filteredData()
	is.Len(fd.Columns, 2, "empty column should be hidden")
	is.Equal("To Do", fd.Columns[0].Name)
	is.Equal("Done", fd.Columns[1].Name)
}

func TestFilteredData_HideEmptyWithMemberFilter(t *testing.T) {
	board := jira.Board{
		Name: "B",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "task", Status: "To Do", Assignee: "Alice"},
			}},
			{Name: "In Progress", Issues: []jira.Card{}},
			{Name: "Done", Issues: []jira.Card{
				{Key: "P-2", Summary: "wip", Status: "Done", Assignee: "Bob"},
			}},
		},
	}

	s := newBoardState(board)
	s.hideEmpty = true
	s.memberFilter = "Alice"

	is := assert.New(t)
	fd := s.filteredData()
	is.Len(fd.Columns, 2, "empty column hidden; In Progress is empty in original data")
	is.Equal("To Do", fd.Columns[0].Name)
	is.Equal("Done", fd.Columns[1].Name)
	is.Len(fd.Columns[0].Issues, 1)
	is.Empty(fd.Columns[1].Issues, "Done has no Alice cards after member filter")
}

func TestFilteredData_HideEmptyAllEmpty(t *testing.T) {
	board := jira.Board{
		Name: "B",
		Columns: []jira.Column{
			{Name: "To Do", Issues: nil},
			{Name: "Done", Issues: nil},
		},
	}

	s := newBoardState(board)
	s.hideEmpty = true

	is := assert.New(t)
	fd := s.filteredData()
	is.Empty(fd.Columns, "all columns empty should result in no columns")
}

func TestClampSelection_AfterHideEmptyToggle(t *testing.T) {
	board := jira.Board{
		Name: "B",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{{Key: "P-1", Summary: "a", Status: "To Do"}}},
			{Name: "In Progress", Issues: []jira.Card{}},
			{Name: "Done", Issues: []jira.Card{{Key: "P-2", Summary: "b", Status: "Done"}}},
		},
	}

	s := newBoardState(board)
	s.colIdx = 1 // pointing at empty "In Progress" column
	s.hideEmpty = true
	s.clampSelection()

	is := assert.New(t)
	// colIdx still points at index 1 in the original data — clampSelection
	// only adjusts cardIdx, not colIdx. But the filtered view removes
	// column at index 1, so cardIdx should be valid.
	is.Equal(0, s.cardIdx[1], "cardIdx for empty column should be 0")
}

func TestUpdateIssue_Summary(t *testing.T) {
	s := newBoardState(testBoard())
	is := require.New(t)

	newSum := "updated summary"
	s.updateIssue("P-1", &newSum, nil, nil, nil)

	card := s.data.Columns[0].Issues[0]
	is.Equal("updated summary", card.Summary)
}

func TestUpdateIssue_ClearsEpicAndParentWhenParentIsEpic(t *testing.T) {
	board := jira.Board{
		Name: "B",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "task", Status: "To Do", Epic: "My Epic", ParentKey: "EPIC-1", ParentSummary: "My Epic", ParentIsEpic: true},
			}},
		},
	}

	s := newBoardState(board)
	is := require.New(t)

	epic := ""
	s.updateIssue("P-1", nil, nil, nil, &epic)

	card := s.data.Columns[0].Issues[0]
	is.Equal("", card.Epic)
	is.Equal("", card.ParentKey)
	is.Equal("", card.ParentSummary)
	is.False(card.ParentIsEpic)
}

func TestUpdateIssue_ClearEpicDoesNotClearParentWhenNotEpic(t *testing.T) {
	board := jira.Board{
		Name: "B",
		Columns: []jira.Column{
			{Name: "To Do", Issues: []jira.Card{
				{Key: "P-1", Summary: "subtask", Status: "To Do", Epic: "", ParentKey: "PARENT-1", ParentSummary: "Parent Story", ParentIsEpic: false},
			}},
		},
	}

	s := newBoardState(board)
	is := require.New(t)

	epic := ""
	s.updateIssue("P-1", nil, nil, nil, &epic)

	card := s.data.Columns[0].Issues[0]
	is.Equal("", card.Epic)
	is.Equal("PARENT-1", card.ParentKey, "parent should not be cleared when ParentIsEpic is false")
	is.Equal("Parent Story", card.ParentSummary)
	is.False(card.ParentIsEpic)
}

func TestUpdateIssue_UpdatesDetailCard(t *testing.T) {
	s := newBoardState(testBoard())
	s.detail = &detailState{card: jira.Card{Key: "P-1", Summary: "original", Description: "old", Epic: "E1"}}
	is := require.New(t)

	newSum := "new summary"
	epic := "E2"
	s.updateIssue("P-1", &newSum, nil, nil, &epic)

	is.Equal("new summary", s.detail.card.Summary)
	is.Equal("E2", s.detail.card.Epic)
}

func TestUpdateIssue_NilFieldsNoOp(t *testing.T) {
	s := newBoardState(testBoard())
	is := require.New(t)

	orig := s.data.Columns[0].Issues[0]
	s.updateIssue("P-1", nil, nil, nil, nil)
	card := s.data.Columns[0].Issues[0]
	is.Equal(orig, card)
}
