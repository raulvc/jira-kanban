package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTransitions(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]any{
			"transitions": []map[string]any{
				{"id": "11", "name": "Start Progress", "to": map[string]string{"id": "3", "name": "In Progress"}},
				{"id": "21", "name": "Done", "to": map[string]string{"id": "4", "name": "Done"}},
				{"id": "31", "name": "Reopen", "to": map[string]string{"id": "2", "name": "Reopen"}},
			},
		})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	transitions, err := c.GetTransitions("P-1")
	must.NoError(err)
	is.Len(transitions, 3)

	is.Equal("Start Progress → In Progress", transitions[0].Name)
	is.Equal("3", transitions[0].ToStatusID)
	is.Equal("In Progress", transitions[0].ToStatus)

	is.Equal("Done", transitions[1].Name, "same name and to.name should not be combined")

	is.Equal("Reopen", transitions[2].Name)
}

func TestGetTransitions_Empty(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]any{"transitions": []any{}})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	transitions, err := c.GetTransitions("P-1")
	must.NoError(err)
	is.Empty(transitions)
}

func TestGetTransitions_APIError(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("no access"))
	})

	c := fake.client()
	_, err := c.GetTransitions("P-1")
	assert.Error(t, err, "expected error on 403")
}

func TestDoTransition(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	var receivedBody map[string]any
	fake.handle("POST /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusNoContent)
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	must.NoError(c.DoTransition("P-1", "42"))

	tr, ok := receivedBody["transition"].(map[string]any)
	must.True(ok, "request body should contain transition object")
	is.Equal("42", tr["id"])
}

func TestDoTransition_Error(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid transition"))
	})

	c := fake.client()
	assert.Error(t, c.DoTransition("P-1", "99"), "expected error on 400")
}