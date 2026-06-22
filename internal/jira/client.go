package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Client talks to the Jira REST API using basic auth.
type Client struct {
	BaseURL     string
	Email       string
	APIToken    string
	AccountID   string // set eagerly so sync can query recent activity
	HTTPClient  *http.Client
	RankFieldID int
}

// NewClient returns a Client configured for the given base URL and credentials.
func NewClient(baseURL, email, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Email:      email,
		APIToken:   token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// postJSONResponse performs an authenticated POST with a JSON body and
// JSON-decodes the response into out.
func (c *Client) postJSONResponse(rawURL string, body any, out any) error {
	raw, err := c.postJSONRaw(rawURL, body)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) postJSONRaw(rawURL string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	slog.Debug("HTTP POST", "url", rawURL, "body", string(data))
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.APIToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("HTTP POST failed", "url", rawURL, "error", err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := parseJiraError(resp.StatusCode, respBody)
		slog.Error("HTTP POST error", "url", rawURL, "status", resp.Status, "body", string(respBody))
		return nil, msg
	}
	return respBody, nil
}

// Ping verifies connectivity and credentials by fetching the board configuration.
func (c *Client) Ping(boardID int) error {
	u := fmt.Sprintf("%s/rest/agile/1.0/board/%d/configuration", c.BaseURL, boardID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.APIToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: check email and API token")
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("board %d not found: check board ID", boardID)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		body, _ := io.ReadAll(resp.Body)
		return parseJiraError(resp.StatusCode, body)
	}
	return nil
}

// GetIssue fetches the full issue details (including description) for a key.
func (c *Client) GetIssue(key string) (Card, error) {
	u := fmt.Sprintf("%s/rest/api/3/issue/%s", c.BaseURL, key)
	var resp issue
	if err := c.getJSON(u, &resp); err != nil {
		return Card{}, fmt.Errorf("get issue %s: %w", key, err)
	}
	return Card{
		Key:         resp.Key,
		Summary:     resp.Fields.Summary,
		StatusID:    strings.TrimSpace(resp.Fields.Status.ID),
		Status:      resp.Fields.Status.Name,
		Assignee:    assigneeName(resp),
		Labels:      resp.Fields.Labels,
		Description: parseDescription(resp.Fields.Description),
		RichDesc:   ParseRichDesc(resp.Fields.Description),
		Epic:        epicName(resp),
		Subtasks:    parseSubtasks(resp),
	}, nil
}

func (c *Client) getJSON(rawURL string, out any) error {
	raw, err := c.getRaw(rawURL)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) getRaw(rawURL string) ([]byte, error) {
	slog.Debug("HTTP GET", "url", rawURL)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.APIToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		slog.Error("HTTP GET failed", "url", rawURL, "error", err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := parseJiraError(resp.StatusCode, body)
		slog.Error("HTTP GET error", "url", rawURL, "status", resp.Status, "body", string(body))
		return nil, msg
	}
	return body, nil
}

// parseJiraError produces a human-readable error from a Jira HTTP error response.
// Jira errors are JSON like: {"errorMessages":["..."], "errors":{"field":"msg"}}.
func parseJiraError(statusCode int, body []byte) error {
	var jerr struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors       map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &jerr); err == nil {
		msgs := make([]string, 0, len(jerr.ErrorMessages)+len(jerr.Errors))
		msgs = append(msgs, jerr.ErrorMessages...)
		for f, m := range jerr.Errors {
			msgs = append(msgs, f+": "+m)
		}
		if len(msgs) > 0 {
			return fmt.Errorf("HTTP %d: %s", statusCode, strings.Join(msgs, "; "))
		}
	}
	return fmt.Errorf("HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
}