package jira

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/raulvc/jira-kanban/internal/cache"
)

// Progress reports fetch progress to the caller.
type Progress struct {
	Cold    bool
	Fetched int
	Total   int
}

// SyncProgress reports background sync progress.
type SyncProgress struct {
	Phase   string
	Fetched int
	Total   int
}

// FetchBoard retrieves the board configuration and all visible issues.
// On cold start (no cache) it does a full fetch synchronously with progress.
// On warm start it returns cached data immediately; the caller should run
// a background RefreshBoard to keep the cache up to date.
func (c *Client) FetchBoard(boardID int, onProgress func(Progress)) (Board, bool, error) {
	columns, boardName, rankFieldID, err := c.fetchColumns(boardID)
	if err != nil {
		return Board{}, false, err
	}
	c.RankFieldID = rankFieldID
	statusIDs := visibleStatusIDs(columns)

	store, err := cache.Load(boardID)
	if err != nil {
		return Board{}, false, fmt.Errorf("loading cache: %w", err)
	}

	if store.IsEmpty() {
		issues, fetchErr := c.fetchIssues(boardID, statusIDs, rankFieldID, true, onProgress)
		if fetchErr != nil {
			return Board{}, false, fetchErr
		}
		store.Merge(issuesToEntries(issues), time.Now())

		board := buildBoard(boardName, columns, entriesToIssues(store.Issues))
		pruneCache(&store, board)
		if err := store.Save(); err != nil {
			return Board{}, false, fmt.Errorf("saving cache: %w", err)
		}
		return board, false, nil
	}

	board := buildBoard(boardName, columns, entriesToIssues(store.Issues))
	return board, true, nil
}

// syncCache performs an incremental sync:
//  1. Fetches issues updated since the last sync timestamp.
//  2. Re-validates only the keys currently in cache to detect removals.
//
// This avoids downloading every issue on the board (which can be 1000+)
// and instead touches only what changed plus what is visible.
func (c *Client) syncCache(store *cache.Store, boardID int, statusIDs []string, onProgress func(SyncProgress)) error {
	// 1. Fetch issues changed since last sync.
	since := store.FetchedAt
	if since.IsZero() {
		// No timestamp — fall back to a full stub scan.
		return c.syncCacheFull(store, boardID, statusIDs, onProgress)
	}

	// Subtract a safety margin so we don't miss issues updated during
	// the previous sync window.
	since = since.Add(-2 * time.Minute)

	changed, err := c.fetchChangedIssues(boardID, statusIDs, since, onProgress)
	if err != nil {
		return err
	}

	if len(changed) > 0 {
		store.Merge(issuesToEntries(changed), time.Now())
	}

	// 2. Re-validate cached keys: check which ones are still on the board
	//    with a visible status.  Only the keys we already have — not the
	//    entire board.
	cachedKeys := make([]string, 0, len(store.Issues))
	for k := range store.Issues {
		cachedKeys = append(cachedKeys, k)
	}

	if len(cachedKeys) > 0 {
		live, fetchErr := c.fetchKeyStatuses(cachedKeys, onProgress)
		if fetchErr != nil {
			return fetchErr
		}

		visibleSet := make(map[string]bool, len(statusIDs))
		for _, id := range statusIDs {
			visibleSet[id] = true
		}

		for _, key := range cachedKeys {
			status, exists := live[key]
			if !exists || !visibleSet[status] {
				delete(store.Issues, key)
			}
		}
	}

	store.FetchedAt = time.Now()
	return nil
}

// syncCacheFull is the original full-scan strategy used when we have no
// prior sync timestamp.
func (c *Client) syncCacheFull(store *cache.Store, boardID int, statusIDs []string, onProgress func(SyncProgress)) error {
	stubs, err := c.fetchStubs(boardID, statusIDs, onProgress)
	if err != nil {
		return err
	}

	var stale []string
	for _, s := range stubs {
		cached, ok := store.Issues[s.key]
		if !ok || cached.StatusID != s.statusID || cached.Summary == "" {
			stale = append(stale, s.key)
		}
	}

	if len(stale) > 0 {
		if onProgress != nil {
			onProgress(SyncProgress{Phase: "Updating", Total: len(stale)})
		}
		fresh, fetchErr := c.fetchIssuesByKeys(stale, func(fetched int) {
			if onProgress != nil {
				onProgress(SyncProgress{Phase: "Updating", Fetched: fetched, Total: len(stale)})
			}
		})
		if fetchErr != nil {
			return fetchErr
		}
		store.Merge(issuesToEntries(fresh), time.Now())
	} else {
		store.FetchedAt = time.Now()
	}

	liveKeys := make(map[string]bool, len(stubs))
	for _, s := range stubs {
		liveKeys[s.key] = true
	}
	for key := range store.Issues {
		if !liveKeys[key] {
			delete(store.Issues, key)
		}
	}
	return nil
}

