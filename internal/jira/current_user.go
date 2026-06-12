package jira

import "fmt"

// CurrentUser represents the currently logged-in Jira user.
type CurrentUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email        string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

// GetCurrentUser fetches the currently authenticated user's profile.
func (c *Client) GetCurrentUser() (CurrentUser, error) {
	u := fmt.Sprintf("%s/rest/api/3/myself", c.BaseURL)
	var user CurrentUser
	if err := c.getJSON(u, &user); err != nil {
		return CurrentUser{}, fmt.Errorf("get current user: %w", err)
	}
	return user, nil
}