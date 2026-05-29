package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
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
	transitions, err := c.GetTransitions("P-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 3 {
		t.Fatalf("expected 3 transitions, got %d", len(transitions))
	}

	// "Start Progress" → "In Progress" should produce "Start Progress → In Progress"
	if transitions[0].Name != "Start Progress → In Progress" {
		t.Fatalf("expected combined name, got %q", transitions[0].Name)
	}
	if transitions[0].ToStatusID != "3" {
		t.Fatalf("expected ToStatusID '3', got %q", transitions[0].ToStatusID)
	}
	if transitions[0].ToStatus != "In Progress" {
		t.Fatalf("expected ToStatus 'In Progress', got %q", transitions[0].ToStatus)
	}

	// "Done" → "Done" should NOT produce "Done → Done"
	if transitions[1].Name != "Done" {
		t.Fatalf("same name and to.name should not be combined, got %q", transitions[1].Name)
	}

	// "Reopen" → "Reopen" (different to.name)
	if transitions[2].Name != "Reopen" {
		t.Fatalf("same name and to.name should use name only, got %q", transitions[2].Name)
	}
}

func TestGetTransitions_Empty(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("GET /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, map[string]any{"transitions": []any{}})
	})

	c := fake.client()
	transitions, err := c.GetTransitions("P-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 0 {
		t.Fatalf("expected 0 transitions, got %d", len(transitions))
	}
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
	if err == nil {
		t.Fatal("expected error on 403")
	}
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
	err := c.DoTransition("P-1", "42")
	if err != nil {
		t.Fatal(err)
	}

	tr, ok := receivedBody["transition"].(map[string]any)
	if !ok {
		t.Fatal("request body should contain transition object")
	}
	if tr["id"] != "42" {
		t.Fatalf("expected transition id '42', got %v", tr["id"])
	}
}

func TestDoTransition_Error(t *testing.T) {
	fake := newFakeJira()
	defer fake.close()

	fake.handle("POST /rest/api/3/issue/P-1/transitions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid transition"))
	})

	c := fake.client()
	err := c.DoTransition("P-1", "99")
	if err == nil {
		t.Fatal("expected error on 400")
	}
}
