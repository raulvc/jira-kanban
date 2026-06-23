package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRichDesc_Nil(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	is.Nil(ParseRichDesc(nil))
}

func TestParseRichDesc_NullJSON(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	is.Nil(ParseRichDesc(json.RawMessage("null")))
}

func TestParseRichDesc_PlainString(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`"some text"`)
	got := ParseRichDesc(raw)
	is.Len(got, 1)
	is.Equal("some text", got[0].Text)
	is.Equal(DsUnknown, got[0].Style)
}

func TestParseRichDesc_EmptyString(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	is.Nil(ParseRichDesc(json.RawMessage(`""`)))
}

func TestParseRichDesc_ADFParagraph(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`)
	got := ParseRichDesc(raw)
	is.Len(got, 1)
	is.Equal("hello", got[0].Text)
	is.Equal(DsText, got[0].Style)
}

func TestParseRichDesc_ADFWithLink(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"https://example.com","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}`)
	got := ParseRichDesc(raw)
	is.True(len(got) >= 1, "expected at least 1 segment")
	is.Equal(DsLink, got[0].Style)
}

func TestParseRichDesc_ADFWithCodeBlock(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println()"}]}]}`)
	got := ParseRichDesc(raw)
	found := false
	for _, seg := range got {
		if seg.Style == DsCode {
			found = true
		}
	}
	is.True(found, "expected DsCode segment")
}

func TestParseRichDesc_MultipleParagraphs(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]},{"type":"paragraph","content":[{"type":"text","text":"two"}]}]}`)
	got := ParseRichDesc(raw)
	is.Len(got, 3, "expected 3 segments (text, newline, text)")
	is.Equal("\n", got[1].Text)
}

func TestParseRichDesc_BulletList(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"item"}]}]}]}]}`)
	got := ParseRichDesc(raw)
	is.NotEmpty(got)
	is.Equal("• ", got[0].Text)
}

func TestParseRichDesc_HardBreak(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"a"},{"type":"hardBreak"},{"type":"text","text":"b"}]}]}`)
	got := ParseRichDesc(raw)
	foundBreak := false
	for _, seg := range got {
		if seg.Text == "\n" {
			foundBreak = true
		}
	}
	is.True(foundBreak, "expected newline for hardBreak")
}

func TestParseRichDesc_Heading(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]}]}`)
	got := ParseRichDesc(raw)
	is.True(len(got) >= 2, "expected at least 2 segments (prefix + text)")
	is.Equal(DsHeading, got[0].Style)
	is.Equal("## ", got[0].Text)
}

func TestParseRichDesc_InvalidJSON(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	is.Nil(ParseRichDesc(json.RawMessage(`{invalid`)))
}

func TestAppendRichText_LinkWithDifferentURL(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{
		Type: "text",
		Text: "click here",
		Marks: []adfMark{
			{Type: "link", Attrs: map[string]string{"href": "https://example.com"}},
		},
	}
	segs := appendRichText(nil, node)
	is.Len(segs, 2, "expected 2 segments (text + href)")
	is.Equal(DsLink, segs[0].Style)
	is.Equal(" (https://example.com)", segs[1].Text)
}

func TestAppendRichText_LinkWithSameURL(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{
		Type: "text",
		Text: "https://example.com",
		Marks: []adfMark{
			{Type: "link", Attrs: map[string]string{"href": "https://example.com"}},
		},
	}
	segs := appendRichText(nil, node)
	is.Len(segs, 1, "same URL should not add href suffix")
}

func TestAppendRichCodeBlock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		node      adfNode
		wantLang  bool
		wantLangV string
	}{
		{
			name:      "with language",
			node:      adfNode{Type: "codeBlock", Attrs: adfAttrs{Language: "python"}, Content: []adfNode{{Type: "text", Text: "print('hi')"}}},
			wantLang:  true,
			wantLangV: "python",
		},
		{
			name:      "without language",
			node:      adfNode{Type: "codeBlock", Content: []adfNode{{Type: "text", Text: "code"}}},
			wantLang:  false,
			wantLangV: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			segs := appendRichCodeBlock(nil, tt.node)
			is.NotEmpty(segs)
			is.Equal(DsCode, segs[0].Style)
			if tt.wantLang {
				is.Equal("```python\n", segs[0].Text)
			} else {
				is.Equal("```\n", segs[0].Text)
			}
			last := segs[len(segs)-1]
			is.Equal("\n```", last.Text)
			is.Equal(DsCode, last.Style)
		})
	}
}