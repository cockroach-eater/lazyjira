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
| `JIRA_EMAIL` | Account email (Cloud only) |
| `JIRA_API_TOKEN` | API token / PAT |
| `JIRA_SERVER_TYPE` | `cloud` (default), `server`, or `datacenter` |
| `JIRA_TLS_CERT` | Path to client certificate PEM |
| `JIRA_TLS_KEY` | Path to client private key PEM |
| `JIRA_TLS_CA` | Path to custom CA bundle PEM |
| `JIRA_TLS_INSECURE` | Set to `1` or `true` to skip TLS verification |

## Default config

Do not copy the entire thing into your config. Only add the settings you want to change.

```yaml
jira:
    host: ""
    email: ""
    serverType: cloud
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
    selectCreatedIssue: true
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
fields: []
git:
    closeOnCheckout: false
    asciiOnly: false
    branchFormat: []
```

## Server type

Set `serverType` to connect to Jira Server or Data Center (uses REST API v2 instead of v3).

```yaml
jira:
  serverType: server
```

Values: `cloud` (default), `server`, `datacenter`.

Cloud uses email + API token (Basic auth). Server/Data Center uses a Personal Access Token (Bearer auth), no email needed.

## TLS

For environments that require client certificates (mTLS) or a custom CA:

```yaml
jira:
  tls:
    certFile: /path/to/client.crt
    keyFile:  /path/to/client.key
    caFile:   /path/to/ca.pem
    insecure: false
```

| Field | Description |
|-------|-------------|
| `certFile` | Client certificate PEM |
| `keyFile` | Client private key PEM |
| `caFile` | Custom CA bundle (optional) |
| `insecure` | Skip TLS verification (not recommended) |

All fields are optional. You can use `caFile` alone for custom CA without client certs. Environment variables `JIRA_TLS_CERT`, `JIRA_TLS_KEY`, `JIRA_TLS_CA`, `JIRA_TLS_INSECURE` also work.

If your certificate is in PKCS#12 format (`.p12`/`.pfx`, e.g. exported from Firefox), convert it to PEM first:

```bash
openssl pkcs12 -in cert.p12 -out client.crt -clcerts -nokeys
openssl pkcs12 -in cert.p12 -out client.key -nocerts -nodes
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

`selectCreatedIssue` controls whether the app auto-selects a newly created issue in the list. If the issue does not match the current tab, the app switches to the All tab. Enabled by default.

```yaml
gui:
  selectCreatedIssue: true
```

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

## Info panel fields

See [Custom Fields](Custom_Fields.md) for details on configuring the info panel.

Without any `fields:` config, the info panel shows default fields: status, priority, assignee, reporter, issuetype, sprint (plus labels and components when set on the issue).

To customize which fields appear and in what order, add a `fields:` section. This replaces the defaults entirely.

```yaml
fields:
  - id: status
  - id: priority
  - id: assignee
  - id: "customfield_10015"
    name: "Story Points"
    type: "text"
```

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
