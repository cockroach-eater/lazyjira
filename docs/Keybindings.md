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

## Comments

| Key | Action |
| --- | --- |
| `c` | Go to the comments tab |
| `n` | Write a new comment in a popup |
| `ctrl+r` | Reply to the selected comment, prefilled with an @-mention and a quote |
| `e` | Edit the selected comment in `$EDITOR` |

Inside the comment popup: `ctrl+d` sends, `esc` cancels, and `ctrl+e` moves
what you have written to `$EDITOR`.

Jira has no threaded replies, so a reply is an ordinary comment that quotes
and mentions the person being answered.

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
