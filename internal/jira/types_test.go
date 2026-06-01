package jira

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestParseDescription_Nil(t *testing.T) {
	got := parseDescription(nil)
	if got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}
}

func TestParseDescription_NullJSON(t *testing.T) {
	got := parseDescription(json.RawMessage("null"))
	if got != "" {
		t.Fatalf("expected empty for null, got %q", got)
	}
}

func TestParseDescription_PlainString(t *testing.T) {
	got := parseDescription(json.RawMessage(`"plain text"`))
	if got != "plain text" {
		t.Fatalf("expected %q, got %q", "plain text", got)
	}
}

func TestParseDescription_ADFObject(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`)
	got := parseDescription(raw)
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestAdfToPlain_Nil(t *testing.T) {
	got := adfToPlain(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil, got %q", got)
	}
}

func TestAdfToPlain_SimpleParagraph(t *testing.T) {
	doc := &adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfNode{
			{Type: "paragraph", Content: []adfNode{
				{Type: "text", Text: "Hello world"},
			}},
		},
	}
	got := adfToPlain(doc)
	if got != "Hello world" {
		t.Fatalf("expected %q, got %q", "Hello world", got)
	}
}

func TestAdfToPlain_MultipleParagraphs(t *testing.T) {
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
	got := adfToPlain(doc)
	if got != "First\nSecond" {
		t.Fatalf("expected %q, got %q", "First\nSecond", got)
	}
}

func TestAdfToPlain_BulletList(t *testing.T) {
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
	got := adfToPlain(doc)
	want := "• Item 1\n• Item 2"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestAdfToPlain_HardBreak(t *testing.T) {
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
	got := adfToPlain(doc)
	if got != "Line 1\nLine 2" {
		t.Fatalf("expected %q, got %q", "Line 1\nLine 2", got)
	}
}

func TestAdfToPlain_EmptyDoc(t *testing.T) {
	doc := &adfDoc{Type: "doc", Version: 1}
	got := adfToPlain(doc)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
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

	card, err := fj.client().GetIssue("PROJ-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.Key != "PROJ-1" {
		t.Fatalf("expected key PROJ-1, got %s", card.Key)
	}
	if card.Summary != "Test issue" {
		t.Fatalf("expected summary 'Test issue', got %s", card.Summary)
	}
	if card.Description != "Some description" {
		t.Fatalf("expected description 'Some description', got %s", card.Description)
	}
	if card.Assignee != "Alice" {
		t.Fatalf("expected assignee Alice, got %s", card.Assignee)
	}
}

func TestGetIssue_NotFound(t *testing.T) {
	fj := newFakeJira()
	defer fj.close()

	fj.handle("/rest/api/3/issue/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := fj.client().GetIssue("PROJ-999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestAdfJSONRoundtrip(t *testing.T) {
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
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := adfToPlain(&doc)
	want := "Hello\n• one\n• two"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}