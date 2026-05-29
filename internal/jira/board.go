package jira

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// FetchBoard retrieves the board configuration and its issues, then maps
// issues into columns. The Backlog column is excluded.
func (c *Client) FetchBoard(boardID int) (Board, error) {
	columns, boardName, err := c.fetchColumns(boardID)
	if err != nil {
		return Board{}, err
	}
	issues, err := c.fetchIssues(boardID)
	if err != nil {
		return Board{}, err
	}
	board := c.buildBoard(boardName, columns, issues)
	return board, nil
}

// fetchColumns returns the column layout from the board configuration endpoint.
func (c *Client) fetchColumns(boardID int) ([]columnMapping, string, error) {
	cfgURL := fmt.Sprintf("%s/rest/agile/1.0/board/%d/configuration", c.BaseURL, boardID)
	var resp boardConfigResponse
	if err := c.getJSON(cfgURL, &resp); err != nil {
		return nil, "", fmt.Errorf("board config: %w", err)
	}
	var cols []columnMapping
	for _, col := range resp.ColumnConfig.Columns {
		statusIDs := make([]string, 0, len(col.Statuses))
		statusNames := make([]string, 0, len(col.Statuses))
		for _, s := range col.Statuses {
			statusIDs = append(statusIDs, s.ID)
			if s.Name != "" {
				statusNames = append(statusNames, s.Name)
			}
		}
		cols = append(cols, columnMapping{
			Name:        col.Name,
			StatusIDs:   statusIDs,
			StatusNames: statusNames,
		})
	}
	return cols, resp.Name, nil
}

// fetchIssues pages through the board issues endpoint.
func (c *Client) fetchIssues(boardID int) ([]issue, error) {
	jql := "updated >= -4w ORDER BY rank ASC"
	var all []issue
	for startAt := 0; ; startAt += 50 {
		u := fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue?startAt=%d&maxResults=50&jql=%s",
			c.BaseURL, boardID, startAt, url.QueryEscape(jql))
		var resp boardIssuesResponse
		if err := c.getJSON(u, &resp); err != nil {
			return nil, fmt.Errorf("board issues (offset %d): %w", startAt, err)
		}
		all = append(all, resp.Issues...)
		if len(all) >= resp.Total || len(resp.Issues) == 0 {
			break
		}
	}
	return all, nil
}

// columnMapping pairs a column name with the status IDs/names that map to it.
type columnMapping struct {
	Name        string
	StatusIDs   []string
	StatusNames []string
}

// buildBoard assigns issues to columns, skips Backlog, and sorts each column.
func (c *Client) buildBoard(boardName string, mappings []columnMapping, issues []issue) Board {
	// Build lookup from status ID and status name to column name.
	statusIDToCol := map[string]string{}
	statusNameToCol := map[string]string{}
	columns := make([]Column, 0, len(mappings))

	issueStatusNames := map[string]string{}
	for _, iss := range issues {
		sid := strings.TrimSpace(iss.Fields.Status.ID)
		sname := strings.TrimSpace(iss.Fields.Status.Name)
		if sid != "" && sname != "" {
			issueStatusNames[sid] = sname
		}
	}

	for _, m := range mappings {
		if strings.EqualFold(m.Name, "Backlog") {
			continue
		}
		columns = append(columns, Column{Name: m.Name})
		for i, sid := range m.StatusIDs {
			statusIDToCol[sid] = m.Name
			name := ""
			if i < len(m.StatusNames) {
				name = m.StatusNames[i]
			}
			if name == "" {
				name = issueStatusNames[sid]
			}
			if name != "" {
				statusNameToCol[strings.ToLower(name)] = m.Name
			}
		}
	}

	colIndex := map[string]int{}
	for i, col := range columns {
		colIndex[col.Name] = i
	}

	for _, iss := range issues {
		card := Card{
			Key:      iss.Key,
			Summary:  iss.Fields.Summary,
			Status:   iss.Fields.Status.Name,
			Assignee: assigneeName(iss),
			Labels:   iss.Fields.Labels,
		}

		colName := resolveColumn(iss, statusIDToCol, statusNameToCol)
		idx, ok := colIndex[colName]
		if !ok {
			columns = append(columns, Column{Name: colName})
			idx = len(columns) - 1
			colIndex[colName] = idx
		}
		columns[idx].Issues = append(columns[idx].Issues, card)
	}

	for i := range columns {
		sort.SliceStable(columns[i].Issues, func(a, b int) bool {
			return columns[i].Issues[a].Key < columns[i].Issues[b].Key
		})
	}

	return Board{Name: boardName, Columns: columns}
}

func resolveColumn(iss issue, byID, byName map[string]string) string {
	if name, ok := byID[strings.TrimSpace(iss.Fields.Status.ID)]; ok {
		return name
	}
	if name, ok := byName[strings.ToLower(strings.TrimSpace(iss.Fields.Status.Name))]; ok {
		return name
	}
	return iss.Fields.Status.Name
}

func assigneeName(iss issue) string {
	if iss.Fields.Assignee != nil && iss.Fields.Assignee.DisplayName != "" {
		return iss.Fields.Assignee.DisplayName
	}
	return "Unassigned"
}
