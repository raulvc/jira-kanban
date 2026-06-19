# jira-kanban

A fast, lightweight Jira kanban board for the terminal.

<img src="docs/assets/demo_basics.gif" width="100%">

<img src="docs/assets/demo_filter_assign.gif" width="100%">

<img src="docs/assets/demo_create_subtask.gif" width="100%">

Jira's web interface is resource-heavy and slow — it easily consumes hundreds of megabytes of RAM and still feels sluggish. `jira-kanban` gives you a snappy, keyboard-driven view of your board that stays out of your way and uses a fraction of the resources.

## Install

```
go install github.com/raulvc/jira-kanban@latest
```

## First run

On first launch, `jira-kanban` will prompt for any missing configuration values interactively (with masked input for the API token) and save them automatically:

```
$ jira-kanban
Jira base URL [https://your-company.atlassian.net]:
Email [you@company.com]:
API token:
Board ID [42]:
```

After that, the config file is stored at `~/.config/jira-kanban/config.yml` and you won't be prompted again.

### Configuration file

```yaml
base_url: "https://your-company.atlassian.net"
email: "you@company.com"
api_token: "your-jira-api-token"
board_id: 42
```

> **API token** — Create one at [id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).

### Override the board

You can switch boards without editing the config:

```
jira-kanban 17           # positional board ID
jira-kanban --board 17   # flag form
```

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `←` / `→` | Move between columns |
| `↑` / `↓` | Move between cards |
| `f` | Filter by assignee |
| `e` | Filter by epic |
| `enter` | View issue details |
| `t` | Transition issue (with search filter) |
| `o` | Open issue in browser |
| `a` | Assign issue (from board or detail view) |
| `c` | Create issue (from board) / Create subtask (from detail view) |
| `h` | Recent activity history |
| `r` | Refresh board |
| `q` | Quit |

## How it works

- **Cold start** — fetches all visible issues from the board and builds a local cache.
- **Warm start** — returns cached data instantly, then syncs changes in the background.
- **Incremental sync** — only fetches issues that changed since the last sync, instead of re-downloading the entire board.
- **Optimistic transitions** — moving a card updates the UI immediately, then persists to Jira in the background.
- **Assignee management** — assign or unassign issues directly from the board or detail view. The current user is highlighted with a ★ marker.
- **Create issues & subtasks** — create new issues from the board view, and subtasks from the detail modal. Rich text descriptions with URL linking and code blocks are supported. Tab-navigable OK/Cancel buttons.
- **Subtask navigation** — view and select subtasks inside the detail modal; press Enter to open a nested detail view for any subtask.
- **Recent activity history** — press `h` to see your recently updated issues with changelog summaries (e.g. "status: To Do → In Progress · 2h ago").

Cache is stored at `$XDG_CACHE_HOME/jira-kanban/<board-id>.json`.

## License

BSD-2-Clause