// UpdateCachedStatus writes a status change directly to the on-disk cache
// so that subsequent syncs don't revert an optimistic UI update.
func (c *Client) UpdateCachedStatus(boardID int, issueKey, statusID, statusName string) {
	store, err := cache.Load(boardID)
	if err != nil {
		return
	}
	store.UpdateStatus(issueKey, statusID, statusName)
	_ = store.Save()
}

// UpdateCachedAssignee persists an assignee change to the cache file
// so that subsequent syncs don't revert an optimistic UI update.
func (c *Client) UpdateCachedAssignee(boardID int, issueKey, assignee string) {
	store, err := cache.Load(boardID)
	if err != nil {
		return
	}
	store.UpdateAssignee(issueKey, assignee)
	_ = store.Save()
}

// RefreshBoard re-syncs the board using the same stub-diff strategy
// as FetchBoard. Intended for in-TUI refresh.
func (c *Client) RefreshBoard(boardID int, onProgress func(SyncProgress)) (Board, error) {
	columns, boardName, rankFieldID, err := c.fetchColumns(boardID)
	if err != nil {
		return Board{}, err
	}
	c.RankFieldID = rankFieldID
	statusIDs := visibleStatusIDs(columns)

	store, loadErr := cache.Load(boardID)
	if loadErr != nil {
		store = cache.Store{BoardID: boardID, Issues: map[string]cache.Entry{}}
	}

	if err := c.syncCache(&store, boardID, statusIDs, onProgress); err != nil {
		return Board{}, err
	}

	board := buildBoard(boardName, columns, entriesToIssues(store.Issues))
	pruneCache(&store, board)
	if err := store.Save(); err != nil {
		return Board{}, fmt.Errorf("saving cache: %w", err)
	}
	return board, nil
}

// issueStub is a lightweight key+status pair used for diffing.
type issueStub struct {
	key      string
	statusID string
}

// fetchStubs fetches only key and status.id for all visible board issues.
func (c *Client) fetchStubs(boardID int, statusIDs []string, onProgress func(SyncProgress)) ([]issueStub, error) {
	jql := "ORDER BY rank ASC"
	if len(statusIDs) > 0 {
		jql = "status in (" + strings.Join(statusIDs, ",") + ") " + jql
	}

	var all []issueStub
	for startAt := 0; ; startAt += 50 {
		u := fmt.Sprintf(
			"%s/rest/agile/1.0/board/%d/issue?startAt=%d&maxResults=50&fields=status&jql=%s",
			c.BaseURL, boardID, startAt, url.QueryEscape(jql))
		var resp boardIssuesResponse
		if err := c.getJSON(u, &resp); err != nil {
			return nil, fmt.Errorf("board stubs (offset %d): %w", startAt, err)
		}
		for _, iss := range resp.Issues {
			all = append(all, issueStub{
				key:      iss.Key,
				statusID: strings.TrimSpace(iss.Fields.Status.ID),
			})
		}
		if onProgress != nil {
			onProgress(SyncProgress{
				Phase:   "Scanning",
				Fetched: min(len(all), resp.Total),
				Total:   resp.Total,
			})
		}
		if len(all) >= resp.Total || len(resp.Issues) == 0 {
			break
		}
	}
	return all, nil
}

// fetchChangedIssues fetches full details for board issues updated since the
// given timestamp.  Only issues with a visible status are returned.
func (c *Client) fetchChangedIssues(boardID int, statusIDs []string, since time.Time, onProgress func(SyncProgress)) ([]issue, error) {
	jql := fmt.Sprintf("updated >= \"%s\" ORDER BY updated DESC",
		since.UTC().Format("2006-01-02 15:04"))
	if len(statusIDs) > 0 {
		jql = "status in (" + strings.Join(statusIDs, ",") + ") AND " + jql
	}

	var all []issue
	for startAt := 0; ; startAt += 50 {
		u := fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue?startAt=%d&maxResults=50&jql=%s",
			c.BaseURL, boardID, startAt, url.QueryEscape(jql))
		raw, err := c.getRaw(u)
		if err != nil {
			return nil, fmt.Errorf("changed issues (offset %d): %w", startAt, err)
		}
		var resp boardIssuesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("changed issues (offset %d): %w", startAt, err)
		}
		extractRanks(resp.Issues, raw, c.RankFieldID)
		all = append(all, resp.Issues...)
		if onProgress != nil {
			onProgress(SyncProgress{
				Phase:   "Syncing",
				Fetched: min(len(all), resp.Total),
				Total:   resp.Total,
			})
		}
		if len(all) >= resp.Total || len(resp.Issues) == 0 {
			break
		}
	}
	return all, nil
}

