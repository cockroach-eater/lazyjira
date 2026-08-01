# Configuration

lazyjira is configured through a YAML file.

## Config file location

| OS | Path |
|----|------|
| Linux | `~/.config/lazyjira/config.yml` |
| macOS | `~/Library/Application Support/lazyjira/config.yml` |
| Windows | `%AppData%\lazyjira\config.yml` |

You can override the config directory with the `LAZYJIRA_CONFIG_DIR` environment variable. `XDG_CONFIG_HOME` is also respected on Linux.

`CONFIG_DIR` is no longer supported. If you previously used it, rename it to `LAZYJIRA_CONFIG_DIR`.

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
    themeColors: {}   # optional palette overrides (see Themes section)
    themeDark: {}     # applied on dark presets only
    themeLight: {}    # applied on light presets only
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
    typeIcons:
        Bug: "🐞"
        Story: "📖"
        Sub-task: "📎"
    statusIcons:
        To do: "📋"
        On hold: "⏸️"
        Future: "🔜"
    priorityIcons:
        Highest: "⇈"
        High: "↑"
        Medium: "→"
        Low: "↓"
        Lowest: "⇊"

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

`theme` selects the color palette. Supported values: `default` (ANSI 16, original look), `auto`, [Catppuccin](https://github.com/catppuccin/catppuccin) presets (`catppuccin-latte`, `catppuccin-frappe`, `catppuccin-macchiato`, `catppuccin-mocha`). Omitting the `theme` key or setting it to `""` keeps the legacy `default` ANSI 16 palette for backward compatibility. Use `theme: auto` to opt into runtime detection: lazyjira inspects your terminal background and picks `catppuccin-mocha` (dark) or `catppuccin-latte` (light). An unknown theme name is an error. Hex-based themes require a terminal with truecolor support.

```yaml
gui:
  theme: catppuccin-mocha
```

### Themes

#### Built-in presets

| Preset                 | Variant | Notes                                |
|------------------------|---------|--------------------------------------|
| `default`              | dark    | ANSI 16 palette — inherits your terminal colors |
| `catppuccin-latte`     | light   | Catppuccin Latte                     |
| `catppuccin-frappe`    | dark    | Catppuccin Frappé (warmest dark)     |
| `catppuccin-macchiato` | dark    | Catppuccin Macchiato (medium)        |
| `catppuccin-mocha`     | dark    | Catppuccin Mocha (deepest dark)      |

#### Per-color overrides

Three optional maps let you override individual palette entries on top of any preset:

- **`themeColors`** — applied to every preset.
- **`themeDark`** — applied only when the active preset is a dark variant.
- **`themeLight`** — applied only when the active preset is a light variant.

The precedence is preset → `themeColors` → (`themeDark` or `themeLight`). Empty values and unknown keys are ignored, so configs stay forward-compatible.

Supported keys:

| Key         | Role                                                  |
|-------------|-------------------------------------------------------|
| `green`     | Primary accent — titles, active borders, success     |
| `blue`      | Secondary accent                                      |
| `red`       | Errors, high priority                                 |
| `yellow`    | Warnings, in-progress, medium priority                |
| `cyan`      | Search mode indicators                                |
| `magenta`   | JQL keywords                                          |
| `orange`    | Names, metadata                                       |
| `white`     | Foreground text — use a **dark** color on light themes |
| `gray`      | Subtitles, hint bars                                  |
| `highlight` | Selection / cursor background                         |

Values are any lipgloss-accepted color: hex (`"#a6e3a1"`), ANSI 16 (`"4"`), or ANSI 256 (`"208"`).

If a value is not a recognized color format, that key falls back to your terminal's default color and a warning is logged (visible with `--debug`).

#### Example: Rose Pine

Auto-detect between Main (dark) and Dawn (light). Palette from [Rose Pine](https://rosepinetheme.com) — All natural pine, faux fur and a bit of soho vibes for the classy minimalist.

```yaml
gui:
  theme: auto

  themeDark:
    white:     "#e0def4"   # Text
    gray:      "#6e6a86"   # Muted
    green:     "#31748f"   # Pine
    blue:      "#9ccfd8"   # Foam
    cyan:      "#9ccfd8"
    red:       "#eb6f92"   # Love
    yellow:    "#f6c177"   # Gold
    magenta:   "#c4a7e7"   # Iris
    orange:    "#ebbcba"   # Rose
    highlight: "#403d52"

  themeLight:
    white:     "#575279"
    gray:      "#9893a5"
    green:     "#286983"
    blue:      "#56949f"
    cyan:      "#56949f"
    red:       "#b4637a"
    yellow:    "#ea9d34"
    magenta:   "#907aa9"
    orange:    "#d7827e"
    highlight: "#dfdad9"
```

#### Example: tweak a Catppuccin preset

Keep Mocha but swap the accent to a softer green and brighten the selection background:

```yaml
gui:
  theme: catppuccin-mocha
  themeColors:
    green:     "#94e2d5"
    highlight: "#45475a"
```

`selectCreatedIssue` controls whether the app auto-selects a newly created issue in the list. If the issue does not match the current tab, the app switches to the All tab. Enabled by default.

```yaml
gui:
  selectCreatedIssue: true
```

`typeIcons` will have issue `type` names replaced by the emojis you set. Mappings are case-sensitive, and support not only emojis but also plaintext. Enable the field `type` under `issueListFields` to profit from this option.

```
gui:
    typeIcons:
        Bug: "🐞"
        Story: "📖"
        Sub-task: "📎"
```

`statusIcons` will have issue `status` indicators replaced by the emojis you set. Default indicators may be shared for similar states (e.g. To do / Future), so one may get more granularity from this option. Mappings are case-sensitive, and support not only emojis but also plaintext. Enable the field `status` under `issueListFields` to profit from this option.

```
gui:
    statusIcons:
        To do: "📋"
        On hold: "⏸️"
        Future: "🔜"
```

`priorityIcons` will have issue `priority` names replaced by the emojis you set. Mappings are case-sensitive, and support not only emojis but also plaintext. Unmapped priorities fall back to the plain priority name. Enable the field `priority` under `issueListFields` to profit from this option.

```
gui:
    priorityIcons:
        Highest: "⇈"
        High: "↑"
        Medium: "→"
        Low: "↓"
        Lowest: "⇊"
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

## Managed tabs (`saved_tabs.yml`)

Tabs created from inside the TUI are stored in a separate file, `saved_tabs.yml`, in the same config directory as `config.yml` (so `~/.config/lazyjira/saved_tabs.yml` on Linux, and honoring `LAZYJIRA_CONFIG_DIR` / `XDG_CONFIG_HOME`). This keeps `config.yml` untouched: lazyjira never rewrites it, so your hand-edited comments and formatting are safe.

There are two sources of tabs:

| Source | File | Edited via | Templates |
|--------|------|------------|-----------|
| Config tabs | `config.yml` `issueTabs` | hand-edited (read-only from the TUI) | yes (`{{.ProjectKey}}`) |
| Managed tabs | `saved_tabs.yml` | created, deleted, reordered in the TUI | no (concrete JQL) |

You normally do not edit `saved_tabs.yml` by hand. It is written by the Save, Delete, Promote, and Reorder actions (see [Keybindings](Keybindings.md#issues)). The format is an ordered list, and the list order is the tab order shown in the bar:

```yaml
tabs:
  - name: "My Bugs"
    jql: "project = ABC AND type = Bug AND statusCategory != Done"
    project: "ABC"      # bind to one project; empty = global (shown under every project)
    maxResults: 50      # optional, same meaning as on issueTabs
```

| Field | Description |
|-------|-------------|
| `name` | Tab label. |
| `jql` | Concrete JQL (no template expansion). |
| `project` | Project key the tab belongs to. Empty means global: the tab appears under every project. |
| `maxResults` | Optional per-tab page size, same resolution as for `issueTabs`. |

### Tab order

For the active project the bar is assembled in this order:

1. Managed tabs whose `project` matches the current project or is empty (global), in `saved_tabs.yml` order.
2. Config tabs from `issueTabs`, except those shadowed by a managed tab (see below).
3. Transient tabs (ad-hoc JQL search, hierarchy view) created at runtime.

### Shadow, promote, un-promote

A managed tab and a config tab can share a name. When they do, the managed tab **shadows** the config one: the config tab is hidden while a same-named managed tab exists. Matching is by name.

This is what the Promote action uses. Promoting a config tab copies it (name, JQL, `maxResults`) into `saved_tabs.yml`, bound to the current project. The config original is then shadowed, but `config.yml` is never modified, so promote is non-destructive and reversible. Deleting the managed copy **un-promotes** it: the config tab reappears unchanged.

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

## Markdown conversion (`converter`)

Controls how Jira's ADF (Atlassian Document Format) is converted to and from Markdown for the editor. Affects both issue descriptions and comments when you open them in your `$EDITOR`.

```yaml
converter: adf-converter
```

| Value | Behavior |
|-------|----------|
| *(unset, default)* | Built-in converter. Stateless. |
| `builtin` | Same as the default; explicit form. |
| `adf-converter` | External [adf-converter](https://github.com/seflue/adf-converter) library. Supports the common Jira ADF element set as editable Markdown. Inline media, internal attachments, and unknown node types fall back to placeholders so the roundtrip stays lossless. |

Any other value causes lazyjira to exit on startup with an error naming the invalid setting.

## ADF preview rendering (`renderer`)

Controls how ADF documents are rendered to styled terminal lines in the description preview (create form and detail view).

```yaml
renderer: glamour
```

| Value | Behavior |
|-------|----------|
| *(unset, default)* | Built-in renderer. Hand-rolled ADF traversal with chroma syntax highlighting. |
| `builtin` | Same as the default; explicit form. |
| `glamour` | Routes ADF through [adf-converter](https://github.com/seflue/adf-converter)'s display module, which produces Markdown and renders it through Glamour. Richer styling for headings, lists, code blocks, and colored text spans. |

Any other value causes lazyjira to exit on startup with an error naming the invalid setting.

## ADF preview style (`rendererStyle`)

Selects the Glamour theme used by the `glamour` renderer. Ignored when `renderer` is unset or `builtin`.

```yaml
rendererStyle: auto
```

| Value | Behavior |
|-------|----------|
| *(unset, default)* | Same as `auto`. |
| `auto` | Picks `dark` or `light` based on `lipgloss.HasDarkBackground`. Terminal background detection can fail under `tmux` or `ssh`; set the value explicitly when autodetect picks wrong. |
| `dark` | Forces the dark Glamour theme. |
| `light` | Forces the light Glamour theme. |
| `notty` | Plain output without ANSI styling. |

Any other value causes lazyjira to exit on startup with an error naming the invalid setting.

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
| `saved_tabs.yml` | Managed tabs created from inside the TUI |
| `auth.json` | Credentials, created automatically with restricted permissions |
| `jql_history.txt` | JQL search history, up to 50 entries |
