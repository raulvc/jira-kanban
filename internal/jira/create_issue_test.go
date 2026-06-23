package jira

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescToADF_PlainText(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	got := descToADF("hello world")
	doc := got
	is.Equal("doc", doc["type"])
	is.Equal(1, doc["version"])
	content := doc["content"].([]map[string]any)
	is.Len(content, 1)
	is.Equal("paragraph", content[0]["type"])
}

func TestDescToADF_CodeBlock(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	must := require.New(t)
	desc := "```go\nfmt.Println(\"hi\")\n```\nAfter code"
	got := descToADF(desc)
	content := got["content"].([]map[string]any)
	is.Len(content, 2)
	is.Equal("codeBlock", content[0]["type"])
	attrs := content[0]["attrs"].(map[string]string)
	is.Equal("go", attrs["language"])
	is.Equal("paragraph", content[1]["type"])
	must.NotNil(attrs)
}

func TestDescToADF_CodeBlockNoLang(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	desc := "```\nplain code\n```"
	got := descToADF(desc)
	content := got["content"].([]map[string]any)
	is.Equal("codeBlock", content[0]["type"])
	attrs := content[0]["attrs"].(map[string]string)
	_, ok := attrs["language"]
	is.False(ok, "empty language should not be set in attrs")
}

func TestDescToADF_URLs(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	desc := "See https://example.com for details"
	got := descToADF(desc)
	content := got["content"].([]map[string]any)
	para := content[0]
	inline := para["content"].([]map[string]any)
	hasLink := false
	for _, node := range inline {
		if marks, ok := node["marks"]; ok {
			for _, m := range marks.([]map[string]any) {
				if m["type"] == "link" {
					hasLink = true
				}
			}
		}
	}
	is.True(hasLink, "expected link mark in paragraph with URL")
}

func TestDescToADF_BlankLineSplit(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	desc := "Para 1\n\nPara 2"
	got := descToADF(desc)
	content := got["content"].([]map[string]any)
	is.Len(content, 3)
	is.Equal("paragraph", content[0]["type"])
	is.Equal("paragraph", content[1]["type"])
	is.Equal("paragraph", content[2]["type"])
}

func TestDescToADF_Empty(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	got := descToADF("")
	content := got["content"].([]map[string]any)
	is.Len(content, 1, "empty input should produce 1 empty paragraph")
	is.Equal("paragraph", content[0]["type"])
}

func TestSplitParagraphs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{name: "single line", lines: []string{"hello"}, want: []string{"hello"}},
		{name: "two lines no blank", lines: []string{"hello", "world"}, want: []string{"hello\nworld"}},
		{name: "split at blank", lines: []string{"hello", "", "world"}, want: []string{"hello", "", "world"}},
		{name: "trailing blank", lines: []string{"hello", ""}, want: []string{"hello", ""}},
		{name: "leading blank", lines: []string{"", "hello"}, want: []string{"", "hello"}},
		{name: "multiple blanks", lines: []string{"a", "", "", "b"}, want: []string{"a", "", "", "b"}},
		{name: "empty input", lines: []string{}, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			got := splitParagraphs(tt.lines)
			is.Len(got, len(tt.want))
			for i := range got {
				is.Equal(tt.want[i], got[i], "paragraph %d", i)
			}
		})
	}
}

func TestParagraphWithLinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		text      string
		wantNodes int
		wantLink  bool
	}{
		{name: "plain text", text: "hello", wantNodes: 1, wantLink: false},
		{name: "url in middle", text: "see https://example.com now", wantNodes: 3, wantLink: true},
		{name: "url at start", text: "https://example.com is great", wantNodes: 2, wantLink: true},
		{name: "url at end", text: "click https://example.com", wantNodes: 2, wantLink: true},
		{name: "only url", text: "https://example.com", wantNodes: 1, wantLink: true},
		{name: "multiple urls", text: "https://a.com and https://b.com", wantNodes: 3, wantLink: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			node := paragraphWithLinks(tt.text)
			is.Equal("paragraph", node["type"])
			inline := node["content"].([]map[string]any)
			is.Len(inline, tt.wantNodes)
			foundLink := false
			for _, n := range inline {
				if marks, ok := n["marks"]; ok {
					for _, m := range marks.([]map[string]any) {
						if m["type"] == "link" {
							foundLink = true
						}
					}
				}
			}
			is.Equal(tt.wantLink, foundLink)
		})
	}
}

func TestCodeBlockNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		code string
		lang string
	}{
		{name: "with language", code: "print('hi')", lang: "python"},
		{name: "without language", code: "echo hi", lang: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			node := codeBlockNode(tt.code, tt.lang)
			is.Equal("codeBlock", node["type"])
			attrs := node["attrs"].(map[string]string)
			if tt.lang != "" {
				is.Equal(tt.lang, attrs["language"])
			} else {
				_, ok := attrs["language"]
				is.False(ok, "empty language should not be in attrs")
			}
		})
	}
}

func TestTextNode(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	node := textNode("hello")
	is.Equal("text", node["type"])
	is.Equal("hello", node["text"])
}

func TestLinkTextNode(t *testing.T) {
	t.Parallel()
	is := assert.New(t)
	url := "https://example.com"
	node := linkTextNode(url)
	is.Equal("text", node["type"])
	is.Equal(url, node["text"])
	marks := node["marks"].([]map[string]any)
	is.Equal("link", marks[0]["type"])
	attrs := marks[0]["attrs"].(map[string]string)
	is.Equal(url, attrs["href"])
}

func TestDescToADF_Roundtrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		desc string
	}{
		{name: "simple", desc: "Hello world"},
		{name: "multiline", desc: "Line 1\nLine 2\n\nNew para"},
		{name: "with code", desc: "Before\n```go\nfmt.Println()\n```\nAfter"},
		{name: "with url", desc: "Visit https://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			must := require.New(t)
			is := assert.New(t)
			adf := descToADF(tt.desc)
			raw, err := json.Marshal(adf)
			must.NoError(err, "marshal ADF")
			var doc adfDoc
			must.NoError(json.Unmarshal(raw, &doc), "unmarshal ADF")
			is.Equal("doc", doc.Type)
		})
	}
}