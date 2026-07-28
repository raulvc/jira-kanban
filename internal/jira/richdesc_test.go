package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
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
		if seg.Style == DsCodeBlock {
			found = true
		}
	}
	is.True(found, "expected DsCodeBlock segment")
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
	is.True(len(got) >= 1, "expected at least 1 segment")
	is.Equal(DsHeading, got[0].Style)
	is.Equal("Title", got[0].Text)
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
		name string
		node adfNode
	}{
		{
			name: "with language",
			node: adfNode{Type: "codeBlock", Attrs: adfAttrs{Language: "python"}, Content: []adfNode{{Type: "text", Text: "print('hi')"}}},
		},
		{
			name: "without language",
			node: adfNode{Type: "codeBlock", Content: []adfNode{{Type: "text", Text: "code"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			segs := appendRichCodeBlock(nil, tt.node)
			is.NotEmpty(segs)
			is.Equal(DsCodeBlock, segs[0].Style)
			// Code blocks no longer emit ``` fences; they use styled segments
			last := segs[len(segs)-1]
			is.Equal(DsCodeBlock, last.Style)
		})
	}
}

func TestAppendRichText_BoldMark(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{Type: "text", Text: "important", Marks: []adfMark{{Type: "strong"}}}
	segs := appendRichText(nil, node)
	is.Len(segs, 1)
	is.Equal("important", segs[0].Text)
	is.True(segs[0].Style&DsBold != 0, "expected DsBold flag")
}

func TestAppendRichText_ItalicMark(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{Type: "text", Text: "emphasis", Marks: []adfMark{{Type: "em"}}}
	segs := appendRichText(nil, node)
	is.Len(segs, 1)
	is.True(segs[0].Style&DsItalic != 0, "expected DsItalic flag")
}

func TestAppendRichText_StrikethroughMark(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{Type: "text", Text: "deleted", Marks: []adfMark{{Type: "strike"}}}
	segs := appendRichText(nil, node)
	is.Len(segs, 1)
	is.True(segs[0].Style&DsStrikethrough != 0, "expected DsStrikethrough flag")
}

func TestAppendRichText_InlineCodeMark(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{Type: "text", Text: "x := 1", Marks: []adfMark{{Type: "code"}}}
	segs := appendRichText(nil, node)
	is.Len(segs, 1)
	is.True(segs[0].Style&DsInlineCode != 0, "expected DsInlineCode flag")
}

func TestAppendRichText_UnderlineMark(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{Type: "text", Text: "underlined", Marks: []adfMark{{Type: "underline"}}}
	segs := appendRichText(nil, node)
	is.Len(segs, 1)
	is.True(segs[0].Style&DsUnderline != 0, "expected DsUnderline flag")
}

func TestAppendRichText_CombinedMarks(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := adfNode{
		Type: "text", Text: "bold italic link",
		Marks: []adfMark{
			{Type: "strong"},
			{Type: "em"},
			{Type: "link", Attrs: map[string]string{"href": "https://example.com"}},
		},
	}
	segs := appendRichText(nil, node)
	is.Len(segs, 2, "expected text seg + link href seg")
	is.True(segs[0].Style&DsBold != 0, "expected DsBold")
	is.True(segs[0].Style&DsItalic != 0, "expected DsItalic")
	is.True(segs[0].Style&DsLink != 0, "expected DsLink")
}

func TestParseRichDesc_OrderedList(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}]}]}`)
	got := ParseRichDesc(raw)
	is.NotEmpty(got)
	is.Equal("1. ", got[0].Text)
	// Find the second item prefix
	foundSecond := false
	for _, seg := range got {
		if seg.Text == "2. " {
			foundSecond = true
		}
	}
	is.True(foundSecond, "expected '2. ' prefix for second ordered list item")
}

func TestParseRichDesc_BulletListPrefix(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"item"}]}]}]}]}`)
	got := ParseRichDesc(raw)
	is.NotEmpty(got)
	is.Equal("• ", got[0].Text)
}

func TestParseRichDesc_Rule(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"before"}]},{"type":"rule"},{"type":"paragraph","content":[{"type":"text","text":"after"}]}]}`)
	got := ParseRichDesc(raw)
	foundRule := false
	for _, seg := range got {
		if seg.Style&DsBlockquote != 0 && strings.Contains(seg.Text, "─") {
			foundRule = true
		}
	}
	is.True(foundRule, "expected a horizontal rule segment")
}

func TestParseRichDesc_Blockquote(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted text"}]}]}]}`)
	got := ParseRichDesc(raw)
	is.NotEmpty(got)
	foundText := false
	for _, seg := range got {
		if seg.Text == "quoted text" {
			foundText = true
		}
	}
	is.True(foundText, "expected blockquote text to be preserved")
}

func TestParseRichDesc_HeadingNoPrefix(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"My Heading"}]}]}`)
	got := ParseRichDesc(raw)
	is.True(len(got) >= 1)
	is.Equal("My Heading", got[0].Text)
	is.Equal(DsHeading, got[0].Style)
	// No "#" prefix should be present
	for _, seg := range got {
		is.NotContains(seg.Text, "#")
	}
}

func TestParseRichDesc_CodeBlockNoFences(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println()"}]}]}`)
	got := ParseRichDesc(raw)
	for _, seg := range got {
		is.NotContains(seg.Text, "```", "code blocks should not emit fence markers")
	}
}

