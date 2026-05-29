package jira

// boardConfigResponse is the JSON shape returned by GET /rest/agile/1.0/board/{id}/configuration.
type boardConfigResponse struct {
	Name         string `json:"name"`
	ColumnConfig struct {
		Columns []struct {
			Name     string `json:"name"`
			Statuses []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"statuses"`
		} `json:"columns"`
	} `json:"columnConfig"`
}

// boardIssuesResponse is the JSON shape returned by GET /rest/agile/1.0/board/{id}/issue.
type boardIssuesResponse struct {
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []issue `json:"issues"`
}

// issue is a single Jira issue as returned by the board issues endpoint.
type issue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Labels []string `json:"labels"`
	} `json:"fields"`
}

// Card is the application-level representation of an issue on the board.
type Card struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
	Labels   []string
}

// Column groups cards under a named board column.
type Column struct {
	Name   string
	Issues []Card
}

// Board holds the complete board state ready for display.
type Board struct {
	Name    string
	Columns []Column
}
