package ui

import (
	"testing"

	"github.com/raulvc/jira-kanban/internal/jira"
)

func TestTruncStr(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "h"},
		{"hello", 0, ""},
		{"", 5, ""},
		{"ab", 2, "ab"},
		{"abc", 2, "a…"},
	}
	for _, tt := range tests {
		got := truncStr(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncStr(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestWrappedLineCount(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  int
	}{
		{"", 10, 1},
		{"hello", 10, 1},
		{"hello", 5, 1},
		{"hello", 3, 2},
		{"hello", 1, 5},
		{"hello", 0, 5},
		{"ab", 2, 1},
		{"abc", 2, 2},
		{"abcdef", 3, 2},
		{"abcdefg", 3, 3},
	}
	for _, tt := range tests {
		got := wrappedLineCount(tt.text, tt.width)
		if got != tt.want {
			t.Errorf("wrappedLineCount(%q, %d) = %d, want %d", tt.text, tt.width, got, tt.want)
		}
	}
}

func TestVisibleCols(t *testing.T) {
	tests := []struct {
		width, totalCols, want int
	}{
		{0, 5, 1},
		{100, 0, 0},
		{100, 5, 3},
		{28, 5, 1},
		{56, 5, 2},
		{200, 3, 3},
	}
	for _, tt := range tests {
		got := visibleCols(tt.width, tt.totalCols)
		if got != tt.want {
			t.Errorf("visibleCols(%d, %d) = %d, want %d", tt.width, tt.totalCols, got, tt.want)
		}
	}
}

func TestViewportStart(t *testing.T) {
	tests := []struct {
		width, totalCols, activeCol, want int
	}{
		{100, 5, 0, 0},
		{100, 5, 2, 0},
		{100, 5, 3, 1},
		{100, 5, 4, 2},
		{28, 5, 4, 4},
		{200, 3, 0, 0},
		{200, 3, 2, 0},
	}
	for _, tt := range tests {
		got := viewportStart(tt.width, tt.totalCols, tt.activeCol)
		if got != tt.want {
			t.Errorf("viewportStart(%d, %d, %d) = %d, want %d",
				tt.width, tt.totalCols, tt.activeCol, got, tt.want)
		}
	}
}

func TestCardHeight(t *testing.T) {
	card := jira.Card{Summary: "hello"}
	h := cardHeight(card, 30)
	if h < 3 {
		t.Errorf("cardHeight simple card = %d, want >= 3", h)
	}
	cardWithLabels := jira.Card{Summary: "hi", Labels: []string{"bug"}}
	h2 := cardHeight(cardWithLabels, 30)
	if h2 <= h {
		t.Errorf("card with labels (%d) should be taller than without (%d)", h2, h)
	}
	longCard := jira.Card{Summary: "a very long summary that should wrap across multiple lines"}
	h3 := cardHeight(longCard, 10)
	if h3 <= h {
		t.Errorf("long summary card (%d) should be taller than short (%d)", h3, h)
	}
}

func TestEnsureVisible(t *testing.T) {
	cards := []jira.Card{
		{Summary: "a"},
		{Summary: "b"},
		{Summary: "c"},
	}
	colW := 30
	viewH := 5

	s := ensureVisible(cards, 0, 0, colW, viewH)
	if s != 0 {
		t.Fatalf("first card should not scroll, got %d", s)
	}

	s = ensureVisible(cards, 2, 0, colW, 100)
	if s != 0 {
		t.Fatalf("large viewport should not need scroll, got %d", s)
	}

	s = ensureVisible(cards, 0, 10, colW, viewH)
	if s != 0 {
		t.Fatalf("should scroll up to 0, got %d", s)
	}
}

func TestEnsureVisible_ScrollDown(t *testing.T) {
	cards := make([]jira.Card, 0, 20)
	for range 20 {
		cards = append(cards, jira.Card{Summary: "card"})
	}
	colW := 30
	viewH := 6

	s := ensureVisible(cards, 15, 0, colW, viewH)
	if s <= 0 {
		t.Fatalf("card 15 should cause scroll down, got %d", s)
	}
}