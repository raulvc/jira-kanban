package jira

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJSON_Success(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /test", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]string{"key": "value"})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	var out map[string]string
	must.NoError(c.getJSON(fake.server.URL+"/test", &out))
	is.Equal("value", out["key"])
}

func TestGetJSON_HTTPError(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /fail", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	})

	c := fake.client()
	var out map[string]string
	err := c.getJSON(fake.server.URL+"/fail", &out)
	assert.Error(t, err, "expected error on 500")
}

func TestPostJSON_HTTPError(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /fail", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	})

	c := fake.client()
	err := c.postJSON(fake.server.URL+"/fail", map[string]string{"a": "b"})
	assert.Error(t, err, "expected error on 400")
}

func TestPostJSONResponse_Success(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /search", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]int{"total": 42})
	})

	c := fake.client()
	must := require.New(t)
	is := assert.New(t)
	var out map[string]int
	must.NoError(c.postJSONResponse(fake.server.URL+"/search", map[string]string{"q": "x"}, &out))
	is.Equal(42, out["total"])
}

func TestPostJSONResponse_HTTPError(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /search", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})

	c := fake.client()
	var out map[string]int
	err := c.postJSONResponse(fake.server.URL+"/search", map[string]string{}, &out)
	assert.Error(t, err, "expected error on 403")
}

func TestNewClient(t *testing.T) {
	is := assert.New(t)
	c := NewClient("https://example.atlassian.net/", "a@b.com", "tok")
	is.Equal("https://example.atlassian.net", c.BaseURL)
	is.Equal("a@b.com", c.Email)
	is.Equal("tok", c.APIToken)
	is.NotNil(c.HTTPClient)
}