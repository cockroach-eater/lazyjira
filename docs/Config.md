# Configuration

lazyjira is configured through a YAML file.

## Config file location

| OS | Path |
|----|------|
| Linux | `~/.config/lazyjira/config.yml` |
| macOS | `~/Library/Application Support/lazyjira/config.yml` |
| Windows | `%AppData%\lazyjira\config.yml` |

You can override the config directory with the `CONFIG_DIR` environment variable. `XDG_CONFIG_HOME` is also respected on Linux.

## Environment variables

Jira credentials can be set via environment variables. These always take priority over the config file and auth.json.

| Variable | Description |
|----------|-------------|
| `JIRA_HOST` | Jira instance URL |
| `JIRA_EMAIL` | Account email |
| `JIRA_API_TOKEN` | API token |

## Default config

Do not copy the entire thing into your config. Only add the settings you want to change.
```yaml
jira:
    host: ""
    email: ""
projects: []
gui:
    theme: default
    language: en
    sidePanelWidth: 40
    showIcons: true
    dateFormat: "2006-01-02"
    mouse: true
    borders: rounded
    issueListFields:
        - key
        - status
        - summary
keybinding:
    universal:
        quit: q
        help: '?'
        search: /
        switchPanel: tab
        refresh: r
        refreshAll: R
        prevTab: '['
        nextTab: ']'
        focusDetail: "0"
        focusStatus: "1"
        focusIssues: "2"
        focusInfo: "3"
        focusProjects: "4"
        jqlSearch: s
    issues:
        select: ' '
        open: enter
        focusRight: l
        transition: t
        browser: o
        urlPicker: u
        copyURL: "y"
        closeJQLTab: x
        createBranch: b
    projects:
        select: ' '
        open: enter
        focusRight: l
    detail:
        focusLeft: h
        infoTab: i
issueTabs:
    - name: All
      jql: project = {{.ProjectKey}} AND statusCategory != Done ORDER BY updated DESC
    - name: Assigned
      jql: project = {{.ProjectKey}} AND assignee=currentUser() AND statusCategory != Done ORDER BY priority DESC, updated DESC
cache:
    enabled: true
    ttl: 5m
refresh:
    autoRefresh: true
    interval: 30s
customFields: []
git:
    closeOnCheckout: false
    asciiOnly: false
    branchFormat: []
```

## GUI

```yaml
gui:
  sidePanelWidth: 40
  issueListFields:
    - "key"
    - "status"
    - "summary"
```

`sidePanelWidth` controls the left panel width in columns. It automatically shrinks on narrow terminals.

### Issue list fields

Controls which columns appear in the issue list. Available fields.

| Field | Width | Description |
|-------|-------|-------------|
| `key` | auto | Issue key like PROJ-123 |
| `status` | 1 char | Status indicator |
| `summary` | fills remaining | Issue title |
| `priority` | 8 chars | Priority name |
| `assignee` | 12 chars | Assignee display name |
| `type` | 10 chars | Issue type |
| `updated` | 8 chars | Time since last update |

## Issue tabs

Define JQL-based tabs for the issue list. Template variables `{{.ProjectKey}}` and `{{.UserEmail}}` are expanded at runtime.

```yaml
issueTabs:
  - name: "All"
    jql: "project = {{.ProjectKey}} AND statusCategory != Done ORDER BY updated DESC"
  - name: "Assigned"
    jql: "project = {{.ProjectKey}} AND assignee=currentUser() ORDER BY priority DESC"
  - name: "Recent"
    jql: "project = {{.ProjectKey}} AND updated >= -7d ORDER BY updated DESC"
```

You can also create temporary JQL tabs at runtime with the `s` key.

## Keybindings

All keybindings are remappable. See [Keybindings](Keybindings.md) for the full list of defaults.

```yaml
keybinding:
  universal:
    quit: "q"
    help: "?"
    search: "/"
  issues:
    transition: "t"
    browser: "o"
    createBranch: "b"
```

Only include keys you want to change. Missing keys keep their defaults.

## Custom fields

See [Custom Fields](Custom_Fields.md).

## Git integration

lazyjira can create branches from issues and detect the current issue from your branch name.

```yaml
git:
  closeOnCheckout: false
  asciiOnly: false
  branchFormat:
    - when:
        type: "Bug"
      template: "bugfix/{{.Key}}-{{.Summary | slugify}}"
    - when:
        type: "*"
      template: "{{.Key}}-{{.Summary | slugify}}"
```

### Branch format rules

Each rule has a `when` condition and a `template`. Rules are evaluated in order and the first match wins. Use `type: "*"` as a catch-all.

Template variables.

| Variable | Description |
|----------|-------------|
| `{{.Key}}` | Issue key like PROJ-123 |
| `{{.Summary}}` | Issue summary |
| `{{.Summary \| slugify}}` | Summary as a slug, lowercase with dashes |

## Files

| File | Description |
|------|-------------|
| `config.yml` | Main configuration |
| `auth.json` | Credentials, created automatically with restricted permissions |
| `jql_history.txt` | JQL search history, up to 50 entries |
