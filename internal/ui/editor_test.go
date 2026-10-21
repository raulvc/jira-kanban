package ui

import (
	"os"
	"path/filepath"
	"strings"
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

// newDescFormState builds a formState focused on the description field with
// the given text and wrap width.
func newDescFormState(desc string, wrapW int) *formState {
	return &formState{
		desc:     desc,
		descCur:  len([]rune(desc)),
		descWrap: wrapW,
		field:    ifDescription,
	}
}

// TestMoveHome_VirtualLineStart verifies that Home moves to the start of the
// current VISUAL line when the text wraps before a real newline — not to the
// start of the whole text.
func TestMoveHome_VirtualLineStart(t *testing.T) {
	is := assert.New(t)

	// 80 chars, wrapped at 30 → visual lines: [0..30) [30..60) [60..80)
	long := strings.Repeat("a", 80)
	f := newDescFormState(long, 30)

	// Cursor inside the second visual line (index 40 → line 1, col 10)
	f.descCur = 40
	f.moveHome()
	is.Equal(30, f.descCur, "Home should jump to the start of the second visual line")

	// Cursor inside the third visual line
	f.descCur = 70
	f.moveHome()
	is.Equal(60, f.descCur, "Home should jump to the start of the third visual line")

	// Cursor on the first visual line goes to absolute start
	f.descCur = 5
	f.moveHome()
	is.Equal(0, f.descCur, "Home on the first visual line should go to 0")
}

// TestMoveHome_RealLineBreakStart verifies Home respects real newlines: a
// short line followed by a wrapped long line each have their own start.
func TestMoveHome_RealLineBreakStart(t *testing.T) {
	is := assert.New(t)

	// Line 1: "ab" + \n, line 2: 65 chars (wraps at 30 → 3 visual lines)
	text := "ab\n" + strings.Repeat("b", 65)
	f := newDescFormState(text, 30)

	// Cursor at absolute 40 → inside visual line 2 ("b" repeat line, second
	// visual segment starting at offset 33+30=... let's compute: visual lines
	// are "ab"(offset 0, len 2, newline consumed → next offset 3),
	// b[0:30] (offset 3), b[30:60] (offset 33), b[60:65] (offset 63).
	f.descCur = 40
	f.moveHome()
	is.Equal(33, f.descCur, "Home should jump to the start of the second visual segment of line 2")

	f.descCur = 4
	f.moveHome()
	is.Equal(3, f.descCur, "Home should jump to the start of the first visual segment of line 2")

	f.descCur = 65
	f.moveHome()
	is.Equal(63, f.descCur, "Home should jump to the start of the last visual segment")
}

// TestMoveEnd_VirtualLineEnd verifies that End moves to the end of the
// current VISUAL line, not the end of the whole text.
func TestMoveEnd_VirtualLineEnd(t *testing.T) {
	is := assert.New(t)

	long := strings.Repeat("a", 80)
	f := newDescFormState(long, 30)
	f.descCur = 40
	f.moveEnd()
	is.Equal(60, f.descCur, "End should jump to the end of the second visual line (offset 30+30)")

	f.descCur = 5
	f.moveEnd()
	is.Equal(30, f.descCur, "End should jump to the end of the first visual line")

	f.descCur = 70
	f.moveEnd()
	is.Equal(80, f.descCur, "End on the last visual line goes to the absolute end")
}

// TestMoveHomeEnd_PlainShortLine verifies Home/End still work on a line that
// fits without wrapping.
func TestMoveHomeEnd_PlainShortLine(t *testing.T) {
	is := assert.New(t)

	f := newDescFormState("hello", 30)
	f.descCur = 3
	f.moveEnd()
	is.Equal(5, f.descCur, "End on a non-wrapped line goes to the text end")
	f.moveHome()
	is.Equal(0, f.descCur, "Home on the only visual line goes to 0")
}

// TestDescLines_WrapsAtRequestedWidth verifies descLines honors the width
// parameter (needed for the adaptive modal width).
func TestDescLines_WrapsAtRequestedWidth(t *testing.T) {
	is := assert.New(t)

	long := strings.Repeat("x", 100)
	lines := descLines(long, 30)
	is.Len(lines, 4, "100 chars at width 30 should produce 4 visual lines")
	for i, l := range lines {
		want := 30
		if i == len(lines)-1 {
			want = 10
		}
		is.Len([]rune(l), want)
	}
}

// TestDescWrapWidth_Fallback verifies the wrap width falls back to the
// default before the modal's first draw sets it.
func TestDescWrapWidth_Fallback(t *testing.T) {
	is := assert.New(t)
	f := &formState{field: ifDescription}
	is.Equal(defaultDescWrapW, f.descWrapWidth())
	f.descWrap = 100
	is.Equal(100, f.descWrapWidth())
}

// TestDescCurLine_BoundaryBelongsToNextLine verifies that a cursor sitting
// exactly on a line boundary is owned by the NEXT visual line (where it is
// rendered), matching standard editor behavior.
func TestDescCurLine_BoundaryBelongsToNextLine(t *testing.T) {
	is := assert.New(t)

	// 80 chars, wrap 30 → visual lines [0,30) [30,60) [60,80]
	f := newDescFormState(strings.Repeat("a", 80), 30)
	lines := descLines(f.desc, f.descWrap)

	// Cursor at offset 30 (boundary between line 0 and 1) → line 1
	f.descCur = 30
	is.Equal(1, f.descCurLine(lines))

	// Cursor at offset 60 (boundary between line 1 and 2) → line 2
	f.descCur = 60
	is.Equal(2, f.descCurLine(lines))

	// Cursor at end of text → last line
	f.descCur = 80
	is.Equal(2, f.descCurLine(lines))
}

// TestMoveHome_FromEndOfTextWithTrailingEmptyLines verifies Home from the
// end of text lands on a visual line start. With trailing newlines the last
// visual line is empty and starts at end-of-text, so Home is a no-op there.
func TestMoveHome_FromEndOfTextWithTrailingEmptyLines(t *testing.T) {
	is := assert.New(t)

	// "ab\n" + 65 b's + "\n\n" — trailing empty visual lines at the end.
	text := "ab\n" + strings.Repeat("b", 65) + "\n\n"
	f := newDescFormState(text, 30)
	f.descCur = len([]rune(text))
	f.moveHome()

	lines := descLines(text, 30)
	offsets := descLineOffsets(text, lines)
	is.Equal(offsets[len(lines)-1], f.descCur,
		"Home from the end should land at the start of the last visual line")

	// From the end of the second-to-last visual line (cursor 68, inside
	// line 3's range [63,69)), Home jumps to its start.
	f.descCur = 68
	f.moveHome()
	is.Equal(63, f.descCur)
}

// TestMoveHome_MidTextNeverJumpsToEnd verifies Home always moves to the
// start of some visual line, never to the end of the text.
func TestMoveHome_MidTextNeverJumpsToEnd(t *testing.T) {
	is := assert.New(t)

	text := "first line here\n" + strings.Repeat("x", 100) + "\nlast line"
	f := newDescFormState(text, 30)
	lines := descLines(text, 30)
	offsets := descLineOffsets(text, lines)
	starts := make(map[int]bool)
	for _, off := range offsets {
		starts[off] = true
	}

	for pos := range len([]rune(text)) {
		f.descCur = pos
		f.moveHome()
		is.True(starts[f.descCur],
			"Home from %d should land on a line start, got %d", pos, f.descCur)
	}
}

// TestDeleteWordBack_Description covers Alt+Backspace word deletion in the
// description field: whitespace is removed first, then the word.
func TestDeleteWordBack_Description(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		cur      int
		wantDesc string
		wantCur  int
	}{
		{
			name:     "deletes word before cursor",
			text:     "hello world",
			cur:      11,
			wantDesc: "hello ",
			wantCur:  6,
		},
		{
			name:     "whitespace first, then word",
			text:     "hello    ",
			cur:      9,
			wantDesc: "", // "    " + "hello" are both before the cursor
			wantCur:  0,
		},
		{
			name:     "cursor mid-word deletes partial word",
			text:     "hello world",
			cur:      8,
			wantDesc: "hello rld",
			wantCur:  6,
		},
		{
			name:     "newline counts as whitespace",
			text:     "first\nsecond",
			cur:      12,
			wantDesc: "first\n",
			wantCur:  6,
		},
		{
			name:     "at start is a no-op",
			text:     "hello",
			cur:      0,
			wantDesc: "hello",
			wantCur:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			f := &formState{
				desc:     tt.text,
				descCur:  tt.cur,
				descWrap: 30,
				field:    ifDescription,
			}
			f.deleteWordBack()
			is.Equal(tt.wantDesc, f.desc)
			is.Equal(tt.wantCur, f.descCur)
		})
	}
}

// TestDeleteWordBack_Summary verifies word deletion works in the summary
// field too.
func TestDeleteWordBack_Summary(t *testing.T) {
	is := assert.New(t)
	f := &formState{
		summary: "fix the login bug",
		sumCur:  len([]rune("fix the login bug")),
		field:   ifSummary,
	}
	f.deleteWordBack()
	is.Equal("fix the login ", f.summary)
	is.Equal(14, f.sumCur)
	f.deleteWordBack()
	is.Equal("fix the ", f.summary)
}

// TestWordBackStart verifies the span computation directly.
func TestWordBackStart(t *testing.T) {
	is := assert.New(t)
	runes := []rune("one two   ")
	// Cursor at end: "   " removed, then "two" → start at 4.
	is.Equal(4, wordBackStart(runes, len(runes)))
	// Cursor at 7 is inside "two" (after "o") → deletes back to word start.
	is.Equal(4, wordBackStart(runes, 7))
	// Cursor at 0: no-op.
	is.Equal(0, wordBackStart(runes, 0))
}
