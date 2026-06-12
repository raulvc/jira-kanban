package jira

import "fmt"

// AssignableUser represents a Jira user who can be assigned to an issue.
type AssignableUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

// SearchAssignableUsers returns users who can be assigned to the given issue.
func (c *Client) SearchAssignableUsers(issueKey, query string) ([]AssignableUser, error) {
	u := fmt.Sprintf("%s/rest/api/3/user/assignable/search?issueKey=%s&maxResults=50", c.BaseURL, issueKey)
	if query != "" {
		u += "&query=" + query
	}
	var users []AssignableUser
	if err := c.getJSON(u, &users); err != nil {
		return nil, fmt.Errorf("search assignable users for %s: %w", issueKey, err)
	}
	var active []AssignableUser
	for _, u := range users {
		if u.Active {
			active = append(active, u)
		}
	}
	return active, nil
}

// AssignIssue sets the assignee of the given issue to the user with the given accountID.
// An empty accountID unassigns the issue.
func (c *Client) AssignIssue(issueKey, accountID string) error {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s/assignee", c.BaseURL, issueKey)
	body := map[string]string{"accountId": accountID}
	return c.putJSON(u, body)
}