// fetchKeyStatuses returns a map of issue key → status ID for the given
// keys.  It uses a lightweight search (fields=status) so the response is
// small.  Keys that no longer exist are simply absent from the result.
func (c *Client) fetchKeyStatuses(keys []string, onProgress func(SyncProgress)) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	const batchSize = 50
	for start := 0; start < len(keys); start += batchSize {
		end := min(start+batchSize, len(keys))
		batch := keys[start:end]
		jql := "key in (" + strings.Join(batch, ",") + ")"

		u := fmt.Sprintf("%s/rest/api/3/search/jql", c.BaseURL)
		reqBody := map[string]any{
			"jql":        jql,
			"maxResults": len(batch),
			"fields":     []string{"status"},
		}
		var resp searchJqlResponse
		if err := c.postJSONResponse(u, reqBody, &resp); err != nil {
			return nil, fmt.Errorf("key statuses: %w", err)
		}
		for _, iss := range resp.Issues {
			result[iss.Key] = strings.TrimSpace(iss.Fields.Status.ID)
		}
		if onProgress != nil {
			onProgress(SyncProgress{
				Phase:   "Validating",
				Fetched: min(end, len(keys)),
				Total:   len(keys),
			})
		}
	}
	return result, nil
}

// fetchIssuesByKeys fetches full details for specific issue keys using
// the v3 search/jql endpoint with token-based pagination.
// Batches are fetched in parallel for speed.
// onProgress reports the cumulative number of issues fetched so far.
func (c *Client) fetchIssuesByKeys(keys []string, onProgress func(fetched int)) ([]issue, error) {
	const batchSize = 50
	const maxConcurrent = 20
	nBatches := (len(keys) + batchSize - 1) / batchSize

	type batchResult struct {
		issues []issue
		raw    []byte
		err    error
		idx    int
	}

	var fetched atomic.Int64
	var wg sync.WaitGroup
	results := make([]batchResult, nBatches)
	sem := make(chan struct{}, maxConcurrent)
	wg.Add(nBatches)

	for i := range nBatches {
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			start := i * batchSize
			end := min(start+batchSize, len(keys))
			batch := keys[start:end]

			jql := "key in (" + strings.Join(batch, ",") + ")"
			u := fmt.Sprintf("%s/rest/api/3/search/jql", c.BaseURL)

			fields := []string{"summary", "status", "assignee", "labels"}
			if c.RankFieldID > 0 {
				fields = append(fields, fmt.Sprintf("customfield_%d", c.RankFieldID))
			}

			var pageIssues []issue
			var token string
			var allRaw []byte
			for {
				reqBody := map[string]any{
					"jql":        jql,
					"maxResults": len(batch),
					"fields":     fields,
				}
				if token != "" {
					reqBody["nextPageToken"] = token
				}
				raw, postErr := c.postJSONRaw(u, reqBody)
				if postErr != nil {
					results[i] = batchResult{err: fmt.Errorf("batch %d: %w", i, postErr), idx: i}
					return
				}
				var resp searchJqlResponse
				if err := json.Unmarshal(raw, &resp); err != nil {
					results[i] = batchResult{err: fmt.Errorf("batch %d: %w", i, err), idx: i}
					return
				}
				pageIssues = append(pageIssues, resp.Issues...)
				allRaw = append(allRaw, raw...)
				if onProgress != nil {
					onProgress(int(fetched.Add(int64(len(resp.Issues)))))
				}
				if resp.NextPageToken == "" {
					break
				}
				token = resp.NextPageToken
			}
			results[i] = batchResult{issues: pageIssues, raw: allRaw, idx: i}
		}(i)
	}

	wg.Wait()

	var firstErr error
	ordered := make([][]issue, nBatches)
	for _, r := range results {
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		extractRanks(r.issues, r.raw, c.RankFieldID)
		ordered[r.idx] = r.issues
	}
	if firstErr != nil {
		return nil, firstErr
	}

	var all []issue
	for _, batch := range ordered {
		all = append(all, batch...)
	}
	return all, nil
}

