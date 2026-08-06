# Changelog

All notable changes to this project will be documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Kanban boards. They fill the detail panel whenever no task is open: selecting a project shows one, `enter` on a card opens that task, and `esc` brings the board back. A project's agile boards each appear with their own columns and their own issues, as configured in Jira -- including columns that collect several statuses -- and `[`/`]` switches between them. A project with no agile board falls back to one column per project status. The board mirrors what the issue list is showing, so a narrower tab such as Assigned or a `/` filter narrows the board with it. `f` filters by assignee on top of that, with the current user and an unassigned bucket pinned on top; the choice is remembered per project

## [2.19.2] - 2026-07-08

### Fixed

- Nix: importing the package outside a flake now works, and cross compilation is possible. `nix/package.nix` used to import `nixpkgs` without a `system`, which broke every manual import. It now takes `lib` and `buildGoApplication` as inputs instead of importing `nixpkgs` itself, and a new `nix/build-go-application.nix` resolves `buildGoApplication` from the pinned `flake.lock` for non flake users (#103)

## [2.19.1] - 2026-07-02

### Added

- Nix: Create a standalone `lazyjira` package and shell for non flake user (#96)

### Fixed

- Confirming a search in the issue list now moves the preview and the current issue to the ticket the cursor lands on, routing the selection through the same path as normal navigation. Actions taken right after a search target the searched ticket instead of the one that was selected before the search (#104)

## [2.19.0] - 2026-06-22

### Added

- `@Name` in a comment or description now resolves to a real Jira mention on submit when exactly one user of the issue's project matches that name. Other names stay as literal text, code spans and fenced blocks are left untouched, and existing mentions are not rewritten. This is Cloud only, Data Center writes are unchanged. When the project's user cache is still cold the write is deferred one tick to fetch users, falling back to literal text on error (#95)

## [2.18.0] - 2026-06-19

### Added

- Color themes with optional auto-detection. `gui.theme` selects a bundled palette: the original `default` ANSI 16 look, or the Catppuccin presets `catppuccin-latte` (light), `catppuccin-frappe`, `catppuccin-macchiato` and `catppuccin-mocha` (dark). `theme: auto` inspects the terminal background at startup and picks `catppuccin-mocha` or `catppuccin-latte` to match. Omitting the key or leaving it empty keeps the legacy ANSI 16 palette, so existing configs are unchanged. An unknown name is an error, and hex-based presets need a truecolor terminal (#88)
- Per-color palette overrides on top of any preset, through three optional maps. `themeColors` applies to every preset, while `themeDark` and `themeLight` apply only on dark or light variants, so a single `auto` config can carry both. Precedence is preset, then `themeColors`, then `themeDark` or `themeLight`. Ten keys are overridable (`green`, `blue`, `red`, `yellow`, `cyan`, `magenta`, `orange`, `white`, `gray`, `highlight`) and accept hex, ANSI 16 or ANSI 256 values. Unknown keys and empty values are ignored, and an unrecognized color falls back to the terminal default with a warning shown under `--debug` (#88)

## [2.17.0] - 2026-06-15

### Added

- Create subtasks under the selected issue, from the task list and the Info panel's Sub tab. The parent is set automatically and the type picker is limited to subtask types. The action appears only where a subtask is valid, so epics and existing subtasks are excluded. The key is configurable like the other issue actions (#94)

### Changed

- The create form now names which required fields are still empty on submit, instead of only counting them, so it is clear what to fill in before the request goes out (#94)
- Reporter is now pre-filled with the current user when creating an issue or subtask, matching Jira's own create defaults. It can still be changed or cleared (#94)
- Create errors now split into pre-form and submit stages. A failed fetch of the parent's issue types aborts with a readable message instead of opening an empty form, while submit errors stay inline. Messages carry Jira's own error text instead of an internal request wrapper (#94)

## [2.16.1] - 2026-06-11

### Changed

- Full unit-test suite across the whole repository, raising coverage from about 50% to about 80% (stdlib `testing`, table-driven cases, race detector). New test linters are enforced in `.golangci.yml` (thelper, tparallel, usetesting, testableexamples, paralleltest) (#92)
- CI runs the test suite with coverage and publishes the total percent to the workflow run summary

### Fixed

- Data race in the author color cache. The shared color map was written without a lock, so rendering many colored author names at once (comments and history) could hit a concurrent map write and crash. Lookups are now guarded by a read-write mutex with double-checked locking (#92)

## [2.16.0] - 2026-06-09

### Added

- Optional Glamour renderer for ADF previews in the detail view and the create form. Set `renderer: glamour` in config.yml to route descriptions and comments through the adf-converter display module and Glamour for richer styling of headings, lists, code blocks and colored text spans. The builtin renderer stays the default. `rendererStyle` (auto, dark, light, notty) selects the Glamour theme. Auto detects the terminal background via lipgloss and can be set explicitly when detection fails under tmux or ssh (#80)
- `--debug <file>` flag that writes structured debug logs to a file for troubleshooting editor launches and other actions (#82)

### Fixed

- Editors configured with arguments in `$EDITOR` or `$VISUAL` now launch correctly. The value is split shell-style into a binary and its arguments, so `EDITOR="code --wait"` or `EDITOR="nvim --cmd 'set ft=md'"` work instead of trying to exec a binary named after the whole string (#82)

## [2.15.0] - 2026-05-21

### Added

- Pluggable ADF-to-Markdown converter. The roundtrip now goes through an `ADFConverter` interface with two implementations: the builtin converter (default and fallback) and `adf-converter` v0.1.0. Set `converter: adf-converter` in config.yml to enable the placeholder-based converter for lossless preservation of panels, tables, status badges and other complex elements. The active converter flows through the full edit lifecycle, including the create-form description preview (#74)

### Changed

- CI now runs Dependabot updates, `govulncheck` vulnerability scanning, GitHub dependency review and a `make check-demo` build check. The `golangci-lint` version is pinned via go tool directives (#76)

## [2.14.0] - 2026-05-15

### Added

- Custom icons for issue priorities in the list view. Configure via `gui.priorityIcons` map in config.yml. Plain text works too.
- `slugify` template function for custom commands. Use in templates like `git switch -c wip/{{ .Summary | slugify }}` to produce URL- and branch-safe slugs. German umlauts (ä→ae, ß→ss) and other accented letters are transliterated to ASCII (#72)

### Changed

- Branch generation with `git.asciiOnly: true` now transliterates German umlauts (ä→ae, ß→ss) and strips other accents (é→e) instead of dropping non-ASCII characters. Issue types like "Lösung" now produce "loesung" instead of "Lsung" (#72)

## [2.13.0] - 2026-05-05

### Added

- Walk through issue hierarchy from the issues list. Space opens an issue's children, Backspace opens its parent, Esc pops back to the previous view. Each step pushes a snapshot of the panel state so cursor, focus and Info tab are restored on the way back. Children walks also work from the Sub and Lnk tabs in the Info panel. Shares a single ad-hoc tab next to the existing ones, similar to the JQL tab (#68)

### Fixed

- Mouse click and wheel scroll on the issues list now update the active issue. Browser open, copy URL, transition and other actions used to target the previously selected issue because the mouse path skipped the canonical selection update
- Demo build (`make check-demo`) failed to compile after the parent-link children feature landed. The demo client now implements `GetChildren` and returns subtasks of the requested parent

## [2.12.0] - 2026-05-03

### Added

- Custom icons for issue statuses in the list view. Configure via `gui.statusIcons` map in config.yml. Plain text works too (#64)

### Fixed

- Sprint field shows None on Jira instances where the `sprint` alias does not resolve to the real custom field id. The id is now discovered at startup via `/field` and used in both reads and writes. Affects older Server/DC and other instances that map sprint to non-default ids like `customfield_10010` (#48)

## [2.11.1] - 2026-04-30

### Fixed

- Branch modal: names that do not match an existing remote ref now create a local branch. Previously any `/` in the input forced remote tracking, breaking prefixed conventions like `feature/PROJ-1-foo` (#62)

## [2.11.0] - 2026-04-30

### Added

- Catppuccin theme support. Pick from Latte, Frappé, Macchiato or Mocha via `gui.theme` in config. Default ANSI palette stays the same. Catppuccin flavors use hex colors and need a terminal with truecolor support (#59)

### Changed

- CI step names mirror the `make` targets they run. Failed jobs now point at the exact command to run locally
- CONTRIBUTING documents how to refresh `gomod2nix.toml` after a Go dependency change. Both `nix develop -c make nix-deps` and `go install gomod2nix` paths are listed. Skipping the refresh fails the nix CI job with a checksum error (#60)

## [2.10.2] - 2026-04-26

### Changed

- Go module path updated to `github.com/textfuel/lazyjira/v2` to follow Go modules v2+ convention

## [2.10.1] - 2026-04-18

### Fixed

- String shorthand in `projects` list (e.g. `- ORCH` instead of `- key: ORCH`) no longer panics on startup. Both forms can now be mixed in the same list (#53)

## [2.10.0] - 2026-04-18

### Added

- Configurable custom commands. Bind shell commands to keys with Go template access to the focused issue, project or comment. Commands declare which UI contexts they fire in and take precedence over built-in keys. Includes `suspend` flag and `shellescape` template helper (#42)
- `maxResults` option to control how many issues are fetched per query. Can be set globally or per tab in `issueTabs`. Default remains 50 (#45)
- Context-sensitive preview for Sub/Lnk tabs. Moving the cursor in subtasks or links previews that issue in the detail pane. Actions target the previewed issue. Preview resets when leaving the tab (#55)

## [2.9.0] - 2026-04-14

### Added

- Custom icons for issue types in the list view. Configure via `gui.typeIcons` map in config.yml (#38)
- `gui.collapsedPanelHeight` option to set the height of non-focused side panels. Default is 5 lines which was often too small to read the info panel without switching focus (#46)

## [2.8.2] - 2026-04-13

### Fixed

- Issue rows no longer wrap to two lines when the panel is narrow. Extra content is cut off instead
- Updated column now lines up vertically. Summary column was not padded so dates floated left on short summaries

## [2.8.1] - 2026-04-10

## [2.8.0] - 2026-04-10

### Added

- Search and filter in keybindings help popup. Press `/` in the `?` menu to filter by key or description (#15)
- Scroll detail panel without switching focus. `J`/`K` scrolls one line, `ctrl+f`/`ctrl+b` scrolls half page from any left panel (#20)
- Navigation keys (`j`/`k`/`g`/`G`/`ctrl+d`/`ctrl+u`) are now configurable via `keybinding.navigation` in config.yml

## [2.7.4] - 2026-04-10

### Added

- Optimistic UI updates. Fields update on screen instantly without waiting for the API
- All paginated API methods fetch every page. Boards, sprints, changelog and labels were capped at one page
- Custom fields without predefined options fall back to inline input or external editor instead of showing an error
- `SetBuiltinFieldValue` and `PatchIssueFields` helpers for consistent field patching across issue list and info panel

### Fixed

- Overlay stack now renders all visible layers so stacked modals display correctly
- `PatchIssue` syncs all fields including sprint, labels, components and custom fields

## [2.7.3] - 2026-04-10

### Added

- Branch name templates support `{{.ParentKey}}` for subtask workflows like `PROJ-100/PROJ-142_summary` (#34)
- Documented `{{.ProjectKey}}`, `{{.Number}}` and `{{.Type}}` template variables that were available but missing from docs (#34)

### Fixed

- Assignee list was capped at 100 users. Now fetches all pages so large orgs see everyone (#35)

## [2.7.2] - 2026-04-03

### Added

- Edit custom fields in info panel. Supports select, multiselect, person, text and textarea
- Field type auto-detected from Jira value. Can also set `type` in `fields` config
- Fields with explicit `type: text` or `type: textarea` skip the CreateMeta API call
- `multiline` option in `fields` config opens external editor for text fields
- CreateMeta cached per project and issue type so repeated edits skip the API call
- Enter on linked issue opens it in detail view without leaving the info panel
- Space on linked issue navigates to it in the issues list
- Space on issues panel opens issue detail
- Info panel label width adjusts to fit custom field names
- None and Unknown values shown in gray
- Ctrl+J / Ctrl+K as alternative navigation keys everywhere (lists, modals, search)

### Fixed

- Person field edit could update the wrong issue if you changed selection before picking a name
- Auto-refresh no longer jumps detail and info panels when you are focused elsewhere
- Editing a field not available in issue metadata now shows an error instead of an empty input

### Removed

- Issue select feature with star marker and pin-to-top

## [2.7.1] - 2026-04-02

### Changed

- Space key disabled in issues panel. Enter is the only way to open issue detail (#25)
- Enter in projects panel now selects the project, same as space
- Double-click, git branch detection and issue creation no longer mark issue as active

### Fixed

- Detail panel updates when switching issue tabs with `[]`
- Detail panel shows first issue after selecting a project instead of staying on project preview
- Projects list limited to 100 items on Jira Cloud. Now fetches all projects with pagination

## [2.7.0] - 2026-04-01

### Added

- Create issues from TUI (n in issues panel, ctrl+n to duplicate): two-phase overlay with type picker and field form
- Configurable issue tabs with JQL templates and {{.ProjectKey}}/{{.UserEmail}} placeholders
- Create form prefills fields from active tab JQL (e.g. assignee from "Mine" tab)
- Duplicate issue (d key): prefills all fields from the source issue
- Custom field support in create form (select, multiselect, text, number)
- Demo mode: issue creation with in-memory data, create metadata endpoint
- Config: `gui.prefillFromTab` option (default true)
- Auto-select newly created issue in the list. Switches to All tab if the issue does not match the current tab. Config: `gui.selectCreatedIssue` (default true)
- e2e test tape for issue creation flow

### Fixed

- ADF code blocks now wrap long lines instead of breaking the panel border
- ADF headings wrap to fit panel width

## [2.6.8] - 2026-04-01

### Fixed

- AUR PKGBUILD: pkgver() now uses git describe for tag-based versioning per Arch Wiki guidelines

## [2.6.7] - 2026-04-01

### Fixed

- Text wrapping now measures display width of Unicode and emoji instead of counting bytes. Panels no longer overflow with multi-byte characters
- Info panel field values truncated by visual width instead of byte length
- ADF list markers use correct display width for indentation
- Stripped carriage returns from wiki markup and ADF text to prevent terminal corruption with Jira Server line endings

## [2.6.6] - 2026-03-30

### Fixed

- Assignee list: current user now always appears first, matched by account ID instead of email. Fixes cases where Jira Cloud hides emails due to privacy settings (#16)
- Assignee modal now scrolls to keep the cursor visible when navigating long lists
- Selected project now pins to the top of the projects list
- Project keys that are reserved JQL words (like DO, IN, IS) no longer cause search errors

### Added

- Assignable users are cached per project and prefetched in background after project switch

## [2.6.5] - 2026-03-30

### Fixed

- Search backspace now correctly deletes multi-byte Unicode characters instead of producing broken glyphs
- Issues list selection no longer jumps to top after confirming search

## [2.6.4] - 2026-03-30

### Fixed

- Homebrew: switched back from Cask to Formula — Cask quarantines unsigned CLI binaries, causing macOS Gatekeeper to block execution

## [2.6.3] - 2026-03-30

### Fixed

- Homebrew tap: removed stale Formula that shadowed the Cask, causing `brew upgrade` to stay on v2.4.0

## [2.6.2] - 2026-03-30

### Fixed

- Homebrew formula not updating since v2.4.0: switched goreleaser from `homebrew_casks` back to `brews`

## [2.6.1] - 2026-03-29

### Changed

- Nix flake: switched from vendorHash to gomod2nix for reproducible builds
- CI: added nix build check to catch outdated dependency hashes
- CONTRIBUTING: added Nix dev environment section

## [2.6.0] - 2026-03-29

### Added

- Jira Server and Data Center support (REST API v2) with automatic endpoint adaptation
- Client certificate authentication (mTLS): `certFile`, `keyFile`, `caFile`, `insecure` in config
- Setup wizard: choose between Cloud and Server/Data Center, prompts adapt accordingly
- Server/DC uses Bearer PAT auth (no email needed), Cloud keeps Basic auth
- Jira wiki markup rendering: bold, italic, links, headings, code blocks converted to plain text
- Error modal with red border for API errors (previously only shown in status bar)
- Issues list updates immediately after editing summary, status, or assignee
- Config: `serverType` field (`cloud`, `server`, `datacenter`) and TLS settings
- Environment variables: `JIRA_SERVER_TYPE`, `JIRA_TLS_CERT`, `JIRA_TLS_KEY`, `JIRA_TLS_CA`, `JIRA_TLS_INSECURE`
- README: clarified that API token and PAT are the same thing, added Server/DC setup instructions

### Fixed

- Edit Summary: long text now wraps instead of being truncated with "..."
- Edit Summary: space key now works (was silently ignored)
- Edit Summary: cursor at end of full line wraps to next line instead of breaking the border
- Edit Summary: ANSI escape codes no longer split across wrapped lines
- Confirm changes diff view: lines wrap instead of being truncated
- Description editor: opens with content on Server/DC (was empty due to ADF/string mismatch)
- Changelog tab: works on Server/DC (uses `?expand=changelog` instead of separate endpoint)
- Status panel: shows host when email is empty (Server/DC)

## [2.5.1] - 2026-03-28

### Fixed

- Arrow keys (up/down) now work for navigating filtered results during `/` search
- Info panel: cursor stays on the selected element after confirming search with Enter (previously jumped to wrong item)

### Added

- Documentation: [Configuration](docs/Config.md), [Keybindings](docs/Keybindings.md), [Custom Fields](docs/Custom_Fields.md)
- README: documentation links, expanded roadmap

### Changed

- Config: annotated unimplemented options with TODO markers (theme, language, mouse toggle, cache, auto-refresh, etc.)

## [2.5.0] - 2026-03-28

### Added

- Info panel: subtasks, links and fields extracted into a dedicated left panel with three tabs (Info/Lnk/Sub)
- Navigate linked issues and subtasks directly from the info panel (Enter to preview, Space to open)
- Edit fields (priority, assignee, sprint, etc.) right from the info panel with e key
- Sprint management: move issues between sprints via the Agile API (MoveToSprint)
- Info panel has its own keybindings section in help overlay
- Mouse support for the info panel (click, scroll, tab switching)
- Number key 3 focuses info panel, projects moved to 4
- Arrow keys cycle through all four left panels (lazygit style)
- Batch prefetch for issue details

### Changed

- Detail panel tabs simplified: removed Sub, Lnk, Info tabs (moved to info panel)
- Left panel navigation reworked: up/down arrows cycle status/issues/info/projects instead of jumping to detail
- Agile API client refactored: doAgile/doAgileMethod avoid mutating baseURL
- e2e tests consolidated into a single preview tape

## [2.4.3] - 2026-03-27

### Fixed

- Cursor warp on panel switch

## [2.4.2] - 2026-03-25

### Changed

- Release notes now include link to CHANGELOG.md

## [2.4.1] - 2026-03-25

### Added

- CI workflow: golangci-lint + vet + build on PRs and main
- Required status checks on main branch
- GitHub issue templates (bug report, feature request)
- Pull request template
- CONTRIBUTING.md

### Changed

- Homebrew distribution: brews -> homebrew_casks (goreleaser v2)
- Refactored app.go: extracted handlers into handlers_keys, handlers_data, handlers_jql, handlers_modal
- OverlayStack: unified modal intercept/render dispatch for all overlay panels
- DRY helpers for modal, inputmodal, jqlmodal, diffview components
- Unit tests for modal, overlaystack, text utilities

## [2.4.0] - 2026-03-25

### Added

- Git integration: create branches from issues with configurable name templates (b key)
- Git integration: search and checkout existing branches by issue key (B key)
- Branch format rules with conditions by issue type (feat/*, fix/*, fallback)
- Auto-detect current issue from branch name
- CHANGELOG.md

## [2.3.0] - 2026-03-24

### Added

- JQL search modal with two-panel UI (input + suggestions/history) (s key)
- JQL autocomplete: field names and values from Jira API
- JQL syntax highlighting in the search input
- JQL history persistence (plain text file, max 50 entries)
- JQL search results appear as a temporary tab in the issues panel
- Custom readline-style text input with cursor, Home/End, Ctrl+A/E/W/K/U
- `make check` target (lint + vet + build)

## [2.2.0] - 2026-03-21

### Added

- Edit fields: transition, priority, assignee changes from TUI (t/p/a keys)
- Comment viewing and posting (c/n keys)
- Input modal component for text entry
- Diff view component for description change history
- ADF-to-Markdown renderer for rich text display in edit/comment workflows

## [2.1.0] - 2026-03-20

### Added

- Rich ADF (Atlassian Document Format) rendering in issue detail
- Support for mentions, emoji, lists, links, code blocks, inline cards
- Windows installation guide in README

## [1.0.0] - 2026-03-18

### Added

- Panel layout inspired by lazygit: Status, Issues, Projects, Detail
- Jira Cloud REST API v3 integration
- Interactive setup wizard on first launch
- Issue list with All/Assigned tabs
- Issue detail with tabs: Body, Sub, Cmt, Lnk, Info, Hist
- Project switcher with auto-fetch from Jira API
- Transition issues (t key) with modal picker
- URL picker (u key) with in-app navigation for Jira links
- History tab with diff for large field changes
- Author color coding consistent across all views
- Search/filter with / key (per-panel)
- Prefetch and cache all issue details for instant navigation
- Auto-refresh every 30 seconds
- Open in browser (o key), copy URL (y key)
- Mouse support: click panels, scroll, click tabs
- Vertical layout for narrow terminals (< 80 cols)
- Responsive side panel width
- Cross-platform: macOS, Linux, Windows
- Homebrew install via tap

[Unreleased]: https://github.com/textfuel/lazyjira/compare/v2.19.2...HEAD
[2.19.2]: https://github.com/textfuel/lazyjira/compare/v2.19.1...v2.19.2
[2.19.1]: https://github.com/textfuel/lazyjira/compare/v2.19.0...v2.19.1
[2.19.0]: https://github.com/textfuel/lazyjira/compare/v2.18.0...v2.19.0
[2.18.0]: https://github.com/textfuel/lazyjira/compare/v2.17.0...v2.18.0
[2.17.0]: https://github.com/textfuel/lazyjira/compare/v2.16.1...v2.17.0
[2.16.1]: https://github.com/textfuel/lazyjira/compare/v2.16.0...v2.16.1
[2.16.0]: https://github.com/textfuel/lazyjira/compare/v2.15.0...v2.16.0
[2.15.0]: https://github.com/textfuel/lazyjira/compare/v2.14.0...v2.15.0
[2.14.0]: https://github.com/textfuel/lazyjira/compare/v2.13.0...v2.14.0
[2.13.0]: https://github.com/textfuel/lazyjira/compare/v2.12.0...v2.13.0
[2.12.0]: https://github.com/textfuel/lazyjira/compare/v2.11.1...v2.12.0
[2.11.1]: https://github.com/textfuel/lazyjira/compare/v2.11.0...v2.11.1
[2.11.0]: https://github.com/textfuel/lazyjira/compare/v2.10.2...v2.11.0
[2.10.2]: https://github.com/textfuel/lazyjira/compare/v2.10.1...v2.10.2
[2.10.1]: https://github.com/textfuel/lazyjira/compare/v2.10.0...v2.10.1
[2.10.0]: https://github.com/textfuel/lazyjira/compare/v2.9.0...v2.10.0
[2.9.0]: https://github.com/textfuel/lazyjira/compare/v2.8.2...v2.9.0
[2.8.2]: https://github.com/textfuel/lazyjira/compare/v2.8.1...v2.8.2
[2.8.1]: https://github.com/textfuel/lazyjira/compare/v2.8.0...v2.8.1
[2.8.0]: https://github.com/textfuel/lazyjira/compare/v2.7.4...v2.8.0
[2.7.4]: https://github.com/textfuel/lazyjira/compare/v2.7.3...v2.7.4
[2.7.3]: https://github.com/textfuel/lazyjira/compare/v2.7.2...v2.7.3
[2.7.2]: https://github.com/textfuel/lazyjira/compare/v2.7.1...v2.7.2
[2.7.1]: https://github.com/textfuel/lazyjira/compare/v2.7.0...v2.7.1
[2.7.0]: https://github.com/textfuel/lazyjira/compare/v2.6.8...v2.7.0
[2.6.8]: https://github.com/textfuel/lazyjira/compare/v2.6.7...v2.6.8
[2.6.7]: https://github.com/textfuel/lazyjira/compare/v2.6.6...v2.6.7
[2.6.6]: https://github.com/textfuel/lazyjira/compare/v2.6.5...v2.6.6
[2.6.5]: https://github.com/textfuel/lazyjira/compare/v2.6.4...v2.6.5
[2.6.4]: https://github.com/textfuel/lazyjira/compare/v2.6.3...v2.6.4
[2.6.3]: https://github.com/textfuel/lazyjira/compare/v2.6.2...v2.6.3
[2.6.2]: https://github.com/textfuel/lazyjira/compare/v2.6.1...v2.6.2
[2.6.1]: https://github.com/textfuel/lazyjira/compare/v2.6.0...v2.6.1
[2.6.0]: https://github.com/textfuel/lazyjira/compare/v2.5.1...v2.6.0
[2.5.1]: https://github.com/textfuel/lazyjira/compare/v2.5.0...v2.5.1
[2.5.0]: https://github.com/textfuel/lazyjira/compare/v2.4.3...v2.5.0
[2.4.3]: https://github.com/textfuel/lazyjira/compare/v2.4.2...v2.4.3
[2.4.2]: https://github.com/textfuel/lazyjira/compare/v2.4.1...v2.4.2
[2.4.1]: https://github.com/textfuel/lazyjira/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/textfuel/lazyjira/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/textfuel/lazyjira/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/textfuel/lazyjira/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/textfuel/lazyjira/compare/v2.0.3...v2.1.0
[1.0.0]: https://github.com/textfuel/lazyjira/releases/tag/v1.1.0
