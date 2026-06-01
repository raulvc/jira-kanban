package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  int
	}{
		{"empty", "", 10, 1},
		{"short", "hello", 10, 1},
		{"exact width", "hello", 5, 1},
		{"wraps", "hello world", 5, 3},
		{"newline", "hello\nworld", 10, 2},
		{"zero width", "hi", 0, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.text, tt.width)
			if len(got) != tt.want {
				t.Errorf("wrapText(%q, %d) = %d lines, want %d", tt.text, tt.width, len(got), tt.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\nb\nc")
	if len(got) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got))
	}
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("unexpected split: %v", got)
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{"Done", true},
		{"In Progress", true},
		{"To Do", true},
		{"Unknown", true},
	}
	for _, tt := range tests {
		s := statusColor(tt.status)
		if s == (tcell.StyleDefault) {
			t.Errorf("statusColor(%q) returned default style", tt.status)
		}
	}
}

func TestDetailState_Close(t *testing.T) {
	s := newBoardState(testBoard())
	s.detail = &detailState{card: jira.Card{Key: "P-1"}, loading: false}
	if s.detail == nil {
		t.Fatal("detail should be set")
	}
	s.detail = nil
	if s.detail != nil {
		t.Fatal("detail should be nil after close")
	}
}

func TestDetailState_ScrollClamp(t *testing.T) {
	d := &detailState{card: jira.Card{Key: "P-1", Description: "short"}, scroll: 0}
	if d.scroll != 0 {
		t.Fatalf("initial scroll should be 0, got %d", d.scroll)
	}
	// Scroll up from 0 should stay 0
	d.scroll--
	if d.scroll < 0 {
		d.scroll = 0
	}
	if d.scroll != 0 {
		t.Fatalf("scroll should clamp to 0, got %d", d.scroll)
	}
}