# Keybindings

Press `?` inside lazyjira to see all available keys. Use `/` inside the help popup to filter keybindings.

All keybindings can be remapped in `config.yml` under the `keybinding` section.

## Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `g` / `G` | Jump to top / bottom |
| `ctrl+d` / `ctrl+u` | Half page down / up |
| `J` / `K` | Scroll detail panel (from issues or info panel) |
| `ctrl+f` / `ctrl+b` | Half page detail panel (from issues or info panel) |
| `tab` | Switch panel |
| `h` / `left` | Focus left panel |
| `l` / `right` | Focus right panel |
| `0` `1` `2` `3` `4` | Focus Detail, Status, Issues, Info, Projects panel |

Navigation keys (`j`/`k`/`g`/`G`/`ctrl+d`/`ctrl+u`) can be remapped via `keybinding.navigation` in config.yml.

Detail scroll keys (`J`/`K`/`ctrl+f`/`ctrl+b`) can be remapped via `keybinding.detail` in config.yml.

## Issues

| Key | Action |
|-----|--------|
| `enter` / `space` | Open issue detail |
| `t` | Transition status |
| `e` | Edit (summary, description, or focused field) |
| `p` | Change priority |
| `a` | Change assignee |
| `n` | Create new issue (issues list) or new comment (comments tab) |
| `ctrl+n` | Duplicate issue |
| `S` | Create a subtask under the selected issue (issues list or the info panel's Sub tab). Parent and project are taken from the selection; not available when the selection is itself a subtask or an epic. |
| `c` | View comments |
| `o` | Open in browser |
| `u` | Pick URL from description |
| `y` | Copy issue URL |
| `b` | Create branch from issue |
| `s` | JQL search |
| `x` | Close JQL tab |

## Kanban board

The board of the active project fills the detail panel whenever no task is
open. Selecting a project shows it, `enter` on a card opens that task, and
`esc` brings the board back.

A project can have several agile boards. Each is shown with its own columns
and its own issues, exactly as configured in Jira, and they appear as tabs in
the panel title with the active one in brackets. A project with no agile board
falls back to one column per project status, filled from the active issue tab.

The board mirrors whatever the issue list is showing: switching to a narrower
tab such as **Assigned**, or typing a `/` filter, narrows the board with it. A
board card the list is not showing drops off the board.

| Key | Action |
| --- | --- |
| `[` / `]` | Previous / next board of the project |
| `h` `j` `k` `l` | Move between cards and columns |
| `enter` / `space` | Open the highlighted task |
| `f` | Filter the board by assignee (works from any panel while the board is up) |
| `esc` | Back to the left panels |

In the assignee filter, `enter` applies the highlighted person; `space` ticks
several before confirming. The first row, **(Everyone)**, clears the filter.
The choice is remembered per project.

## Help popup

| Key | Action |
|-----|--------|
| `/` | Filter keybindings |
| `j` / `k` | Navigate up / down |
| `g` / `G` | Jump to top / bottom |
| `esc` | Clear filter or close |
| `q` / `?` | Close |

## General

| Key | Action |
|-----|--------|
| `?` | Help |
| `/` | Search |
| `r` | Refresh current view |
| `R` | Refresh all data |
| `[` / `]` | Previous / next tab |
| `q` / `ctrl+c` | Quit |