func TestDescStyleBitmask_Combinable(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	combined := DsBold | DsItalic | DsLink
	is.True(combined&DsBold != 0)
	is.True(combined&DsItalic != 0)
	is.True(combined&DsLink != 0)
	is.True(combined&DsStrikethrough == 0)
}

func TestHighlightCode_GoKeywords(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("func main() {}", "go")
	is.NotEmpty(segs)
	foundKeyword := false
	for _, seg := range segs {
		if seg.Style&DsKeyword != 0 && strings.Contains(seg.Text, "func") {
			foundKeyword = true
		}
	}
	is.True(foundKeyword, "expected 'func' to be highlighted as keyword")
}

func TestHighlightCode_GoStrings(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode(`s := "hello"`, "go")
	is.NotEmpty(segs)
	foundString := false
	for _, seg := range segs {
		if seg.Style&DsString != 0 && strings.Contains(seg.Text, "hello") {
			foundString = true
		}
	}
	is.True(foundString, "expected string literal to be highlighted")
}

func TestHighlightCode_GoComments(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("// comment\nx := 1", "go")
	is.NotEmpty(segs)
	foundComment := false
	for _, seg := range segs {
		if seg.Style&DsComment != 0 && strings.Contains(seg.Text, "comment") {
			foundComment = true
		}
	}
	is.True(foundComment, "expected comment to be highlighted")
}

func TestHighlightCode_GoNumbers(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("x := 42", "go")
	is.NotEmpty(segs)
	foundNumber := false
	for _, seg := range segs {
		if seg.Style&DsNumber != 0 && strings.Contains(seg.Text, "42") {
			foundNumber = true
		}
	}
	is.True(foundNumber, "expected number to be highlighted")
}

func TestHighlightCode_PythonKeywords(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("def foo():\n    pass", "python")
	is.NotEmpty(segs)
	foundKeyword := false
	for _, seg := range segs {
		if seg.Style&DsKeyword != 0 && (strings.Contains(seg.Text, "def") || strings.Contains(seg.Text, "pass")) {
			foundKeyword = true
		}
	}
	is.True(foundKeyword, "expected Python keywords to be highlighted")
}

func TestHighlightCode_UnknownLanguage(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("some random text", "totally-not-a-language")
	// Should either return nil (no lexer found) or segments without syntax flags
	for _, seg := range segs {
		is.True(seg.Style&DsKeyword == 0, "unknown language should not produce keyword highlighting")
	}
}

func TestHighlightCode_EmptyCode(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("", "go")
	// Should return nil or empty for empty input
	is.True(len(segs) == 0 || (len(segs) == 1 && segs[0].Text == ""))
}

func TestHighlightCode_AllSegmentsHaveCodeBlockFlag(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	segs := highlightCode("func main() {}", "go")
	is.NotEmpty(segs)
	for _, seg := range segs {
		is.True(seg.Style&DsCodeBlock != 0, "all highlighted segments should have DsCodeBlock flag")
	}
}

func TestChromaToDescStyle_Mappings(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	tests := []struct {
		name  string
		input chroma.TokenType
		want  DescStyle
	}{
		{"keyword", chroma.Keyword, DsKeyword},
		{"keyword declaration", chroma.KeywordDeclaration, DsKeyword},
		{"string", chroma.String, DsString},
		{"string double", chroma.StringDouble, DsString},
		{"number", chroma.Number, DsNumber},
		{"number float", chroma.NumberFloat, DsNumber},
		{"comment", chroma.Comment, DsComment},
		{"comment single", chroma.CommentSingle, DsComment},
		{"function name", chroma.NameFunction, DsFuncName},
		{"operator", chroma.Operator, DsNormal},
		{"punctuation", chroma.Punctuation, DsNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := chromaToDescStyle(tt.input)
			is.Equal(tt.want, got)
		})
	}
}

func TestParseRichDesc_CodeBlockWithSyntaxHighlighting(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"func main() {}"}]}]}`)
	got := ParseRichDesc(raw)
	is.NotEmpty(got)
	foundKeyword := false
	for _, seg := range got {
		if seg.Style&DsKeyword != 0 {
			foundKeyword = true
		}
	}
	is.True(foundKeyword, "expected syntax highlighting in parsed code block")
}

func TestParseRichDesc_CodeBlockNoLanguageFallsBack(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"codeBlock","content":[{"type":"text","text":"plain code"}]}]}`)
	got := ParseRichDesc(raw)
	is.NotEmpty(got)
	// Should still have DsCodeBlock styled segments
	foundCodeBlock := false
	for _, seg := range got {
		if seg.Style&DsCodeBlock != 0 {
			foundCodeBlock = true
		}
	}
	is.True(foundCodeBlock, "expected DsCodeBlock segments even without language")
}