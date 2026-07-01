package jira

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// IssueType represents a Jira issue type (Task, Bug, Epic, etc.).
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

// EpicItem represents an epic issue for the create-issue picker.
type EpicItem struct {
	Key      string
	Summary string
	Name     string
}

// CreateIssueResult holds the response from creating an issue.
type CreateIssueResult struct {
	Key string `json:"key"`
	ID  string `json:"id"`
}

// GetIssueTypes returns the available issue types for the given project.
func (c *Client) GetIssueTypes(projectKey string) ([]IssueType, error) {
	u := fmt.Sprintf("%s/rest/api/3/project/%s", c.BaseURL, projectKey)
	var resp struct {
		IssueTypes []IssueType `json:"issueTypes"`
	}
	if err := c.getJSON(u, &resp); err != nil {
		return nil, fmt.Errorf("get issue types for project %s: %w", projectKey, err)
	}
	var out []IssueType
	for _, it := range resp.IssueTypes {
		if !it.Subtask {
			out = append(out, it)
		}
	}
	return out, nil
}

// SearchEpics searches for epic issues in the given project.
func (c *Client) SearchEpics(projectKey, query string) ([]EpicItem, error) {
	jql := fmt.Sprintf("project = %s AND type = Epic", projectKey)
	if query != "" {
		jql += fmt.Sprintf(" AND summary ~ \"%s\"", query)
	}
	jql += " ORDER BY key DESC"

	u := fmt.Sprintf("%s/rest/api/3/search/jql?jql=%s&maxResults=100&fields=summary",
		c.BaseURL, url.QueryEscape(jql))
	var resp searchJqlResponse
	if err := c.getJSON(u, &resp); err != nil {
		return nil, fmt.Errorf("search epics for project %s: %w", projectKey, err)
	}
	out := make([]EpicItem, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		epic := EpicItem{Key: iss.Key, Summary: iss.Fields.Summary}
		if iss.Fields.Epic != nil {
			epic.Name = iss.Fields.Epic.Name
		}
		out = append(out, epic)
	}
	return out, nil
}

// GetSubtaskTypes returns the available subtask issue types for the given project.
func (c *Client) GetSubtaskTypes(projectKey string) ([]IssueType, error) {
	u := fmt.Sprintf("%s/rest/api/3/project/%s", c.BaseURL, projectKey)
	var resp struct {
		IssueTypes []IssueType `json:"issueTypes"`
	}
	if err := c.getJSON(u, &resp); err != nil {
		return nil, fmt.Errorf("get subtask types for project %s: %w", projectKey, err)
	}
	var out []IssueType
	for _, it := range resp.IssueTypes {
		if it.Subtask {
			out = append(out, it)
		}
	}
	return out, nil
}

// CreateSubtask creates a new subtask under a parent issue and returns its key.
func (c *Client) CreateSubtask(projectKey, issueTypeID, summary, description, parentKey string, labels []string) (CreateIssueResult, error) {
	u := fmt.Sprintf("%s/rest/api/3/issue", c.BaseURL)
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"id": issueTypeID},
		"summary":   summary,
		"parent":    map[string]string{"key": parentKey},
	}
	if description != "" {
		fields["description"] = descToADF(description)
	}
	if len(labels) > 0 {
		fields["labels"] = labels
	}
	body := map[string]any{"fields": fields}
	var result CreateIssueResult
	if err := c.postJSONResponse(u, body, &result); err != nil {
		return CreateIssueResult{}, fmt.Errorf("create subtask: %w", err)
	}
	return result, nil
}

// CreateIssue creates a new Jira issue and returns its key.
func (c *Client) CreateIssue(projectKey, issueTypeID, summary, description string, labels []string) (CreateIssueResult, error) {
	u := fmt.Sprintf("%s/rest/api/3/issue", c.BaseURL)
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"id": issueTypeID},
		"summary":   summary,
	}
	if description != "" {
		fields["description"] = descToADF(description)
	}
	if len(labels) > 0 {
		fields["labels"] = labels
	}
	body := map[string]any{"fields": fields}
	var result CreateIssueResult
	if err := c.postJSONResponse(u, body, &result); err != nil {
		return CreateIssueResult{}, fmt.Errorf("create issue: %w", err)
	}
	return result, nil
}

// LinkEpic links an issue to an epic using the Jira Cloud unified parent
// field.  Jira Cloud has consolidated the old Epic Link custom field into
// the parent field for all project types.
func (c *Client) LinkEpic(issueKey, epicKey string) error {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, issueKey)
	body := map[string]any{
		"fields": map[string]any{
			"parent": map[string]string{"key": epicKey},
		},
	}
	err := c.putJSON(u, body)
	if err != nil {
		return fmt.Errorf("link epic %s to %s: %w", epicKey, issueKey, err)
	}
	return nil
}

