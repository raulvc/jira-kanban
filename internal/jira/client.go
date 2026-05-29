package jira

import (
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

// getJSON performs an authenticated GET and JSON-decodes the response into out.
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
