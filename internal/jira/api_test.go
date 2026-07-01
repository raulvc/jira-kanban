package jira

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIssueTypes(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/project/PROJ", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]any{
			"issueTypes": []map[string]any{
				{"id": "1", "name": "Task", "subtask": false},
				{"id": "2", "name": "Bug", "subtask": false},
				{"id": "3", "name": "Sub-task", "subtask": true},
			},
		})
	})

	c := fake.client()
	types, err := c.GetIssueTypes("PROJ")
	must := require.New(t)
	is := assert.New(t)
	must.NoError(err)
	is.Len(types, 2)
	for _, it := range types {
		is.False(it.Subtask, "subtask type should be filtered")
	}
}

func TestGetIssueTypes_APIError(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/project/PROJ", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	})

	c := fake.client()
	_, err := c.GetIssueTypes("PROJ")
	assert.Error(t, err, "expected error for 404")
}

func TestGetSubtaskTypes(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/project/PROJ", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]any{
			"issueTypes": []map[string]any{
				{"id": "1", "name": "Task", "subtask": false},
				{"id": "2", "name": "Sub-task", "subtask": true},
			},
		})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	types, err := c.GetSubtaskTypes("PROJ")
	must.NoError(err)
	is.Len(types, 1)
	is.True(types[0].Subtask)
}

func TestSearchEpics(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				{Key: "PROJ-1"},
			},
		})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	epics, err := c.SearchEpics("PROJ", "test")
	must.NoError(err)
	is.Len(epics, 1)
}

func TestCreateIssue(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		var body map[string]any
		must.NoError(json.NewDecoder(r.Body).Decode(&body))
		fields := body["fields"].(map[string]any)
		assert.Equal(t, "New issue", fields["summary"])
		jsonResponse(w, CreateIssueResult{Key: "PROJ-100", ID: "10001"})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	result, err := c.CreateIssue("PROJ", "1", "New issue", "description text", nil)
	must.NoError(err)
	is.Equal("PROJ-100", result.Key)
}

func TestCreateIssue_NoDescription(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		is := assert.New(t)
		var body map[string]any
		must.NoError(json.NewDecoder(r.Body).Decode(&body))
		fields := body["fields"].(map[string]any)
		_, ok := fields["description"]
		is.False(ok, "empty description should not be included in fields")
		jsonResponse(w, CreateIssueResult{Key: "PROJ-101"})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	result, err := c.CreateIssue("PROJ", "1", "Title only", "", nil)
	must.NoError(err)
	is.Equal("PROJ-101", result.Key)
}

func TestCreateSubtask(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		is := assert.New(t)
		var body map[string]any
		must.NoError(json.NewDecoder(r.Body).Decode(&body))
		fields := body["fields"].(map[string]any)
		_, ok := fields["parent"]
		is.True(ok, "subtask should have parent field")
		jsonResponse(w, CreateIssueResult{Key: "PROJ-200"})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	result, err := c.CreateSubtask("PROJ", "2", "Sub task", "desc", "PROJ-1", nil)
	must.NoError(err)
	is.Equal("PROJ-200", result.Key)
}

func TestLinkEpic(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]any
	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		must.NoError(json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	must.NoError(c.LinkEpic("PROJ-1", "EPIC-1"))
	fields := receivedBody["fields"].(map[string]any)
	parent := fields["parent"].(map[string]any)
	is.Equal("EPIC-1", parent["key"])
}

