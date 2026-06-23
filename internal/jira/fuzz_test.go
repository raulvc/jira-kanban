package jira

import (
	"encoding/json"
	"testing"
)

func FuzzParseDescription(f *testing.F) {
	seeds := []string{
		`null`,
		`"plain text"`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"code"}]}]}`,
		``,
		`{}`,
		`invalid`,
		`{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}]}]}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		raw := json.RawMessage(input)
		_ = parseDescription(raw)
	})
}

func FuzzParseRichDesc(f *testing.F) {
	seeds := []string{
		`null`,
		`"plain text"`,
		`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`,
		`{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":3},"content":[{"type":"text","text":"Title"}]}]}`,
		`{}`,
		`invalid`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		raw := json.RawMessage(input)
		segs := ParseRichDesc(raw)
		for _, seg := range segs {
			_ = seg.Text
			_ = seg.Style
		}
	})
}