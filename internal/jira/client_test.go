package jira

import (
	"net/http"
	"testing"
)

func TestGetJSON_Success(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /test", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]string{"key": "value"})
	})

	c := fake.client()
	var out map[string]string
	err := c.getJSON(fake.server.URL+"/test", &out)
	if err != nil {
		t.Fatal(err)
	}
	if out["key"] != "value" {
		t.Fatalf("expected 'value', got %q", out["key"])
	}
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
	if err == nil {
		t.Fatal("expected error on 500")
	}
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
	if err == nil {
		t.Fatal("expected error on 400")
	}
}

func TestPostJSONResponse_Success(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /search", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]int{"total": 42})
	})

	c := fake.client()
	var out map[string]int
	err := c.postJSONResponse(fake.server.URL+"/search", map[string]string{"q": "x"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out["total"] != 42 {
		t.Fatalf("expected 42, got %d", out["total"])
	}
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
	if err == nil {
		t.Fatal("expected error on 403")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.atlassian.net/", "a@b.com", "tok")
	if c.BaseURL != "https://example.atlassian.net" {
		t.Fatalf("trailing slash should be trimmed, got %q", c.BaseURL)
	}
	if c.Email != "a@b.com" || c.Token != "tok" {
		t.Fatal("credentials not set")
	}
	if c.HTTPClient == nil {
		t.Fatal("HTTPClient should be set")
	}
}
