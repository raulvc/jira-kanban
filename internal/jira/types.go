package jira

import (
	"encoding/json"
	"fmt"
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
	Ranking struct {
		RankCustomFieldID int `json:"rankCustomFieldId"`
	} `json:"ranking"`
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
		Labels      []string       `json:"labels"`
		Description json.RawMessage `json:"description"`
		Parent      *issueParent    `json:"parent"`
		Epic        *issueEpic      `json:"epic"`
		Subtasks    []issueSubtask  `json:"subtasks"`
	} `json:"fields"`
	Rank string `json:"-"`
}

// extractRanks parses the raw API response to find each issue's rank custom
// field value and sets it on the corresponding issue. The board configuration
// provides rankCustomFieldId which maps to a field named customfield_XXXXX.
func extractRanks(issues []issue, raw []byte, rankFieldID int) {
	if rankFieldID <= 0 || len(raw) == 0 {
		return
	}
	fieldName := fmt.Sprintf("customfield_%d", rankFieldID)
	var resp struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields map[string]json.RawMessage `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}
	rankMap := make(map[string]string, len(resp.Issues))
	for _, ri := range resp.Issues {
		if raw, ok := ri.Fields[fieldName]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				rankMap[ri.Key] = s
			}
		}
	}
	for i := range issues {
		if r, ok := rankMap[issues[i].Key]; ok {
			issues[i].Rank = r
		}
	}
}

type Subtask struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
}

// issueParent is the parent link returned for sub-tasks.
type issueParent struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
	} `json:"fields"`
}

// issueSubtask is a single sub-task entry inside fields.subtasks.
type issueSubtask struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
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
	StatusID    string
	Status      string
	Assignee    string
	Labels      []string
	Description string
	RichDesc    []DescSeg
	Epic        string
	Rank        string
	Subtasks    []Subtask
}

// DescStyle identifies the visual style of a description segment.
type DescStyle int

const (
	DsUnknown  DescStyle = iota // unknown/default
	DsText                       // normal text
	DsLink                       // hyperlink (cyan)
	DsCode                       // code block (dim/indented)
	DsHeading                    // heading (bold)
)

// DescSeg is a styled segment of a description.
type DescSeg struct {
	Text  string
	Style DescStyle
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

// adfMark represents text marks (bold, italic, link, etc).
type adfMark struct {
	Type  string            `json:"type"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// adfAttrs holds node attributes (e.g. level for headings, language for codeBlock).
type adfAttrs struct {
	Level    int    `json:"level,omitempty"`
	Language string `json:"language,omitempty"`
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
		renderADFChildren(b, node.Content)
	case "bulletList", "orderedList":
		renderADFListItems(b, node.Content)
	case "listItem":
		renderADFListItem(b, node.Content)
	case "heading":
		renderADFHeading(b, node)
	case "text", "inlineCard":
		renderADFText(b, node)
	case "hardBreak":
		b.WriteByte('\n')
	case "codeBlock":
		renderADFCodeBlock(b, node)
	case "blockCard", "mediaGroup", "media", "rule":
		renderADFBlockChildren(b, node.Content)
	default:
		renderADFChildren(b, node.Content)
	}
}

func renderADFChildren(b *strings.Builder, children []adfNode) {
	for _, child := range children {
		renderADFNode(b, child)
	}
}

func renderADFListItems(b *strings.Builder, items []adfNode) {
	for i, child := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		renderADFNode(b, child)
	}
}

func renderADFListItem(b *strings.Builder, children []adfNode) {
	b.WriteString("• ")
	for i, child := range children {
		if i > 0 {
			b.WriteByte('\n')
		}
		renderADFNode(b, child)
	}
}

func renderADFHeading(b *strings.Builder, node adfNode) {
	prefix := strings.Repeat("#", node.Attrs.Level) + " "
	b.WriteString(prefix)
	for _, child := range node.Content {
		renderADFNode(b, child)
	}
	b.WriteByte('\n')
}