// visibleStatusIDs collects status IDs from all non-Backlog columns.
func visibleStatusIDs(columns []columnMapping) []string {
	var ids []string
	for _, col := range columns {
		if strings.EqualFold(col.Name, "Backlog") {
			continue
		}
		ids = append(ids, col.StatusIDs...)
	}
	return ids
}

// fetchColumns returns the column layout from the board configuration endpoint.
func (c *Client) fetchColumns(boardID int) ([]columnMapping, string, int, error) {
	cfgURL := fmt.Sprintf("%s/rest/agile/1.0/board/%d/configuration", c.BaseURL, boardID)
	var resp boardConfigResponse
	if err := c.getJSON(cfgURL, &resp); err != nil {
		return nil, "", 0, fmt.Errorf("board config: %w", err)
	}
	var cols []columnMapping
	for _, col := range resp.ColumnConfig.Columns {
		statusIDs := make([]string, 0, len(col.Statuses))
		statusNames := make([]string, 0, len(col.Statuses))
		for _, s := range col.Statuses {
			statusIDs = append(statusIDs, s.ID)
			if s.Name != "" {
				statusNames = append(statusNames, s.Name)
			}
		}
		cols = append(cols, columnMapping{
			Name:        col.Name,
			StatusIDs:   statusIDs,
			StatusNames: statusNames,
		})
	}
	return cols, resp.Name, resp.Ranking.RankCustomFieldID, nil
}

// fetchIssues pages through the board issues endpoint, filtered to only
// the given visible status IDs.
func (c *Client) fetchIssues(boardID int, statusIDs []string, rankFieldID int, cold bool, onProgress func(Progress)) ([]issue, error) {
	jql := "ORDER BY rank ASC"
	if len(statusIDs) > 0 {
		jql = "status in (" + strings.Join(statusIDs, ",") + ") " + jql
	}

	var all []issue
	for startAt := 0; ; startAt += 50 {
		u := fmt.Sprintf("%s/rest/agile/1.0/board/%d/issue?startAt=%d&maxResults=50&jql=%s",
			c.BaseURL, boardID, startAt, url.QueryEscape(jql))
		raw, err := c.getRaw(u)
		if err != nil {
			return nil, fmt.Errorf("board issues (offset %d): %w", startAt, err)
		}
		var resp boardIssuesResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("board issues (offset %d): %w", startAt, err)
		}
		extractRanks(resp.Issues, raw, rankFieldID)
		all = append(all, resp.Issues...)

		if onProgress != nil {
			onProgress(Progress{
				Cold:    cold,
				Fetched: min(len(all), resp.Total),
				Total:   resp.Total,
			})
		}

		if len(all) >= resp.Total || len(resp.Issues) == 0 {
			break
		}
	}
	return all, nil
}

// pruneCache removes cache entries for issues that aren't visible on the
// board (e.g. excess "Done" issues trimmed by buildBoard).  This keeps the
// cache small so incremental syncs only re-validate what the UI shows.
func pruneCache(store *cache.Store, board Board) {
	visible := board.VisibleKeys()
	for key := range store.Issues {
		if !visible[key] {
			delete(store.Issues, key)
		}
	}
}

// ── conversion between cache entries and internal issue type ────────────────

func issuesToEntries(issues []issue) []cache.Entry {
	entries := make([]cache.Entry, len(issues))
	for i, iss := range issues {
		entries[i] = cache.Entry{
			Key:         iss.Key,
			Summary:     iss.Fields.Summary,
			StatusID:    strings.TrimSpace(iss.Fields.Status.ID),
			Status:      strings.TrimSpace(iss.Fields.Status.Name),
			Assignee:    assigneeName(iss),
			Labels:      iss.Fields.Labels,
			Description: parseDescription(iss.Fields.Description),
			Epic:        epicName(iss),
			Rank:        iss.Rank,
		}
	}
	return entries
}

func entriesToIssues(entries map[string]cache.Entry) []issue {
	issues := make([]issue, 0, len(entries))
	for _, e := range entries {
		var iss issue
		iss.Key = e.Key
		iss.Fields.Summary = e.Summary
		iss.Fields.Status.ID = e.StatusID
		iss.Fields.Status.Name = e.Status
		iss.Fields.Labels = e.Labels
		if e.Assignee != "" && e.Assignee != "Unassigned" {
			iss.Fields.Assignee = &struct {
				DisplayName string `json:"displayName"`
			}{DisplayName: e.Assignee}
		}
		if e.Epic != "" {
			iss.Fields.Epic = &issueEpic{Summary: e.Epic}
		}
		iss.Rank = e.Rank
		issues = append(issues, iss)
	}
	return issues
}

