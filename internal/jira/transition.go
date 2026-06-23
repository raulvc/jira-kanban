package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Transition represents one available status transition for an issue.
type Transition struct {
	ID         string
	Name       string
	ToStatusID string
	ToStatus   string
}

// transitionsResponse is the JSON shape returned by GET /rest/api/3/issue/{key}/transitions.
type transitionsResponse struct {
	Transitions []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		To   struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"to"`
	} `json:"transitions"`
}

// GetTransitions returns the available transitions for the given issue key.
func (c *Client) GetTransitions(issueKey string) ([]Transition, error) {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.BaseURL, issueKey)
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
		out[i] = Transition{ID: t.ID, Name: name, ToStatusID: t.To.ID, ToStatus: t.To.Name}
	}
	return out, nil
}

// DoTransition executes a status transition on the given issue.
func (c *Client) DoTransition(issueKey, transitionID string) error {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.BaseURL, issueKey)
	body := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	return c.postJSON(u, body)
}

// postJSON performs an authenticated POST with a JSON body.
func (c *Client) postJSON(rawURL string, body any) error {
	return c.sendJSON(http.MethodPost, rawURL, body)
}

// putJSON performs an authenticated PUT with a JSON body.
func (c *Client) putJSON(rawURL string, body any) error {
	return c.sendJSON(http.MethodPut, rawURL, body)
}

// sendJSON performs an authenticated HTTP request with a JSON body.
func (c *Client) sendJSON(method, rawURL string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
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