func renderADFText(b *strings.Builder, node adfNode) {
	b.WriteString(node.Text)
	for _, m := range node.Marks {
		if m.Type == "link" {
			if href := m.Attrs["href"]; href != "" && href != node.Text {
				b.WriteString(" (")
				b.WriteString(href)
				b.WriteByte(')')
			}
		}
	}
}

func renderADFCodeBlock(b *strings.Builder, node adfNode) {
	lang := node.Attrs.Language
	if lang != "" {
		b.WriteString("```")
		b.WriteString(lang)
		b.WriteByte('\n')
	} else {
		b.WriteString("```\n")
	}
	for _, child := range node.Content {
		renderADFNode(b, child)
	}
	b.WriteString("\n```")
}

func renderADFBlockChildren(b *strings.Builder, children []adfNode) {
	for _, child := range children {
		renderADFNode(b, child)
		b.WriteByte('\n')
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

// ParseRichDesc converts raw ADF JSON into styled description segments.
func ParseRichDesc(raw json.RawMessage) []DescSeg {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var doc adfDoc
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Type != "doc" {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return []DescSeg{{Text: s, Style: DsUnknown}}
		}
		return nil
	}
	var segs []DescSeg
	for i, node := range doc.Content {
		if i > 0 {
			segs = append(segs, DescSeg{Text: "\n"})
		}
		segs = appendRichNode(segs, node)
	}
	return segs
}

func appendRichNode(segs []DescSeg, node adfNode) []DescSeg {
	switch node.Type {
	case "paragraph":
		for _, child := range node.Content {
			segs = appendRichNode(segs, child)
		}
	case "heading":
		segs = appendRichHeading(segs, node)
	case "bulletList", "orderedList":
		for i, child := range node.Content {
			if i > 0 {
				segs = append(segs, DescSeg{Text: "\n"})
			}
			segs = appendRichNode(segs, child)
		}
	case "listItem":
		segs = append(segs, DescSeg{Text: "• "})
		for i, child := range node.Content {
			if i > 0 {
				segs = append(segs, DescSeg{Text: "\n"})
			}
			segs = appendRichNode(segs, child)
		}
	case "text", "inlineCard":
		segs = appendRichText(segs, node)
	case "hardBreak":
		segs = append(segs, DescSeg{Text: "\n"})
	case "codeBlock":
		segs = appendRichCodeBlock(segs, node)
	case "blockCard", "mediaGroup", "media", "rule":
		for _, child := range node.Content {
			segs = appendRichNode(segs, child)
			segs = append(segs, DescSeg{Text: "\n"})
		}
	default:
		for _, child := range node.Content {
			segs = appendRichNode(segs, child)
		}
	}
	return segs
}

func appendRichHeading(segs []DescSeg, node adfNode) []DescSeg {
	prefix := strings.Repeat("#", node.Attrs.Level) + " "
	segs = append(segs, DescSeg{Text: prefix, Style: DsHeading})
	for _, child := range node.Content {
		segs = append(segs, DescSeg{Text: child.Text, Style: DsHeading})
	}
	segs = append(segs, DescSeg{Text: "\n"})
	return segs
}

func appendRichText(segs []DescSeg, node adfNode) []DescSeg {
	style := DsText
	for _, m := range node.Marks {
		if m.Type == "link" {
			style = DsLink
		}
	}
	segs = append(segs, DescSeg{Text: node.Text, Style: style})
	for _, m := range node.Marks {
		if m.Type == "link" {
			if href := m.Attrs["href"]; href != "" && href != node.Text {
				segs = append(segs, DescSeg{Text: " (" + href + ")", Style: DsLink})
			}
		}
	}
	return segs
}

func appendRichCodeBlock(segs []DescSeg, node adfNode) []DescSeg {
	prefix := "```\n"
	if lang := node.Attrs.Language; lang != "" {
		prefix = "```" + lang + "\n"
	}
	segs = append(segs, DescSeg{Text: prefix, Style: DsCode})
	for _, child := range node.Content {
		segs = append(segs, DescSeg{Text: child.Text, Style: DsCode})
	}
	segs = append(segs, DescSeg{Text: "\n```", Style: DsCode})
	return segs
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
