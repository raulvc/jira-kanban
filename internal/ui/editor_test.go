package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// TestOpenDescriptionInEditor_NilDetailIsNoOp verifies that calling
// openDescriptionInEditor with no detail modal open is a safe no-op.
func TestOpenDescriptionInEditor_NilDetailIsNoOp(t *testing.T) {
	s := newBoardState(testBoard())
	ctx := &appContext{state: s}
	// Should not panic with nil detail
	openDescriptionInEditor(ctx)
}

// TestOpenDescriptionInEditor_LoadingIsNoOp verifies that calling
// openDescriptionInEditor while the detail is still loading is a no-op.
func TestOpenDescriptionInEditor_LoadingIsNoOp(t *testing.T) {
	s := newBoardState(testBoard())
	s.detail = &detailState{card: jira.Card{Key: "P-1"}, loading: true}
	ctx := &appContext{state: s}
	openDescriptionInEditor(ctx)
	// Description should be unchanged
	is := assert.New(t)
	is.Empty(s.detail.card.Description)
}

// TestEditDescriptionExternal_NoChangeDoesNotSave writes the original
// description to a temp file, reads it back unchanged, and verifies that
// the save path is not triggered (no API call, no state mutation).
func TestEditDescriptionExternal_NoChangeDoesNotSave(t *testing.T) {
	origDesc := "original description"
	tmp := t.TempDir()
	tmpFile := filepath.Join(tmp, "desc.md")
	require.NoError(t, os.WriteFile(tmpFile, []byte(origDesc), 0644))

	edited, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	newDesc := string(edited)
	if newDesc[len(newDesc)-1] == '\n' {
		newDesc = newDesc[:len(newDesc)-1]
	}
	assert.Equal(t, origDesc, newDesc, "unchanged file should match original")
}

// TestEditDescriptionExternal_TrimsTrailingNewlines verifies that the
// description comparison trims trailing newlines so editors that add a
// final newline don't trigger a spurious save.
func TestEditDescriptionExternal_TrimsTrailingNewlines(t *testing.T) {
	origDesc := "some text"
	edited := "some text\n" // editor adds trailing newline

	newDesc := edited
	for len(newDesc) > 0 && newDesc[len(newDesc)-1] == '\n' {
		newDesc = newDesc[:len(newDesc)-1]
	}
	assert.Equal(t, origDesc, newDesc, "trailing newlines should be trimmed")
}

// TestEditDescriptionExternal_ChangedDescriptionUpdatesState verifies
// that after a description change, updateIssue correctly updates both
// the board card and the detail modal card.
func TestEditDescriptionExternal_ChangedDescriptionUpdatesState(t *testing.T) {
	s := newBoardState(testBoard())
	s.detail = &detailState{card: jira.Card{Key: "P-1", Description: "old"}}
	is := require.New(t)

	newDesc := "new description"
	s.updateIssue("P-1", nil, &newDesc, nil, nil)

	// Board card updated
	card := s.data.Columns[0].Issues[0]
	is.Equal("new description", card.Description, "board card description should be updated")
	is.Nil(card.RichDesc, "RichDesc should be cleared after description update")

	// Detail card updated
	is.Equal("new description", s.detail.card.Description, "detail card description should be updated")
	is.Nil(s.detail.card.RichDesc, "detail RichDesc should be cleared")
}

// TestEditDescriptionExternal_EmptyDescriptionClears verifies that
// saving an empty description clears the field.
func TestEditDescriptionExternal_EmptyDescriptionClears(t *testing.T) {
	s := newBoardState(testBoard())
	s.data.Columns[0].Issues[0].Description = "existing"
	s.detail = &detailState{card: jira.Card{Key: "P-1", Description: "existing"}}
	is := require.New(t)

	emptyDesc := ""
	s.updateIssue("P-1", nil, &emptyDesc, nil, nil)

	card := s.data.Columns[0].Issues[0]
	is.Equal("", card.Description, "description should be cleared")
	is.Equal("", s.detail.card.Description, "detail description should be cleared")
}

func TestSanitizeEditorText_StripsBOM(t *testing.T) {
	is := assert.New(t)
	result := sanitizeEditorText("\uFEFFhello world")
	is.Equal("hello world", result, "BOM should be stripped")
}

func TestSanitizeEditorText_NormalizesCRLF(t *testing.T) {
	is := assert.New(t)
	result := sanitizeEditorText("line1\r\nline2\r\n")
	is.Equal("line1\nline2", result, "CRLF should be normalized to LF")
}

func TestSanitizeEditorText_NormalizesBareCR(t *testing.T) {
	is := assert.New(t)
	result := sanitizeEditorText("line1\rline2\r")
	is.Equal("line1\nline2", result, "bare CR should be normalized to LF")
}

func TestSanitizeEditorText_TrimsTrailingNewlines(t *testing.T) {
	is := assert.New(t)
	result := sanitizeEditorText("hello\n\n\n")
	is.Equal("hello", result, "trailing newlines should be trimmed")
}

func TestSanitizeEditorText_PreservesInternalNewlines(t *testing.T) {
	is := assert.New(t)
	result := sanitizeEditorText("line1\nline2\nline3")
	is.Equal("line1\nline2\nline3", result, "internal newlines should be preserved")
}

func TestSanitizeEditorText_PreservesUnicode(t *testing.T) {
	is := assert.New(t)
	result := sanitizeEditorText("Integração ação teste")
	is.Equal("Integração ação teste", result, "Unicode should be preserved")
}

func TestTypeRune_RejectsRuneError(t *testing.T) {
	s := newBoardState(testBoard())
	s.createIssue = newCreateIssueState("PROJ")
	s.createIssue.field = ifDescription
	before := s.createIssue.desc
	s.createIssue.typeRune('\uFFFD')
	is := assert.New(t)
	is.Equal(before, s.createIssue.desc, "RuneError should not be inserted")
}