func TestLinkEpic_Error(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["bad request"]}`))
	})

	c := fake.client()
	assert.Error(t, c.LinkEpic("PROJ-1", "EPIC-1"), "expected error for 400")
}

func TestClearEpic(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]any
	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		must.NoError(json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	must.NoError(c.ClearEpic("PROJ-1"))
	fields := receivedBody["fields"].(map[string]any)
	is.Nil(fields["parent"])
}

func TestGetCurrentUser(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/myself", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, CurrentUser{
			AccountID:   "abc123",
			DisplayName: "Test User",
			Email:       "test@example.com",
			Active:      true,
		})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	user, err := c.GetCurrentUser()
	must.NoError(err)
	is.Equal("abc123", user.AccountID)
	is.True(user.Active)
}

func TestGetCurrentUser_Error(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/myself", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	})

	c := fake.client()
	_, err := c.GetCurrentUser()
	assert.Error(t, err, "expected error for 401")
}

func TestSearchAssignableUsers(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/user/assignable/search", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, []AssignableUser{
			{AccountID: "1", DisplayName: "Alice", Active: true},
			{AccountID: "2", DisplayName: "Bob", Active: false},
		})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	users, err := c.SearchAssignableUsers("PROJ-1", "")
	must.NoError(err)
	is.Len(users, 1)
	is.Equal("Alice", users[0].DisplayName)
}

func TestAssignIssue(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]string
	fake.handle("PUT /rest/api/3/issue/P-1/assignee", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		must.NoError(json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	must.NoError(c.AssignIssue("P-1", "user123"))
	is.Equal("user123", receivedBody["accountId"])
}

func TestPing_Success(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, boardConfigJSON())
	})

	c := fake.client()
	assert.NoError(t, c.Ping(1))
}

func TestPing_Unauthorized(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	c := fake.client()
	assert.Error(t, c.Ping(1), "expected error for 401")
}

func TestPing_NotFound(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/agile/1.0/board/1/configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	c := fake.client()
	assert.Error(t, c.Ping(1), "expected error for 404")
}

func TestSearchRecentActivity(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]any{
			"issues": []map[string]any{
				{
					"key": "P-1",
					"fields": map[string]any{
						"summary": "Test",
						"status":   map[string]string{"name": "To Do"},
						"updated": "2024-01-15T10:30:00.000+0000",
					},
					"changelog": map[string]any{
						"histories": []map[string]any{
							{"items": []map[string]any{
								{"field": "status", "fromString": "To Do", "toString": "In Progress"},
							}},
						},
					},
				},
			},
		})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	items, err := c.SearchRecentActivity("user123", 7)
	must.NoError(err)
	is.Len(items, 1)
	is.Equal("status: To Do → In Progress", items[0].ChangeDesc)
}

func TestEditIssue_SummaryOnly(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]any
	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		must.NoError(json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)

	summary := "New Summary"
	must.NoError(c.EditIssue("PROJ-1", IssueEdit{Summary: &summary}))

	fields := receivedBody["fields"].(map[string]any)
	is.Equal("New Summary", fields["summary"])
	_, hasDesc := fields["description"]
	is.False(hasDesc, "description should not be set when nil")
	_, hasLabels := fields["labels"]
	is.False(hasLabels, "labels should not be set when nil")
}

func TestEditIssue_MultipleFields(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]any
	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		must.NoError(json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)

	summary := "Updated"
	desc := "New description"
	labels := []string{"bug", "urgent"}
	must.NoError(c.EditIssue("PROJ-1", IssueEdit{Summary: &summary, Description: &desc, Labels: &labels}))

	fields := receivedBody["fields"].(map[string]any)
	is.Equal("Updated", fields["summary"])
	is.NotNil(fields["description"])
	is.Equal([]any{"bug", "urgent"}, fields["labels"])
}

func TestEditIssue_NoFieldsIsNoop(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	called := false
	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, _ *http.Request) {
		called = true
	})

	c := fake.client()
	must := require.New(t)

	must.NoError(c.EditIssue("PROJ-1", IssueEdit{}))
	assert.False(t, called, "no fields should not make API call")
}

func TestClearEpic_SendsNullParent(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]any
	fake.handle("PUT /rest/api/3/issue/PROJ-5", func(w http.ResponseWriter, r *http.Request) {
		must := require.New(t)
		must.NoError(json.NewDecoder(r.Body).Decode(&receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)

	must.NoError(c.ClearEpic("PROJ-5"))
	fields := receivedBody["fields"].(map[string]any)
	is.Nil(fields["parent"], "parent should be nil in fields")
}

func TestClearEpic_Error(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("PUT /rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["bad request"]}`))
	})

	c := fake.client()
	assert.Error(t, c.ClearEpic("PROJ-1"), "expected error for 400")
}