package jira

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIssues_Text(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		is := assert.New(t)
		jql := r.URL.Query().Get("jql")
		is.Contains(jql, "summary", "text search should query summary")
		is.Contains(jql, "description", "text search should query description")
		is.Contains(jql, "login bug", "text search should contain the query")
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				newTestIssue("PROJ-1", "Fix login bug", "In Progress", ""),
			},
		})
	})

	c := fake.client()
	results, err := c.SearchIssues("login bug")
	must := require.New(t)
	is := assert.New(t)
	must.NoError(err)
	is.Len(results, 1)
	is.Equal("PROJ-1", results[0].Key)
	is.Equal("Fix login bug", results[0].Summary)
	is.Equal("In Progress", results[0].Status)
}

func TestSearchIssues_JQL(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		is := assert.New(t)
		jql := r.URL.Query().Get("jql")
		is.Equal("project = PROJ AND status = Done", jql, "JQL query should be passed as-is")
		jsonResponse(w, searchJqlResponse{Issues: []issue{}})
	})

	c := fake.client()
	results, err := c.SearchIssues("project = PROJ AND status = Done")
	must := require.New(t)
	is := assert.New(t)
	must.NoError(err)
	is.Empty(results)
}

func TestSearchIssues_EmptyQuery(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	c := fake.client()
	results, err := c.SearchIssues("")
	must := require.New(t)
	is := assert.New(t)
	must.NoError(err)
	is.Nil(results)
}

func TestSearchIssues_WithAssignee(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("/rest/api/3/search/jql", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, searchJqlResponse{
			Issues: []issue{
				newTestIssue("PROJ-2", "Task", "To Do", "Jane Doe"),
			},
		})
	})

	c := fake.client()
	results, err := c.SearchIssues("task")
	must := require.New(t)
	is := assert.New(t)
	must.NoError(err)
	is.Len(results, 1)
	is.Equal("Jane Doe", results[0].Assignee)
}

func TestBuildSearchJQL_TextQuery(t *testing.T) {
	jql := buildSearchJQL("login bug")
	is := assert.New(t)
	is.Contains(jql, `summary ~ "login bug"`)
	is.Contains(jql, `description ~ "login bug"`)
	is.Contains(jql, "ORDER BY updated DESC")
}

func TestBuildSearchJQL_JQLQuery(t *testing.T) {
	is := assert.New(t)
	jql := buildSearchJQL("project = PROJ AND status = Done")
	is.Equal("project = PROJ AND status = Done", jql, "JQL should be passed through as-is")
}

func TestBuildSearchJQL_EscapesQuotes(t *testing.T) {
	jql := buildSearchJQL(`test "quote"`)
	is := assert.New(t)
	is.Contains(jql, `\"quote\"`, "double quotes should be escaped")
}

func TestBuildSearchJQL_EscapesBackslash(t *testing.T) {
	jql := buildSearchJQL(`test\path`)
	is := assert.New(t)
	is.Contains(jql, `test\\path`, "backslashes should be escaped")
}

func TestLooksLikeJQL_DetectsJQL(t *testing.T) {
	is := assert.New(t)
	is.True(looksLikeJQL("project = PROJ"))
	is.True(looksLikeJQL("status = Done AND assignee = me"))
	is.True(looksLikeJQL("summary ~ \"bug\" OR description ~ \"bug\""))
	is.True(looksLikeJQL("project = X ORDER BY key DESC"))
}

func TestLooksLikeJQL_RejectsPlainText(t *testing.T) {
	is := assert.New(t)
	is.False(looksLikeJQL("login bug"))
	is.False(looksLikeJQL("fix authentication"))
	is.False(looksLikeJQL(""))
}

func newTestIssue(key, summary, status, assignee string) issue {
	iss := issue{Key: key}
	iss.Fields.Summary = summary
	iss.Fields.Status.Name = status
	if assignee != "" {
		iss.Fields.Assignee = &struct {
			DisplayName string `json:"displayName"`
		}{DisplayName: assignee}
	}
	return iss
}
