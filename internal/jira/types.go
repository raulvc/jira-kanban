package jira

import (
	"encoding/json"
	"strings"
)

// boardConfigResponse is the JSON shape returned by GET /rest/agile/1.0/board/{id}/configuration.
type boardConfigResponse struct {
	Name         string `json:"name"`
	ColumnConfig struct {
		Columns []struct {
			Name     string `json:"name"`
			Statuses []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"statuses"`
		} `json:"columns"`
	} `json:"columnConfig"`
}

// boardIssuesResponse is the JSON shape returned by GET /rest/agile/1.0/board/{id}/issue.
type boardIssuesResponse struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []issue `json:"issues"`
}

// searchJqlResponse is the JSON shape returned by POST /rest/api/3/search/jql.
type searchJqlResponse struct {
	Issues        []issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken"`
}

// issue is a single Jira issue as returned by the board issues endpoint.
type issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Labels      []string         `json:"labels"`
		Description json.RawMessage   `json:"description"`
		Parent      *issueParent      `json:"parent"`
		Epic        *issueEpic        `json:"epic"`
	} `json:"fields"`
}

// issueParent is the parent link returned for sub-tasks.
type issueParent struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
	} `json:"fields"`
}

// issueEpic is the epic link returned for epic-linked issues.
type issueEpic struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Summary string `json:"summary"`
}

// Card is the application-level representation of an issue on the board.
type Card struct {
	Key         string
	Summary     string
	Status      string
	Assignee    string
	Labels      []string
	Description string
	Epic        string
}

// Column groups cards under a named board column.
type Column struct {
	Name   string
	Issues []Card
}

// Board holds the complete board state ready for display.
type Board struct {
	Name    string
	Columns []Column
}

// adfDoc represents the top-level Atlassian Document Format node.
type adfDoc struct {
	Type    string     `json:"type"`
	Version int        `json:"version"`
	Content []adfNode  `json:"content"`
}

// adfNode is a generic node inside an ADF document.
type adfNode struct {
	Type    string    `json:"type"`
	Content []adfNode `json:"content"`
	Text    string    `json:"text"`
	Marks   []adfMark `json:"marks"`
	Attrs   adfAttrs  `json:"attrs"`
}

// adfMark represents text marks (bold, italic, etc).
type adfMark struct {
	Type string `json:"type"`
}

// adfAttrs holds node attributes (e.g. level for headings).
type adfAttrs struct {
	Level int `json:"level"`
}

// adfToPlain extracts plain text from an ADF document, adding newlines
// between paragraphs and preserving list structure.
func adfToPlain(doc *adfDoc) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	for i, node := range doc.Content {
		if i > 0 {
			b.WriteByte('\n')
		}
		renderADFNode(&b, node)
	}
	return b.String()
}

func renderADFNode(b *strings.Builder, node adfNode) {
	switch node.Type {
	case "paragraph":
		for _, child := range node.Content {
			renderADFNode(b, child)
		}
	case "bulletList", "orderedList":
		for i, child := range node.Content {
			if i > 0 {
				b.WriteByte('\n')
			}
			renderADFNode(b, child)
		}
	case "listItem":
		b.WriteString("• ")
		for i, child := range node.Content {
			if i > 0 {
				b.WriteByte('\n')
			}
			renderADFNode(b, child)
		}
	case "heading":
		for _, child := range node.Content {
			renderADFNode(b, child)
		}
		b.WriteByte('\n')
	case "text", "inlineCard":
		b.WriteString(node.Text)
	case "hardBreak":
		b.WriteByte('\n')
	case "codeBlock", "blockCard", "mediaGroup", "media", "rule":
		for _, child := range node.Content {
			renderADFNode(b, child)
			b.WriteByte('\n')
		}
	default:
		for _, child := range node.Content {
			renderADFNode(b, child)
		}
	}
}

func parseDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// Try as ADF object first.
	var doc adfDoc
	if err := json.Unmarshal(raw, &doc); err == nil && doc.Type == "doc" {
		return adfToPlain(&doc)
	}
	// Try as plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

// VisibleKeys returns the set of issue keys present on the board.
func (b Board) VisibleKeys() map[string]bool {
	keys := make(map[string]bool)
	for _, col := range b.Columns {
		for _, card := range col.Issues {
			keys[card.Key] = true
		}
	}
	return keys
}
