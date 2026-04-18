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
    collapsedPanelHeight: 5
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
    navigation:
        down: j
        up: k
        top: g
        bottom: G
        halfPageDown: ctrl+d
        halfPageUp: ctrl+u
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
        scrollDown: J
        scrollUp: K
        halfPageDown: ctrl+f
        halfPageUp: ctrl+b
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

`collapsedPanelHeight` sets the height of non-focused left panels in lines (default 5, minimum 3).

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
    maxResults: 100
```

You can also create temporary JQL tabs at runtime with the `s` key.

Per-tab page size can be set via `maxResults` on the tab entry — see [Page size](#page-size-maxresults) below.

## Page size (`maxResults`)

The number of issues fetched per query. Can be set globally and overridden per tab:

```yaml
maxResults: 75          # global default for all tabs and ad-hoc JQL searches
issueTabs:
  - name: "All"
    jql: "project = {{.ProjectKey}} ORDER BY updated DESC"
    maxResults: 200     # per-tab override
  - name: "Assigned"
    jql: "project = {{.ProjectKey}} AND assignee=currentUser()"
                        # inherits global 75
```

Resolution order: per-tab `maxResults` → global `maxResults` → built-in default (50). Values `<= 0` are treated as unset. Note that the Jira server may enforce its own upper bound and silently return fewer issues than requested.

## Keybindings

All keybindings are remappable. See [Keybindings](Keybindings.md) for the full list of defaults.

```yaml
keybinding:
  universal:
    quit: "q"
    help: "?"
    search: "/"
  navigation:
    down: "j"
    up: "k"
    top: "g"
    bottom: "G"
    halfPageDown: "ctrl+d"
    halfPageUp: "ctrl+u"
  issues:
    transition: "t"
    browser: "o"
    createBranch: "b"
```

Only include keys you want to change. Missing keys keep their defaults.

The `navigation` section controls list navigation keys used across all panels. These default to vim-style keys (`j`/`k`/`g`/`G`/`ctrl+d`/`ctrl+u`).

The `detail` section includes keys for scrolling the detail panel from any left panel without switching focus. Defaults: `J`/`K` for line scroll, `ctrl+f`/`ctrl+b` for half-page.

Setting a navigation key replaces all defaults for that action. For example, `down: "n"` means only `n` navigates down, `j`, arrow down and `ctrl+j` will no longer work.

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
        type: "Sub-task"
      template: "{{.ParentKey}}/{{.Key}}_{{.Summary | slugify}}"
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
| `{{.ProjectKey}}` | Project prefix extracted from key (e.g. PROJ) |
| `{{.Number}}` | Issue number extracted from key (e.g. 123) |
| `{{.Summary}}` | Issue summary |
| `{{.Summary \| slugify}}` | Summary as a slug, lowercase with dashes |
| `{{.Type}}` | Issue type name (e.g. Bug, Story, Task) |
| `{{.ParentKey}}` | Parent issue key (empty if no parent) |

## Custom commands

Bind shell commands to keys, with Go template access to the focused issue, project, or comment. Custom bindings take precedence over built-in keys, so they can be used to override any action.

```yaml
customCommands:
  - key: "ctrl+y"
    name: "Copy issue key"
    command: "printf %s {{.Key}} | wl-copy"
    suspend: false
  - key: "ctrl+l"
    name: "Copy comment link"
    command: "printf '%s?focusedCommentId=%s' {{.URL}} {{.CommentID}} | wl-copy"
    contexts: [detail.comments]
    suspend: false
  - key: "ctrl+w"
    name: "Log work"
    command: "jira issue worklog add {{.Key}}"
```

Each command has:

| Field | Description |
|-------|-------------|
| `key` | Key binding. Supports single letters, modifiers (`ctrl+x`, `alt+x`), and special keys (`tab`, `enter`). |
| `name` | Label shown in the help overlay and help bar. |
| `command` | Shell command string. Rendered as a Go template before execution. |
| `contexts` | Optional list of UI contexts the command fires in. Defaults to `[issues, info, detail]`. |
| `suspend` | Optional. `true` (default) hands the terminal to the child process; set `false` for background commands like clipboard copies or notifications. |

### Contexts

A command fires when one of its declared contexts matches the current UI state.

| Context | Active when |
|---------|-------------|
| `issues` | Issues list panel is focused. |
| `info` | Info panel is focused (any sub-tab). |
| `projects` | Projects list panel is focused. |
| `detail` | Right detail panel is showing an issue, on any tab. |
| `detail.comments` | Right detail panel is on the Comments tab with a comment selected. |

When more than one context matches at the same time (`detail` and `detail.comments` in the Comments tab), the more specific context wins.

### Template variables

The fields available to the template depend on the command's contexts.

**Issue scope** (`issues`, `info`, `detail`):

| Variable | Description |
|----------|-------------|
| `{{.Key}}` | Issue key like `PROJ-123`. |
| `{{.ProjectKey}}` | Project prefix extracted from the key. |
| `{{.ParentKey}}` | Parent issue key (empty if no parent). |
| `{{.Summary}}` | Issue summary. |
| `{{.Type}}` | Issue type name. |
| `{{.Status}}` | Status name. |
| `{{.Assignee}}` | Assignee display name. |
| `{{.Priority}}` | Priority name. |
| `{{.URL}}` | Fully qualified issue URL. |

**Project scope** (`projects`):

| Variable | Description |
|----------|-------------|
| `{{.ProjectKey}}` | Project key. |
| `{{.ProjectName}}` | Project name. |

**Comment scope** (`detail.comments`):

Exposes the Issue scope fields above plus the focused comment:

| Variable | Description |
|----------|-------------|
| `{{.CommentID}}` | Comment ID. |
| `{{.CommentAuthor}}` | Comment author display name. |
| `{{.CommentBody}}` | Comment body. |

### Shared fields

Available in every scope:

| Variable | Description |
|----------|-------------|
| `{{.JiraHost}}` | Jira host from the Jira config. |
| `{{.GitBranch}}` | Current git branch, if lazyjira was started in a git repo. |
| `{{.GitRepoPath}}` | Git repository path. |

### Template helpers

| Helper | Description |
|--------|-------------|
| `{{.X \| shellescape}}` | Wraps the value in single quotes with inner quotes escaped. Use for any template value that could contain shell metacharacters. |

## Files

| File | Description |
|------|-------------|
| `config.yml` | Main configuration |
| `auth.json` | Credentials, created automatically with restricted permissions |
| `jql_history.txt` | JQL search history, up to 50 entries |
