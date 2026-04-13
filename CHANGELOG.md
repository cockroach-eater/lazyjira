# Changelog

All notable changes to this project will be documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/textfuel/lazyjira/compare/v2.8.2...HEAD
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