// ClearEpic removes the epic/parent link from a standard issue.
// Sets fields.parent to null which works for both team-managed and
// company-managed projects in Jira Cloud (the old Epic Link custom field
// has been consolidated into the parent field).
func (c *Client) ClearEpic(issueKey string) error {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, issueKey)
	body := map[string]any{
		"fields": map[string]any{
			"parent": nil,
		},
	}
	if err := c.putJSON(u, body); err != nil {
		return fmt.Errorf("clear epic for %s: %w", issueKey, err)
	}
	return nil
}

// IssueEdit describes which fields to update on an existing issue.
// Nil pointer fields are skipped (not changed); non-nil fields are sent
// even if the value is empty (to clear the field).
type IssueEdit struct {
	Summary     *string
	Description *string
	Labels      *[]string
}

// EditIssue updates only the fields specified in the IssueEdit struct.
// Fields with nil pointers are omitted from the request.
func (c *Client) EditIssue(key string, edit IssueEdit) error {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, key)
	fields := map[string]any{}
	if edit.Summary != nil {
		fields["summary"] = *edit.Summary
	}
	if edit.Description != nil {
		fields["description"] = descToADF(*edit.Description)
	}
	if edit.Labels != nil {
		fields["labels"] = *edit.Labels
	}
	if len(fields) == 0 {
		return nil
	}
	body := map[string]any{"fields": fields}
	if err := c.putJSON(u, body); err != nil {
		return fmt.Errorf("edit issue %s: %w", key, err)
	}
	return nil
}

// urlRe matches http(s) URLs in text.
var urlRe = regexp.MustCompile(`https?://[^\s)\]>}]+`)

// descToADF converts a plain-text description into an ADF document,
// detecting triple-backtick code blocks and URLs.
func descToADF(desc string) map[string]any {
	var content []map[string]any

	lines := strings.Split(desc, "\n")
	i := 0
	for i < len(lines) {
		// Check for code block fence
		if lang, ok := strings.CutPrefix(lines[i], "```"); ok {
			var codeLines []string
			i++
			for i < len(lines) && lines[i] != "```" {
				codeLines = append(codeLines, lines[i])
				i++
			}
			i++ // skip closing ```
			content = append(content, codeBlockNode(strings.Join(codeLines, "\n"), lang))
			continue
		}
		// Accumulate non-code lines into paragraphs
		var paraLines []string
		for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
			paraLines = append(paraLines, lines[i])
			i++
		}
		if len(paraLines) > 0 {
			// Split at blank lines into separate paragraphs
			for _, para := range splitParagraphs(paraLines) {
				if para == "" {
					content = append(content, map[string]any{
						"type":    "paragraph",
						"content": []map[string]any{},
					})
				} else {
					content = append(content, paragraphWithLinks(para))
				}
			}
		}
	}

	if len(content) == 0 {
		content = append(content, map[string]any{
			"type":    "paragraph",
			"content": []map[string]any{},
		})
	}

	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

// splitParagraphs groups consecutive non-blank lines, splitting at blank lines.
func splitParagraphs(lines []string) []string {
	var result []string
	var buf strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if buf.Len() > 0 {
				result = append(result, buf.String())
				buf.Reset()
			}
			result = append(result, "")
		} else if buf.Len() > 0 {
			buf.WriteByte('\n')
			buf.WriteString(line)
		} else {
			buf.WriteString(line)
		}
	}
	if buf.Len() > 0 {
		result = append(result, buf.String())
	}
	return result
}

// paragraphWithLinks builds a paragraph ADF node, splitting text at URL
// boundaries and adding link marks.
func paragraphWithLinks(text string) map[string]any {
	var inline []map[string]any
	last := 0
	for _, m := range urlRe.FindAllStringIndex(text, -1) {
		if m[0] > last {
			inline = append(inline, textNode(text[last:m[0]]))
		}
		inline = append(inline, linkTextNode(text[m[0]:m[1]]))
		last = m[1]
	}
	if last < len(text) {
		inline = append(inline, textNode(text[last:]))
	}
	if len(inline) == 0 {
		inline = append(inline, textNode(text))
	}
	return map[string]any{
		"type":    "paragraph",
		"content": inline,
	}
}

func textNode(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func linkTextNode(url string) map[string]any {
	return map[string]any{
		"type": "text",
		"text": url,
		"marks": []map[string]any{
			{
				"type":  "link",
				"attrs": map[string]string{"href": url},
			},
		},
	}
}

func codeBlockNode(code, lang string) map[string]any {
	attrs := map[string]string{}
	if lang != "" {
		attrs["language"] = lang
	}
	return map[string]any{
		"type": "codeBlock",
		"attrs": attrs,
		"content": []map[string]any{
			textNode(code),
		},
	}
}