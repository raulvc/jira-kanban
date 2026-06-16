package jira

import "fmt"

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
		c.BaseURL, jql)
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

// CreateIssue creates a new Jira issue and returns its key.
func (c *Client) CreateIssue(projectKey, issueTypeID, summary, description, epicKey string) (CreateIssueResult, error) {
	u := fmt.Sprintf("%s/rest/api/3/issue", c.BaseURL)
	fields := map[string]any{
		"project":   map[string]string{"key": projectKey},
		"issuetype": map[string]string{"id": issueTypeID},
		"summary":   summary,
	}
	if description != "" {
		fields["description"] = map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []map[string]any{
				{
					"type": "paragraph",
					"content": []map[string]any{
						{"type": "text", "text": description},
					},
				},
			},
		}
	}
	if epicKey != "" {
		fields["epic"] = map[string]string{"key": epicKey}
	}
	body := map[string]any{"fields": fields}
	var result CreateIssueResult
	if err := c.postJSONResponse(u, body, &result); err != nil {
		return CreateIssueResult{}, fmt.Errorf("create issue: %w", err)
	}
	return result, nil
}