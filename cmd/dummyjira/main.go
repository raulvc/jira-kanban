package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	mux := http.NewServeMux()

	// GET /rest/agile/1.0/board/{id}/configuration
	mux.HandleFunc("/rest/agile/1.0/board/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/configuration") {
			_ = json.NewEncoder(w).Encode(boardConfig())
			return
		}

		// /rest/agile/1.0/board/{id}/issue
		_ = json.NewEncoder(w).Encode(boardIssues())
	})

	// POST /rest/api/3/search/jql
	mux.HandleFunc("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(searchResults())
	})

	// GET /rest/api/3/issue/{key}/transitions
	mux.HandleFunc("/rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/transitions") {
			_ = json.NewEncoder(w).Encode(transitions())
			return
		}
		// POST transition — just accept
		w.WriteHeader(http.StatusNoContent)
	})

	addr := ":9191"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	fmt.Fprintf(os.Stderr, "dummy jira server on %s\n", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func boardConfig() any {
	return map[string]any{
		"name": "Engineering",
		"columnConfig": map[string]any{
			"columns": []map[string]any{
				{
					"name": "Backlog",
					"statuses": []map[string]string{
						{"id": "1", "name": "Backlog"},
					},
				},
				{
					"name": "To Do",
					"statuses": []map[string]string{
						{"id": "2", "name": "To Do"},
					},
				},
				{
					"name": "In Progress",
					"statuses": []map[string]string{
						{"id": "3", "name": "In Progress"},
					},
				},
				{
					"name": "In Review",
					"statuses": []map[string]string{
						{"id": "5", "name": "In Review"},
					},
				},
				{
					"name": "Done",
					"statuses": []map[string]string{
						{"id": "4", "name": "Done"},
					},
				},
			},
		},
	}
}

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

func makeIssue(key, summary, statusID, statusName, assignee string, labels ...string) issue {
	var iss issue
	iss.Key = key
	iss.Fields.Summary = summary
	iss.Fields.Status.ID = statusID
	iss.Fields.Status.Name = statusName
	if assignee != "" {
		iss.Fields.Assignee = &struct {
			DisplayName string `json:"displayName"`
		}{DisplayName: assignee}
	}
	iss.Fields.Labels = labels
	return iss
}

func allIssues() []issue {
	return []issue{
		makeIssue("ENG-101", "Implement user auth flow", "2", "To Do", "Alice", "security"),
		makeIssue("ENG-102", "Add pagination to list API", "2", "To Do", "", "backend"),
		makeIssue("ENG-103", "Fix mobile nav overlap", "2", "To Do", "Bob", "bug", "frontend"),
		makeIssue("ENG-104", "Write migration script for DB", "2", "To Do", "Carol"),
		makeIssue("ENG-201", "Refactor payment service", "3", "In Progress", "Alice", "tech-debt"),
		makeIssue("ENG-202", "Update CI pipeline", "3", "In Progress", "Dave", "devops"),
		makeIssue("ENG-203", "Add rate limiting middleware", "3", "In Progress", "Bob", "security", "backend"),
		makeIssue("ENG-204", "Implement webhook retry logic", "3", "In Progress", ""),
		makeIssue("ENG-301", "Review SSO integration PR", "5", "In Review", "Carol", "security"),
		makeIssue("ENG-302", "Review logging changes", "5", "In Review", "Dave"),
		makeIssue("ENG-401", "Deploy v2.1 to staging", "4", "Done", "Alice", "release"),
		makeIssue("ENG-402", "Set up monitoring dashboards", "4", "Done", "Dave", "devops"),
		makeIssue("ENG-403", "Migrate to new CDN", "4", "Done", "Carol", "infra"),
	}
}

func boardIssues() any {
	time.Sleep(300 * time.Millisecond) // simulate latency for visible progress bar
	return map[string]any{
		"startAt":    0,
		"maxResults": 50,
		"total":      len(allIssues()),
		"issues":     allIssues(),
	}
}

func searchResults() any {
	return map[string]any{
		"issues": allIssues(),
	}
}

func transitions() any {
	return map[string]any{
		"transitions": []map[string]any{
			{"id": "21", "name": "To Do", "to": map[string]string{"id": "2", "name": "To Do"}},
			{"id": "31", "name": "In Progress", "to": map[string]string{"id": "3", "name": "In Progress"}},
			{"id": "51", "name": "In Review", "to": map[string]string{"id": "5", "name": "In Review"}},
			{"id": "41", "name": "Done", "to": map[string]string{"id": "4", "name": "Done"}},
		},
	}
}