package jira

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchActiveSprint(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/sprint", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, sprintResponse{
			Values: []struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				State string `json:"state"`
			}{
				{ID: 42, Name: "Sprint 7", State: "active"},
			},
		})
	})

	c := fake.client()
	s, err := c.FetchActiveSprint(1)
	require.NoError(t, err)
	assert.Equal(t, 42, s.ID)
	assert.Equal(t, "Sprint 7", s.Name)
	assert.Equal(t, "active", s.State)
}

func TestFetchActiveSprint_None(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/sprint", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, sprintResponse{})
	})

	c := fake.client()
	_, err := c.FetchActiveSprint(1)
	assert.Error(t, err, "expected error when no active sprint")
}

func TestFetchSprintIssueKeys_Paginated(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	calls := 0
	fake.handle("GET /rest/agile/1.0/board/1/sprint/42/issue", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("startAt") == "0" {
			jsonResponse(w, boardIssuesResponse{
				StartAt: 0, MaxResults: 50, Total: 3,
				Issues: []issue{
					{Key: "P-1"}, {Key: "P-2"},
				},
			})
			return
		}
		jsonResponse(w, boardIssuesResponse{
			StartAt: 2, MaxResults: 50, Total: 3,
			Issues: []issue{{Key: "P-3"}},
		})
	})

	c := fake.client()
	keys, err := c.FetchSprintIssueKeys(1, 42)
	require.NoError(t, err)
	assert.Equal(t, []string{"P-1", "P-2", "P-3"}, keys)
	assert.Equal(t, 2, calls)
}

func TestFetchSprintIssueKeys_APIError(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/sprint/42/issue", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "boom"})
	})

	c := fake.client()
	_, err := c.FetchSprintIssueKeys(1, 42)
	assert.Error(t, err)
}
