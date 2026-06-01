package jira

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the Jira REST API using basic auth.
type Client struct {
	BaseURL    string
	Email      string
	Token      string
	HTTPClient *http.Client
}

// NewClient returns a Client configured for the given base URL and credentials.
func NewClient(baseURL, email, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Email:      email,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// postJSONResponse performs an authenticated POST with a JSON body and
// JSON-decodes the response into out.
func (c *Client) postJSONResponse(rawURL string, body any, out any) error {
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
	return json.NewDecoder(resp.Body).Decode(out)
}
// Ping verifies connectivity and credentials by fetching the board configuration.
func (c *Client) Ping(boardID int) error {
	u := fmt.Sprintf("%s/rest/agile/1.0/board/%d/configuration", c.BaseURL, boardID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.Token))
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
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) getJSON(rawURL string, out any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.Token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
