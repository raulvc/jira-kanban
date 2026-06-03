// Package cache persists board issue data locally so that subsequent loads
// only need to fetch issues updated since the last sync.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry is a cached issue stored by key.
type Entry struct {
	Key         string   `json:"key"`
	Summary     string   `json:"summary"`
	StatusID    string   `json:"status_id"`
	Status      string   `json:"status"`
	Assignee    string   `json:"assignee"`
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
	Epic        string   `json:"epic"`
	Rank        string   `json:"rank,omitempty"`
}

// Store holds the full cache state for a single board.
type Store struct {
	BoardID   int              `json:"board_id"`
	FetchedAt time.Time        `json:"fetched_at"`
	Issues    map[string]Entry `json:"issues"`
}

// Path returns the cache file location for a given board ID.
func Path(boardID int) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jira-kanban", fmt.Sprintf("board-%d.json", boardID)), nil
}

// Load reads a cache file. Returns a zero Store (not an error) if the file
// does not exist.
func Load(boardID int) (Store, error) {
	p, err := Path(boardID)
	if err != nil {
		return Store{}, err
	}
	data, err := os.ReadFile(p) //nolint:gosec // path derived from cache dir + board ID
	if os.IsNotExist(err) {
		return Store{BoardID: boardID, Issues: map[string]Entry{}}, nil
	}
	if err != nil {
		return Store{}, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupt cache — treat as cold start.
		return Store{BoardID: boardID, Issues: map[string]Entry{}}, nil //nolint:nilerr // intentional: corrupt cache is not fatal
	}
	if s.Issues == nil {
		s.Issues = map[string]Entry{}
	}
	return s, nil
}

// Save writes the cache to disk.
func (s *Store) Save() error {
	p, err := Path(s.BoardID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Merge upserts entries into the cache and updates FetchedAt.
// If a new entry has no Rank but an existing entry does, the Rank is preserved.
func (s *Store) Merge(entries []Entry, fetchedAt time.Time) {
	for _, e := range entries {
		if e.Rank == "" {
			if existing, ok := s.Issues[e.Key]; ok && existing.Rank != "" {
				e.Rank = existing.Rank
			}
		}
		s.Issues[e.Key] = e
	}
	s.FetchedAt = fetchedAt
}

// UpdateStatus changes the status of a cached issue and persists to disk.
func (s *Store) UpdateStatus(key, statusID, statusName string) {
	e, ok := s.Issues[key]
	if !ok {
		return
	}
	e.StatusID = statusID
	e.Status = statusName
	s.Issues[key] = e
}

// IsEmpty reports whether the cache has any issues.
func (s *Store) IsEmpty() bool {
	return len(s.Issues) == 0
}

// SinceDuration returns how long ago the cache was last fetched.
// Returns zero if the cache has never been populated.
func (s *Store) SinceDuration() time.Duration {
	if s.FetchedAt.IsZero() {
		return 0
	}
	return time.Since(s.FetchedAt)
}
