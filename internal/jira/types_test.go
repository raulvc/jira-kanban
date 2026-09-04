package jira

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDescription_Nil(t *testing.T) {
	is := assert.New(t)
	is.Empty(parseDescription(nil))
}

func TestParseDescription_NullJSON(t *testing.T) {
	is := assert.New(t)
	is.Empty(parseDescription(json.RawMessage("null")))
}

func TestParseDescription_PlainString(t *testing.T) {
	is := assert.New(t)
	is.Equal("plain text", parseDescription(json.RawMessage(`"plain text"`)))
}

func TestParseDescription_ADFObject(t *testing.T) {
	is := assert.New(t)
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`)
	is.Equal("hello", parseDescription(raw))
}

func TestAdfToPlain_Nil(t *testing.T) {
	is := assert.New(t)
	is.Empty(adfToPlain(nil))
}

func TestAdfToPlain_SimpleParagraph(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "paragraph", Content: []adfNode{
				{Type: "text", Text: "Hello world"},
			}},
		},
	}
	is.Equal("Hello world", adfToPlain(doc))
}

func TestAdfToPlain_MultipleParagraphs(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "paragraph", Content: []adfNode{
				{Type: "text", Text: "First"},
			}},
			{Type: "paragraph", Content: []adfNode{
				{Type: "text", Text: "Second"},
			}},
		},
	}
	is.Equal("First\nSecond", adfToPlain(doc))
}

func TestAdfToPlain_BulletList(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "bulletList", Content: []adfNode{
				{Type: "listItem", Content: []adfNode{
					{Type: "paragraph", Content: []adfNode{{Type: "text", Text: "Item 1"}}},
				}},
				{Type: "listItem", Content: []adfNode{
					{Type: "paragraph", Content: []adfNode{{Type: "text", Text: "Item 2"}}},
				}},
			}},
		},
	}
	is.Equal("• Item 1\n• Item 2", adfToPlain(doc))
}

func TestAdfToPlain_HardBreak(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "paragraph", Content: []adfNode{
				{Type: "text", Text: "Line 1"},
				{Type: "hardBreak"},
				{Type: "text", Text: "Line 2"},
			}},
		},
	}
	is.Equal("Line 1\nLine 2", adfToPlain(doc))
}

func TestAdfToPlain_EmptyDoc(t *testing.T) {
	is := assert.New(t)
	is.Empty(adfToPlain(&adfDoc{Type: "doc", Version: 1}))
}

func TestAdfToPlain_CodeBlockWithFences(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "codeBlock", Attrs: adfAttrs{Language: "go"}, Content: []adfNode{
				{Type: "text", Text: "fmt.Println()"},
			}},
		},
	}
	result := adfToPlain(doc)
	is.Contains(result, "```go", "plain text should include language fence")
	is.Contains(result, "fmt.Println()", "plain text should include code content")
	is.Contains(result, "```", "plain text should include closing fence")
}

func TestAdfToPlain_CodeBlockNoLanguage(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "codeBlock", Content: []adfNode{
				{Type: "text", Text: "echo hello"},
			}},
		},
	}
	result := adfToPlain(doc)
	is.Contains(result, "```\n", "plain text should include bare fence")
	is.Contains(result, "echo hello")
}

func TestAdfToPlain_HeadingWithPrefix(t *testing.T) {
	is := assert.New(t)
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "heading", Attrs: adfAttrs{Level: 2}, Content: []adfNode{
				{Type: "text", Text: "My Heading"},
			}},
		},
	}
	result := adfToPlain(doc)
	is.Contains(result, "## My Heading", "plain text should include heading prefix")
}

func TestDescToADF_HeadingRoundTrip(t *testing.T) {
	is := assert.New(t)
	adf := descToADF("## My Heading\n\nSome text")
	is.NotEmpty(adf["content"])
	content := adf["content"].([]map[string]any)
	is.GreaterOrEqual(len(content), 1)
	is.Equal("heading", content[0]["type"])
}

func TestDescToADF_CodeBlockRoundTrip(t *testing.T) {
	is := assert.New(t)
	adf := descToADF("```go\nfmt.Println()\n```")
	content := adf["content"].([]map[string]any)
	is.GreaterOrEqual(len(content), 1)
	is.Equal("codeBlock", content[0]["type"])
}

func TestParseHeadingLine(t *testing.T) {
	is := assert.New(t)
	text, level := parseHeadingLine("## Hello")
	is.Equal(2, level)
	is.Equal("Hello", text)

	text, level = parseHeadingLine("###### Deep")
	is.Equal(6, level)
	is.Equal("Deep", text)

	text, level = parseHeadingLine("# Top")
	is.Equal(1, level)
	is.Equal("Top", text)

	_, level = parseHeadingLine("Not a heading")
	is.Equal(0, level)

	_, level = parseHeadingLine("####### Too deep")
	is.Equal(0, level)
}

func TestGetIssue_Success(t *testing.T) {
	fj := newFakeJira()
	defer fj.close()

	fj.handle("/rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary": "Test issue",
				"status":  map[string]string{"id": "1", "name": "To Do"},
				"assignee": map[string]string{
					"displayName": "Alice",
				},
				"reporter": map[string]string{
					"displayName": "Bob",
				},
				"labels": []string{"bug"},
				"description": map[string]any{
					"type":    "doc",
					"version": 1,
					"content": []map[string]any{
						{
							"type": "paragraph",
							"content": []map[string]any{
								{"type": "text", "text": "Some description"},
							},
						},
					},
				},
			},
		}
		jsonResponse(w, resp)
	})

	must := require.New(t)
	is := assert.New(t)
	card, err := fj.client().GetIssue("PROJ-1")
	must.NoError(err)
	is.Equal("PROJ-1", card.Key)
	is.Equal("Test issue", card.Summary)
	is.Equal("Some description", card.Description)
	is.Equal("Alice", card.Assignee)
	is.Equal("Bob", card.Reporter)
}

func TestGetIssue_NotFound(t *testing.T) {
	fj := newFakeJira()
	defer fj.close()

	fj.handle("/rest/api/3/issue/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := fj.client().GetIssue("PROJ-999")
	assert.Error(t, err, "expected error for 404")
}

func TestAdfJSONRoundtrip(t *testing.T) {
	must := require.New(t)
	is := assert.New(t)
	raw := `{
		"type": "doc",
		"version": 1,
		"content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "Hello"}]},
			{"type": "bulletList", "content": [
				{"type": "listItem", "content": [
					{"type": "paragraph", "content": [{"type": "text", "text": "one"}]}
				]},
				{"type": "listItem", "content": [
					{"type": "paragraph", "content": [{"type": "text", "text": "two"}]}
				]}
			]}
		]
	}`
	var doc adfDoc
	must.NoError(json.Unmarshal([]byte(raw), &doc))
	got := adfToPlain(&doc)
	is.Equal("Hello\n• one\n• two", got)
}