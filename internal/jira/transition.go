package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Transition represents one available status transition for an issue.
type Transition struct {
	ID   string
	Name string
}

// transitionsResponse is the JSON shape returned by GET /rest/api/2/issue/{key}/transitions.
type transitionsResponse struct {
	Transitions []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		To   struct {
			Name string `json:"name"`
		} `json:"to"`
	} `json:"transitions"`
}

// GetTransitions returns the available transitions for the given issue key.
func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	u := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.BaseURL, issueKey)
	var resp transitionsResponse
	if err := c.getJSON(u, &resp); err != nil {
		return nil, fmt.Errorf("get transitions for %s: %w", issueKey, err)
	}
	out := make([]Transition, len(resp.Transitions))
	for i, t := range resp.Transitions {
		name := t.Name
		if t.To.Name != "" && t.To.Name != t.Name {
			name = t.Name + " → " + t.To.Name
		}
		out[i] = Transition{ID: t.ID, Name: name}
	}
	return out, nil
}

// DoTransition executes a status transition on the given issue.
func (c *Client) DoTransition(issueKey, transitionID string) error {
	u := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.BaseURL, issueKey)
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	return c.postJSON(u, body)
}

// postJSON performs an authenticated POST with a JSON body.
func (c *Client) postJSON(rawURL string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.Token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}
