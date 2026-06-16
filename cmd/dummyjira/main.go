package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var store struct {
	sync.RWMutex
	issues       map[string]*issueStatus
	assignees    map[string]string // key -> accountID
	nextID       int
	epicList     []epicInfo
	assigneeList []assignableUser
}

type issueStatus struct {
	Key       string
	Summary   string
	StatusID  string
	StatusName string
	Assignee  string // displayName
	Epic      string // epic key
	Labels    []string
}

type epicInfo struct {
	Key      string
	Summary string
	Name     string
}

type assignableUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

func initStore() {
	initial := []issueStatus{
		{"ENG-101", "Implement user auth flow", "2", "To Do", "Alice", "ENG-Platform Auth", []string{"security"}},
		{"ENG-102", "Add pagination to list API", "2", "To Do", "", "ENG-API V2", []string{"backend"}},
		{"ENG-103", "Fix mobile nav overlap", "2", "To Do", "Bob", "ENG-Mobile UX", []string{"bug", "frontend"}},
		{"ENG-104", "Write migration script for DB", "2", "To Do", "Carol", "ENG-DB Migration", nil},
		{"ENG-105", "Deploy v2.1 to staging", "2", "To Do", "Alice", "ENG-Release V2.1", []string{"release"}},
		{"ENG-106", "Set up monitoring dashboards", "2", "To Do", "Dave", "ENG-DevOps Infra", []string{"devops"}},
		{"ENG-201", "Refactor payment service", "3", "In Progress", "Alice", "ENG-Payment V2", []string{"tech-debt"}},
		{"ENG-202", "Update CI pipeline", "3", "In Progress", "Dave", "ENG-DevOps Infra", []string{"devops"}},
		{"ENG-203", "Add rate limiting middleware", "3", "In Progress", "Bob", "ENG-Platform Auth", []string{"security", "backend"}},
		{"ENG-204", "Implement webhook retry logic", "3", "In Progress", "", "ENG-API V2", nil},
		{"ENG-205", "Migrate to new CDN", "3", "In Progress", "Carol", "ENG-DevOps Infra", []string{"infra"}},
		{"ENG-301", "Review SSO integration PR", "5", "In Review", "Carol", "ENG-Platform Auth", []string{"security"}},
		{"ENG-302", "Review logging changes", "5", "In Review", "Dave", "ENG-DevOps Infra", nil},
		{"ENG-303", "Configure load balancer", "5", "In Review", "Bob", "ENG-DevOps Infra", []string{"infra"}},
		{"ENG-304", "Roll back v2.0 hotfix", "5", "In Review", "Alice", "ENG-Release V2.1", []string{"release"}},
	}
	store.issues = make(map[string]*issueStatus, len(initial))
	for i := range initial {
		store.issues[initial[i].Key] = &initial[i]
	}
	store.nextID = 104
	store.assigneeList = []assignableUser{
		{AccountID: "alice-1", DisplayName: "Alice", Active: true},
		{AccountID: "bob-2", DisplayName: "Bob", Active: true},
		{AccountID: "carol-3", DisplayName: "Carol", Active: true},
		{AccountID: "dave-4", DisplayName: "Dave", Active: true},
		{AccountID: "eve-5", DisplayName: "Eve", Active: true},
	}
	store.assignees = make(map[string]string)
	for _, u := range store.assigneeList {
		for _, iss := range initial {
			if iss.Assignee == u.DisplayName {
				store.assignees[iss.Key] = u.AccountID
			}
		}
	}
	store.epicList = []epicInfo{
		{Key: "ENG-Platform Auth", Summary: "Platform Auth", Name: "Platform Auth"},
		{Key: "ENG-API V2", Summary: "API V2", Name: "API V2"},
		{Key: "ENG-Mobile UX", Summary: "Mobile UX", Name: "Mobile UX"},
		{Key: "ENG-DB Migration", Summary: "DB Migration", Name: "DB Migration"},
		{Key: "ENG-Payment V2", Summary: "Payment V2", Name: "Payment V2"},
		{Key: "ENG-DevOps Infra", Summary: "DevOps Infra", Name: "DevOps Infra"},
		{Key: "ENG-Release V2.1", Summary: "Release V2.1", Name: "Release V2.1"},
	}
}

var statusMap = map[string]string{"1": "Backlog", "2": "To Do", "3": "In Progress", "4": "Done", "5": "In Review"}

var transitionMap = map[string]string{
	"21": "2", "31": "3", "41": "4", "51": "5",
}

