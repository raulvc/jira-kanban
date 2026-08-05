package fuzzy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchCard_EmptyQuery(t *testing.T) {
	is := assert.New(t)
	m, matched := MatchCard("", "P-1", "Fix login", "To Do", "John", "description", []string{"bug"})
	is.True(matched, "empty query should match")
	is.Equal(0, m.Score)
}

func TestMatchCard_ExactKey(t *testing.T) {
	is := assert.New(t)
	m, matched := MatchCard("P-1", "P-1", "Fix login", "To Do", "John", "desc", nil)
	is.True(matched)
	is.True(m.Score > 0, "should have positive score for exact key match")
	is.NotEmpty(m.Positions, "should have match positions for highlighting")
}

func TestMatchCard_FuzzySummary(t *testing.T) {
	is := assert.New(t)
	m, matched := MatchCard("fixlog", "P-1", "Fix login bug", "To Do", "John", "desc", nil)
	is.True(matched)
	is.NotEmpty(m.Positions, "should have positions for fuzzy match")
}

func TestMatchCard_MatchesAssignee(t *testing.T) {
	is := assert.New(t)
	_, matched := MatchCard("john", "P-1", "Fix login", "To Do", "John Doe", "desc", nil)
	is.True(matched, "should match assignee field")
}

func TestMatchCard_MatchesStatus(t *testing.T) {
	is := assert.New(t)
	_, matched := MatchCard("todo", "P-1", "Fix login", "To Do", "John", "desc", nil)
	is.True(matched, "should match status field")
}

func TestMatchCard_MatchesDescription(t *testing.T) {
	is := assert.New(t)
	_, matched := MatchCard("oauth", "P-1", "Fix login", "To Do", "John", "OAuth token expired", nil)
	is.True(matched, "should match description field")
}

func TestMatchCard_MatchesLabel(t *testing.T) {
	is := assert.New(t)
	_, matched := MatchCard("urgent", "P-1", "Fix login", "To Do", "John", "desc", []string{"urgent", "backend"})
	is.True(matched, "should match label field")
}

func TestMatchCard_NoMatch(t *testing.T) {
	is := assert.New(t)
	_, matched := MatchCard("xyzqwerty", "P-1", "Fix login", "To Do", "John", "desc", []string{"bug"})
	is.False(matched, "should not match when no characters are found")
}

func TestMatchCard_CaseInsensitive(t *testing.T) {
	is := assert.New(t)
	_, matched := MatchCard("FIX", "P-1", "fix login bug", "To Do", "John", "desc", nil)
	is.True(matched, "should match case-insensitively")
}

func TestMatchCard_PositionsInDisplayString(t *testing.T) {
	is := assert.New(t)
	m, matched := MatchCard("p1", "P-1", "Fix login", "To Do", "John", "desc", nil)
	is.True(matched)
	// Display string is "P-1 Fix login" — positions should point into it
	is.NotEmpty(m.Positions)
	// Verify positions are valid indices
	display := "P-1 Fix login"
	for _, p := range m.Positions {
		is.True(p >= 0 && p < len(display), "position %d should be within display string", p)
	}
}

func TestMatchCard_ExactSubstringHighlight(t *testing.T) {
	is := assert.New(t)
	// "location" appears as exact substring in "Integração location profile"
	m, matched := MatchCard("location", "PROJ-1", "Integração location profile", "To Do", "John", "desc", nil)
	is.True(matched)
	is.NotEmpty(m.Positions)
	// Positions should be contiguous (exact match), not scattered
	for i := 1; i < len(m.Positions); i++ {
		is.Equal(m.Positions[i], m.Positions[i-1]+1,
			"positions should be contiguous for exact substring match, got %d after %d",
			m.Positions[i], m.Positions[i-1])
	}
	// Verify the highlighted text spells "location"
	display := "PROJ-1 Integração location profile"
	var highlighted strings.Builder
	posSet := make(map[int]bool)
	for _, p := range m.Positions {
		posSet[p] = true
	}
	col := 0
	for _, r := range display {
		if posSet[col] {
			highlighted.WriteRune(r)
		}
		col++
	}
	is.Equal("location", highlighted.String(), "highlighted characters should spell the query")
}