// ── column mapping and board building ───────────────────────────────────────

// columnMapping pairs a column name with the status IDs/names that map to it.
type columnMapping struct {
	Name        string
	StatusIDs   []string
	StatusNames []string
}

// buildBoard assigns issues to columns, skips Backlog, and sorts each column.
func buildBoard(boardName string, mappings []columnMapping, issues []issue) Board {
	statusIDToCol := map[string]string{}
	statusNameToCol := map[string]string{}
	columns := make([]Column, 0, len(mappings))

	issueStatusNames := map[string]string{}
	for _, iss := range issues {
		sid := strings.TrimSpace(iss.Fields.Status.ID)
		sname := strings.TrimSpace(iss.Fields.Status.Name)
		if sid != "" && sname != "" {
			issueStatusNames[sid] = sname
		}
	}

	for _, m := range mappings {
		if strings.EqualFold(m.Name, "Backlog") {
			continue
		}
		columns = append(columns, Column{Name: m.Name})
		for i, sid := range m.StatusIDs {
			statusIDToCol[sid] = m.Name
			name := ""
			if i < len(m.StatusNames) {
				name = m.StatusNames[i]
			}
			if name == "" {
				name = issueStatusNames[sid]
			}
			if name != "" {
				statusNameToCol[strings.ToLower(name)] = m.Name
			}
		}
	}

	colIndex := map[string]int{}
	for i, col := range columns {
		colIndex[col.Name] = i
	}

	for _, iss := range issues {
		card := Card{
			Key:      iss.Key,
			Summary:  iss.Fields.Summary,
			Status:   iss.Fields.Status.Name,
			Assignee: assigneeName(iss),
			Labels:   iss.Fields.Labels,
			Epic:     epicName(iss),
			Rank:     iss.Rank,
		}

		colName := resolveColumn(iss, statusIDToCol, statusNameToCol)
		idx, ok := colIndex[colName]
		if !ok {
			columns = append(columns, Column{Name: colName})
			idx = len(columns) - 1
			colIndex[colName] = idx
		}
		columns[idx].Issues = append(columns[idx].Issues, card)
	}

	lastCol := len(columns) - 1
	if lastCol >= 0 {
		const maxFinal = 20
		if len(columns[lastCol].Issues) > maxFinal {
			columns[lastCol].Issues = columns[lastCol].Issues[:maxFinal]
		}
	}

	// Rank-ordered sort: issues with a rank value come first (sorted by rank),
	// issues without rank fall back to key order.
	for i := range columns {
		sort.SliceStable(columns[i].Issues, func(a, b int) bool {
			ca, cb := columns[i].Issues[a], columns[i].Issues[b]
			if ca.Rank != "" && cb.Rank != "" {
				return ca.Rank < cb.Rank
			}
			if ca.Rank != "" {
				return true
			}
			if cb.Rank != "" {
				return false
			}
			return ca.Key < cb.Key
		})
	}

	return Board{Name: boardName, Columns: columns}
}

func resolveColumn(iss issue, byID, byName map[string]string) string {
	if name, ok := byID[strings.TrimSpace(iss.Fields.Status.ID)]; ok {
		return name
	}
	if name, ok := byName[strings.ToLower(strings.TrimSpace(iss.Fields.Status.Name))]; ok {
		return name
	}
	return iss.Fields.Status.Name
}

func assigneeName(iss issue) string {
	if iss.Fields.Assignee != nil && iss.Fields.Assignee.DisplayName != "" {
		return iss.Fields.Assignee.DisplayName
	}
	return "Unassigned"
}

func epicName(iss issue) string {
	if iss.Fields.Epic != nil && iss.Fields.Epic.Summary != "" {
		return iss.Fields.Epic.Summary
	}
	if iss.Fields.Epic != nil && iss.Fields.Epic.Name != "" {
		return iss.Fields.Epic.Name
	}
	if iss.Fields.Parent != nil && iss.Fields.Parent.Key != "" {
		return iss.Fields.Parent.Fields.Summary
	}
	return ""
}

// issueNum extracts the numeric suffix from a key like "PROJ-1234".
func issueNum(key string) int {
	if i := strings.LastIndex(key, "-"); i >= 0 {
		n, _ := strconv.Atoi(key[i+1:])
		return n
	}
	return 0
}