func main() {
	initStore()

	mux := http.NewServeMux()

	mux.HandleFunc("/rest/agile/1.0/board/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/configuration") {
			_ = json.NewEncoder(w).Encode(boardConfig())
			return
		}
		_ = json.NewEncoder(w).Encode(boardIssues())
	})

	mux.HandleFunc("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			jql := r.URL.Query().Get("jql")
			if strings.Contains(jql, "type = Epic") || strings.Contains(strings.ToLower(jql), "type = epic") {
				query := strings.ToLower(r.URL.Query().Get("query"))
				_ = json.NewEncoder(w).Encode(epicSearchResults(query))
				return
			}
		}
		// POST with JQL body — return all issues
		_ = json.NewEncoder(w).Encode(searchResults())
	})

	mux.HandleFunc("/rest/api/3/user/assignable/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		query := strings.ToLower(r.URL.Query().Get("query"))
		store.RLock()
		defer store.RUnlock()
		_ = json.NewEncoder(w).Encode(filterAssignees(query))
	})

	mux.HandleFunc("/rest/api/3/myself", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":    "current-user",
			"displayName":  "Demo User",
			"emailAddress": "demo@example.com",
			"active":       true,
		})
	})

	mux.HandleFunc("/rest/api/3/project/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(projectDetail())
	})

	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body struct {
			Fields struct {
				Parent    map[string]string `json:"parent"`
				Project   map[string]string `json:"project"`
				IssueType map[string]string `json:"issuetype"`
				Summary   string            `json:"summary"`
			} `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		store.Lock()
		store.nextID++
		id := store.nextID
		key := fmt.Sprintf("ENG-%d", id)
		isSubtask := body.Fields.Parent != nil && body.Fields.Parent["key"] != ""
		statusID := "2"
		statusName := "To Do"
		if isSubtask {
			parentKey := body.Fields.Parent["key"]
			if parent, ok := store.issues[parentKey]; ok {
				statusID = parent.StatusID
				statusName = parent.StatusName
			}
		}
		store.issues[key] = &issueStatus{
			Key:        key,
			Summary:    body.Fields.Summary,
			StatusID:   statusID,
			StatusName: statusName,
		}
		store.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   strconv.Itoa(id),
			"key":  key,
			"self":  "http://localhost:9191/rest/api/3/issue/" + key,
		})
	})

	mux.HandleFunc("/rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		if r.Method == http.MethodPut && strings.HasSuffix(path, "/assignee") {
			key := extractIssueKey(path, "/assignee")
			var body struct {
				AccountID string `json:"accountId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			store.Lock()
			if iss, ok := store.issues[key]; ok {
				iss.Assignee = assigneeName(body.AccountID)
				store.assignees[key] = body.AccountID
			}
			store.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method == http.MethodGet && strings.HasSuffix(path, "/transitions") {
			_ = json.NewEncoder(w).Encode(transitions())
			return
		}

		if r.Method == http.MethodGet {
			key, ok := strings.CutPrefix(path, "/rest/api/3/issue/")
			if !ok {
				http.NotFound(w, r)
				return
			}
			key = strings.TrimSuffix(key, "/transitions")
			store.RLock()
			iss, found := store.issues[key]
			store.RUnlock()
			if found {
				_ = json.NewEncoder(w).Encode(issueDetail(iss))
				return
			}
			http.NotFound(w, r)
			return
		}

		// POST transition — persist it
		key := extractIssueKey(path, "/transitions")
		var body struct {
			Transition struct {
				ID string `json:"id"`
			} `json:"transition"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if toStatusID, ok := transitionMap[body.Transition.ID]; ok {
			store.Lock()
			if iss, ok := store.issues[key]; ok {
				iss.StatusID = toStatusID
				iss.StatusName = statusMap[toStatusID]
			}
			store.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	})

	addr := ":9191"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	fmt.Fprintf(os.Stderr, "dummy jira server on %s\n", addr)
	logHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "%s %s\n", r.Method, r.URL.String())
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: addr, Handler: logHandler, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func extractIssueKey(path, suffix string) string {
	s := strings.TrimSuffix(path, suffix)
	key, _ := strings.CutPrefix(s, "/rest/api/3/issue/")
	return key
}

func assigneeName(accountID string) string {
	for _, u := range store.assigneeList {
		if u.AccountID == accountID {
			return u.DisplayName
		}
	}
	return ""
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
		"ranking": map[string]any{
			"rankCustomFieldId": 10020,
		},
	}
}

func boardIssues() any {
	time.Sleep(300 * time.Millisecond)
	store.RLock()
	defer store.RUnlock()
	issues := make([]any, 0, len(store.issues))
	for _, iss := range store.issues {
		issues = append(issues, marshalIssue(iss))
	}
	return map[string]any{
		"startAt":    0,
		"maxResults": 50,
		"total":      len(issues),
		"issues":     issues,
	}
}

func searchResults() any {
	store.RLock()
	defer store.RUnlock()
	issues := make([]any, 0, len(store.issues))
	for _, iss := range store.issues {
		issues = append(issues, marshalIssue(iss))
	}
	return map[string]any{"issues": issues}
}

func marshalIssue(iss *issueStatus) any {
	var assignee any
	if iss.Assignee != "" {
		assignee = map[string]string{"displayName": iss.Assignee}
	}
	var epic any
	if iss.Epic != "" {
		epicInfo := findEpicByKey(iss.Epic)
		epic = map[string]string{
			"key":      iss.Epic,
			"summary":  epicInfo.Summary,
			"name":     epicInfo.Name,
		}
	}
	return map[string]any{
		"key": iss.Key,
		"fields": map[string]any{
			"summary":  iss.Summary,
			"status":   map[string]string{"id": iss.StatusID, "name": iss.StatusName},
			"assignee": assignee,
			"labels":    iss.Labels,
			"epic":      epic,
		},
	}
}

func findEpicByKey(key string) epicInfo {
	for _, e := range store.epicList {
		if e.Key == key {
			return e
		}
	}
	return epicInfo{Key: key, Summary: key, Name: key}
}

func epicSearchResults(query string) any {
	store.RLock()
	defer store.RUnlock()
	var result []any
	for _, e := range store.epicList {
		if query != "" && !strings.Contains(strings.ToLower(e.Summary), query) && !strings.Contains(strings.ToLower(e.Name), query) {
			continue
		}
		result = append(result, map[string]any{
			"key": e.Key,
			"fields": map[string]any{
				"summary": e.Summary,
				"epic":    map[string]string{"name": e.Name},
			},
		})
	}
	return map[string]any{"issues": result}
}

func issueDetail(iss *issueStatus) any {
	desc := adfParagraph(iss.Summary + " — details for " + iss.Key)
	if iss.Key == "ENG-101" {
		desc = adfFromLines(
			"Implement the full OAuth 2.0 authorization code flow.",
			"Support PKCE and refresh token rotation.",
			"",
			"Acceptance criteria:",
			"• Login via Google and GitHub providers",
			"• Token refresh without re-auth",
			"• Session timeout after 30 min",
		)
	}
	var assignee any
	if iss.Assignee != "" {
		assignee = map[string]string{"displayName": iss.Assignee}
	}
	var epic any
	if iss.Epic != "" {
		epicInfo := findEpicByKey(iss.Epic)
		epic = map[string]string{
			"key":      iss.Epic,
			"summary":  epicInfo.Summary,
			"name":     epicInfo.Name,
		}
	}
	return map[string]any{
		"key": iss.Key,
		"fields": map[string]any{
			"summary":     iss.Summary,
			"status":      map[string]string{"id": iss.StatusID, "name": iss.StatusName},
			"assignee":    assignee,
			"labels":      iss.Labels,
			"description": desc,
			"epic":        epic,
		},
	}
}

func adfParagraph(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []map[string]any{
			{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": text},
				},
			},
		},
	}
}

func adfFromLines(lines ...string) map[string]any {
	var content []map[string]any
	for _, line := range lines {
		if line == "" {
			continue
		}
		if text, ok := strings.CutPrefix(line, "• "); ok {
			content = append(content, map[string]any{
				"type": "bulletList",
				"content": []map[string]any{
					{
						"type": "listItem",
						"content": []map[string]any{
							{
								"type": "paragraph",
								"content": []map[string]any{
									{"type": "text", "text": text},
								},
							},
						},
					},
				},
			})
		} else {
			content = append(content, map[string]any{
				"type": "paragraph",
				"content": []map[string]any{
					{"type": "text", "text": line},
				},
			})
		}
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
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

func filterAssignees(query string) []assignableUser {
	if query == "" {
		return store.assigneeList
	}
	var result []assignableUser
	for _, u := range store.assigneeList {
		if strings.Contains(strings.ToLower(u.DisplayName), query) {
			result = append(result, u)
		}
	}
	return result
}

func projectDetail() any {
	return map[string]any{
		"key": "ENG",
		"name": "Engineering",
		"issueTypes": []map[string]any{
			{"id": "10001", "name": "Task", "subtask": false},
			{"id": "10002", "name": "Bug", "subtask": false},
			{"id": "10003", "name": "Story", "subtask": false},
			{"id": "10004", "name": "Epic", "subtask": false},
			{"id": "10005", "name": "Sub-task", "subtask": true},
		},
	}
}