func TestMatchCard_FuzzyPositionsWhenNoExactMatch(t *testing.T) {
	is := assert.New(t)
	// "fxlgn" is a fuzzy match, not an exact substring
	m, matched := MatchCard("fxlgn", "P-1", "fix login", "To Do", "John", "desc", nil)
	is.True(matched)
	is.NotEmpty(m.Positions)
	// Positions need not be contiguous for fuzzy-only matches
}

func TestExactOrFuzzyPositions_ExactMatch(t *testing.T) {
	positions := exactOrFuzzyPositions("cat", "the cat sat")
	is := assert.New(t)
	is.Len(positions, 3)
	is.Equal(4, positions[0], "should start at index of 'cat' in 'the cat sat'")
	is.Equal(5, positions[1])
	is.Equal(6, positions[2])
}

func TestExactOrFuzzyPositions_NoExactMatch(t *testing.T) {
	// "xyz" doesn't appear in "hello world" — should return nil or fuzzy positions
	positions := exactOrFuzzyPositions("xyz", "hello world")
	is := assert.New(t)
	is.Nil(positions, "should return nil when no exact or fuzzy match")
}

func TestExactOrFuzzyPositions_MultiByte(t *testing.T) {
	// "ção" appears after "Integra" (7 ASCII chars) — byte offset would be 7,
	// but "ção" starts at rune index 7 too (all ASCII before it).
	// Let's test with a case where byte and rune offsets differ:
	// "é location" — "é" is 2 bytes, so "location" starts at byte 2 but rune 1.
	positions := exactOrFuzzyPositions("location", "é location")
	is := assert.New(t)
	is.Len(positions, 8)
	// Rune index of "location" in "é location" is 2 (é=1, space=1)
	is.Equal(2, positions[0], "should use rune index, not byte offset")
}

func TestExactOrFuzzyPositions_MultiByteQuery(t *testing.T) {
	// Query with multi-byte chars
	positions := exactOrFuzzyPositions("ção", "Integração")
	is := assert.New(t)
	is.Len(positions, 3)
	// "Integra" is 7 runes, so "ção" starts at rune index 7
	is.Equal(7, positions[0])
}

func TestMatchFields_Description(t *testing.T) {
	is := assert.New(t)
	_, _, _, _, descMatch, _ := MatchFields("oauth", "P-1", "Fix login", "To Do", "John", "OAuth token expired", nil)
	is.NotEmpty(descMatch, "should find match in description")
	// Verify positions point to "OAuth" in the description
	desc := "OAuth token expired"
	for _, p := range descMatch {
		is.True(p >= 0 && p < len(desc), "position should be within description")
	}
}

func TestMatchFields_Assignee(t *testing.T) {
	is := assert.New(t)
	_, _, _, assigneeMatch, _, _ := MatchFields("john", "P-1", "Fix login", "To Do", "John Doe", "desc", nil)
	is.NotEmpty(assigneeMatch, "should find match in assignee")
}

func TestMatchFields_Status(t *testing.T) {
	is := assert.New(t)
	_, _, statusMatch, _, _, _ := MatchFields("progress", "P-1", "Fix login", "In Progress", "John", "desc", nil)
	is.NotEmpty(statusMatch, "should find match in status")
}

func TestMatchFields_NoMatchInField(t *testing.T) {
	is := assert.New(t)
	_, _, _, _, descMatch, _ := MatchFields("zzz", "P-1", "Fix login", "To Do", "John", "desc", nil)
	is.Nil(descMatch, "should return nil when no match in description")
}

func TestMatchFields_Labels(t *testing.T) {
	is := assert.New(t)
	_, _, _, _, _, labelMatches := MatchFields("urgent", "P-1", "Fix login", "To Do", "John", "desc", []string{"urgent", "backend"})
	is.Len(labelMatches, 2)
	is.NotEmpty(labelMatches[0], "should match first label")
	is.Nil(labelMatches[1], "should not match second label")
}
