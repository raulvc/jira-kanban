package jira

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// HistoryItem represents a recently updated issue for the history view.
type HistoryItem struct {
	Key        string
	Summary    string
	Status     string
	Assignee  string
	Epic      string
	Updated   time.Time
	ChangeDesc string
}

// SearchRecentActivity fetches recently updated issues for the given user.
// It searches for issues where the user is the assignee or reporter, updated
// within the last days, ordered by most recently updated first.
func (c *Client) SearchRecentActivity(accountID string, days int) ([]HistoryItem, error) {
	jql := fmt.Sprintf(
		"(assignee = %s OR reporter = %s) AND updated >= -%dd ORDER BY updated DESC",
		accountID, accountID, days,
	)
	u := fmt.Sprintf("%s/rest/api/3/search/jql?jql=%s&maxResults=50&fields=summary,status,assignee,epic,updated&expand=changelog",
		c.BaseURL, url.QueryEscape(jql))

	type clHistory struct {
		Items []changelogItem `json:"items"`
	}

	var resp struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
				Assignee *struct {
					DisplayName string `json:"displayName"`
				} `json:"assignee"`
				Epic *issueEpic `json:"epic"`
				Updated string `json:"updated"`
			} `json:"fields"`
			Changelog struct {
				Histories []clHistory `json:"histories"`
			} `json:"changelog"`
		} `json:"issues"`
	}
	if err := c.getJSON(u, &resp); err != nil {
		return nil, fmt.Errorf("search recent activity: %w", err)
	}

	out := make([]HistoryItem, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		item := HistoryItem{
			Key:     iss.Key,
			Summary: iss.Fields.Summary,
			Status:  iss.Fields.Status.Name,
		}
		if iss.Fields.Assignee != nil {
			item.Assignee = iss.Fields.Assignee.DisplayName
		}
		if iss.Fields.Epic != nil {
			item.Epic = iss.Fields.Epic.Summary
		}
		if len(iss.Changelog.Histories) > 0 {
			last := iss.Changelog.Histories[len(iss.Changelog.Histories)-1]
			item.ChangeDesc = describeChange(last.Items)
		}
		if t, err := time.Parse("2006-01-02T15:04:05.000-0700", iss.Fields.Updated); err == nil {
			item.Updated = t
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b HistoryItem) int {
		if a.Updated.After(b.Updated) {
			return -1
		}
		if a.Updated.Before(b.Updated) {
			return 1
		}
		return 0
	})
	return out, nil
}

type changelogItem struct {
	Field      string `json:"field"`
	FromString string `json:"fromString"`
	ToString   string `json:"toString"`
}

func describeChange(items []changelogItem) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		it := items[0]
		switch {
		case it.FromString == "" && it.ToString != "":
			return fmt.Sprintf("%s set to %s", it.Field, it.ToString)
		case it.FromString != "" && it.ToString == "":
			return fmt.Sprintf("%s removed", it.Field)
		case it.FromString != "" && it.ToString != "":
			return fmt.Sprintf("%s: %s → %s", it.Field, it.FromString, it.ToString)
		default:
			return fmt.Sprintf("%s changed", it.Field)
		}
	}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Field
	}
	return fmt.Sprintf("%d changes: %s", len(items), strings.Join(names, ", "))
}