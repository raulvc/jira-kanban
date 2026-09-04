package jira

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Sentinel errors for sprint support, so callers can distinguish a board
// that can't have sprints (e.g. a Kanban board) from transient failures.
var (
	// ErrSprintsUnsupported is returned when the board type does not
	// support sprints at all. Jira answers the sprint endpoint with a
	// 400 in this case.
	ErrSprintsUnsupported = errors.New("sprints not supported on this board")
	// ErrNoActiveSprint is returned for boards that support sprints but
	// currently have none active.
	ErrNoActiveSprint = errors.New("no active sprint")
)

// Sprint is a Jira sprint on a board.
type Sprint struct {
	ID    int
	Name  string
	State string
}

// sprintResponse is the JSON shape returned by GET /rest/agile/1.0/board/{id}/sprint.
type sprintResponse struct {
	Values []struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"values"`
}

// FetchActiveSprint returns the currently active sprint on the board.
// Returns ErrSprintsUnsupported if the board type does not support sprints
// and ErrNoActiveSprint if the board supports sprints but none is active.
func (c *Client) FetchActiveSprint(boardID int) (Sprint, error) {
	u := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint?state=active", c.BaseURL, boardID)
	var resp sprintResponse
	if err := c.getJSON(u, &resp); err != nil {
		// Kanban boards (and other sprint-less board types) reject the
		// sprint endpoint with HTTP 400.
		if strings.HasPrefix(err.Error(), "HTTP 400") {
			return Sprint{}, fmt.Errorf("active sprint: %w", ErrSprintsUnsupported)
		}
		return Sprint{}, fmt.Errorf("active sprint: %w", err)
	}
	if len(resp.Values) == 0 {
		return Sprint{}, fmt.Errorf("active sprint: %w", ErrNoActiveSprint)
	}
	s := resp.Values[0]
	return Sprint{ID: s.ID, Name: s.Name, State: s.State}, nil
}

// FetchSprintIssueKeys returns the keys of all issues in the given sprint.
func (c *Client) FetchSprintIssueKeys(boardID, sprintID int) ([]string, error) {
	var all []string
	for startAt := 0; ; startAt += 50 {
		u := fmt.Sprintf(
			"%s/rest/agile/1.0/board/%d/sprint/%d/issue?startAt=%d&maxResults=50&fields=status",
			c.BaseURL, boardID, sprintID, startAt)
		var resp boardIssuesResponse
		if err := c.getJSON(u+"&jql="+url.QueryEscape("ORDER BY rank ASC"), &resp); err != nil {
			return nil, fmt.Errorf("sprint issues (offset %d): %w", startAt, err)
		}
		for _, iss := range resp.Issues {
			all = append(all, iss.Key)
		}
		if len(all) >= resp.Total || len(resp.Issues) == 0 {
			break
		}
	}
	return all, nil
}
