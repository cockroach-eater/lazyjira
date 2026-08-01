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
| `M` | Manage the active tab (see below) |
| `D` | Delete the active managed tab (asks for confirmation) |
| `<` / `>` | Move the active managed tab left / right within the managed section |

### Managed tabs

The `M` key is context-sensitive and is how you turn queries into lasting, named tabs (stored in `saved_tabs.yml`, see [Config](Config.md#managed-tabs-saved_tabsyml)):

- On a transient JQL tab (created with `s`): **Save** it as a managed tab. You are prompted for a name, and the tab is bound to the current project.
- On a config tab (from `issueTabs`): **Promote** a copy into the store. The name is pre-filled. The config original is then shadowed by the managed copy; `config.yml` is left untouched.

`D` deletes the active managed tab after a confirmation. If a same-named config tab was shadowed, it reappears (un-promote). `<` and `>` reorder the active managed tab. All three are no-ops on tabs they do not apply to (for example `D` does nothing on a config tab). `x` still only closes a transient JQL tab.

These keys are remappable under `keybinding.issues` in `config.yml`:

| Action | Config key | Default |
|--------|-----------|---------|
| Manage (save / promote) | `manageTab` | `M` |
| Delete managed tab | `deleteManagedTab` | `D` |
| Reorder left | `reorderTabLeft` | `<` |
| Reorder right | `reorderTabRight` | `>` |

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
