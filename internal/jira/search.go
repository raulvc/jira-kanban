package jira

import (
	"fmt"
	"net/url"
)

// SearchResult is a single issue returned by SearchIssues.
type SearchResult struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
}

// SearchIssues searches for issues using a JQL query.
// The query is used as-is if it looks like JQL (contains =, ~, IN, etc.),
// otherwise it searches across summary and description.
func (c *Client) SearchIssues(query string) ([]SearchResult, error) {
	if query == "" {
		return nil, nil
	}

	jql := buildSearchJQL(query)
	u := fmt.Sprintf("%s/rest/api/3/search/jql?jql=%s&maxResults=50&fields=summary,status,assignee",
		c.BaseURL, url.QueryEscape(jql))

	var resp searchJqlResponse
	if err := c.getJSON(u, &resp); err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}

	out := make([]SearchResult, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		r := SearchResult{
			Key:     iss.Key,
			Summary: iss.Fields.Summary,
			Status:  iss.Fields.Status.Name,
		}
		if iss.Fields.Assignee != nil {
			r.Assignee = iss.Fields.Assignee.DisplayName
		}
		out = append(out, r)
	}
	return out, nil
}

// buildSearchJQL constructs a JQL query from the user's search text.
// If the text looks like JQL (contains operators), it's used as-is.
// Otherwise, it searches across summary and description text.
func buildSearchJQL(query string) string {
	if looksLikeJQL(query) {
		return query
	}
	escaped := escapeJQLString(query)
	return fmt.Sprintf(`summary ~ "%[1]s" OR description ~ "%[1]s" ORDER BY updated DESC`, escaped)
}

func looksLikeJQL(s string) bool {
	upper := toUpperASCII(s)
	keywords := []string{" = ", " != ", " ~ ", " !~ ", " IN ", " NOT ", " AND ", " OR ", " ORDER BY ", " project ", " status ", " assignee "}
	for _, kw := range keywords {
		if contains(upper, kw) {
			return true
		}
	}
	return false
}

func escapeJQLString(s string) string {
	var b []rune
	for _, r := range s {
		if r == '"' || r == '\\' {
			b = append(b, '\\')
		}
		b = append(b, r)
	}
	return string(b)
}

func toUpperASCII(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'a' && r <= 'z' {
			runes[i] = r - 32
		}
	}
	return string(runes)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
