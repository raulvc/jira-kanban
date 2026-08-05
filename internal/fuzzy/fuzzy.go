package fuzzy

import (
	"strings"
	"unicode/utf8"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

// Match holds the result of a fuzzy match.
type Match struct {
	Score     int
	Positions []int // byte indices of matched chars in the searched string
}

// FieldMatch holds match info for a single field.
type FieldMatch struct {
	Positions []int // matched char indices in the field text
}

// Result holds a matched card and its match info.
type Result struct {
	Card        string // "KEY Summary" display string
	Key         string
	Summary     string
	Status      string
	Assignee    string
	Description string
	Labels      []string
	Epic        string
	Match       Match
	// Per-field match positions for highlighting in the preview.
	KeyMatch         []int
	SummaryMatch     []int
	StatusMatch      []int
	AssigneeMatch    []int
	DescriptionMatch []int
	LabelMatches     [][]int // parallel to Labels
}

var slab = util.MakeSlab(100*1024, 2048)

func init() {
	algo.Init("default")
}

// MatchCard performs a fuzzy match against all text fields of a card.
// It returns the best score across all fields, positions in the display
// string for list highlighting, and per-field positions for preview highlighting.
func MatchCard(query, key, summary, status, assignee, description string, labels []string) (Match, bool) {
	if query == "" {
		return Match{}, true
	}
	display := key + " " + summary
	var best Match
	matched := false

	check := func(field string) {
		if field == "" {
			return
		}
		if m := matchString(query, field); m.Score > 0 {
			matched = true
			if m.Score > best.Score {
				best = m
			}
		}
	}

	check(display)
	check(summary)
	check(key)
	check(status)
	check(assignee)
	check(description)
	for _, label := range labels {
		check(label)
	}
	if !matched {
		return Match{}, false
	}
	best.Positions = exactOrFuzzyPositions(query, display)
	return best, true
}

// MatchFields returns per-field match positions for highlighting in the preview.
// Each returned slice contains the char indices within that field that match the query.
func MatchFields(query, key, summary, status, assignee, description string, labels []string) (
	keyMatch, summaryMatch, statusMatch, assigneeMatch, descMatch []int, labelMatches [][]int,
) {
	keyMatch = exactOrFuzzyPositions(query, key)
	summaryMatch = exactOrFuzzyPositions(query, summary)
	statusMatch = exactOrFuzzyPositions(query, status)
	assigneeMatch = exactOrFuzzyPositions(query, assignee)
	descMatch = exactOrFuzzyPositions(query, description)
	if len(labels) > 0 {
		labelMatches = make([][]int, len(labels))
		for i, l := range labels {
			labelMatches[i] = exactOrFuzzyPositions(query, l)
		}
	}
	return
}

// exactOrFuzzyPositions returns positions for the exact case-insensitive
// substring match if found, otherwise falls back to fuzzy match positions.
// Positions are rune indices (not byte offsets) for safe use with multi-byte text.
func exactOrFuzzyPositions(query, target string) []int {
	if query == "" || target == "" {
		return nil
	}
	lowerTarget := strings.ToLower(target)
	lowerQuery := strings.ToLower(query)
	if idx := strings.Index(lowerTarget, lowerQuery); idx >= 0 {
		// Convert byte offset to rune offset
		runeOffset := utf8.RuneCountInString(target[:idx])
		queryRunes := utf8.RuneCountInString(query)
		positions := make([]int, queryRunes)
		for i := range queryRunes {
			positions[i] = runeOffset + i
		}
		return positions
	}
	return matchPositions(query, target)
}

// matchString runs fzf's FuzzyMatchV2 against a single string.
func matchString(query, target string) Match {
	if query == "" || target == "" {
		return Match{}
	}
	input := util.ToChars([]byte(target))
	runes := []rune(strings.ToLower(query))
	result, posPtr := algo.FuzzyMatchV2(false, false, true, &input, runes, true, slab)
	if result.Start < 0 {
		return Match{}
	}
	var positions []int
	if posPtr != nil {
		positions = *posPtr
	}
	return Match{Score: result.Score, Positions: positions}
}

// matchPositions returns just the matched positions for highlighting.
func matchPositions(query, target string) []int {
	if query == "" || target == "" {
		return nil
	}
	input := util.ToChars([]byte(target))
	runes := []rune(strings.ToLower(query))
	result, posPtr := algo.FuzzyMatchV2(false, false, true, &input, runes, true, slab)
	if result.Start < 0 || posPtr == nil {
		return nil
	}
	return *posPtr
}
