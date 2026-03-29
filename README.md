<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/textfuel/lazyjira" alt="Go"></a>
  <a href="https://github.com/textfuel/lazyjira/releases"><img src="https://img.shields.io/github/v/release/textfuel/lazyjira" alt="Release"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>

# lazyjira

Terminal UI for Jira. Like [lazygit](https://github.com/jesseduffield/lazygit) but for Jira.

Jira's web UI is painfully slow. Changing a ticket status takes multiple clicks, pages take seconds to load, and you spend more time fighting the interface than actually working. lazyjira gives you a fast, keyboard-driven terminal UI so you can browse issues, update statuses, read descriptions and more with minimum latency.

<p>
  <img src="e2e/golden/00_preview.gif" width="67%" alt="preview">&nbsp;<img src="e2e/golden/00_preview_vertical.gif" width="31%" alt="preview vertical">
</p>

### Demo mode

Try without a Jira account (build from source required):

```
make build-demo
./lazyjira --demo
```

## Features

- **JQL search** with autocomplete, syntax highlighting, and persistent history
- **4-panel layout** - issues, projects, detail, status - with vim-style navigation
- **Inline editing** - transitions, priority, assignee, labels, comments, description (`$EDITOR`)
- **Configurable** - custom keybindings, JQL tabs, issue columns, custom fields
- **Adaptive** - side-by-side or stacked layout, mouse support, ANSI 16 colors

## Installation

<details>
<summary><b>macOS</b></summary>

#### Homebrew

```
brew install textfuel/tap/lazyjira
```

</details>

<details>
<summary><b>Linux</b></summary>

#### Arch Linux (AUR)

```
yay -S lazyjira-bin     # prebuilt binary
yay -S lazyjira-git     # build from source
```

#### Nix / NixOS

```
nix run github:textfuel/lazyjira
```

Or add to your flake inputs:

```nix
inputs.lazyjira.url = "github:textfuel/lazyjira";
```

#### deb (Debian, Ubuntu)

Download `.deb` from [Releases](https://github.com/textfuel/lazyjira/releases):

```
sudo dpkg -i lazyjira_*.deb
```

#### rpm (Fedora, RHEL)

Download `.rpm` from [Releases](https://github.com/textfuel/lazyjira/releases):

```
sudo rpm -i lazyjira_*.rpm
```

#### apk (Alpine)

Download `.apk` from [Releases](https://github.com/textfuel/lazyjira/releases):

```
sudo apk add --allow-untrusted lazyjira_*.apk
```

</details>

<details>
<summary><b>Windows</b></summary>

Download `.zip` from [Releases](https://github.com/textfuel/lazyjira/releases), extract `lazyjira.exe`, and add it to your `PATH`.

Use [Windows Terminal](https://aka.ms/terminal) for best rendering.

</details>

<details>
<summary><b>Go / From source</b></summary>

```
go install github.com/textfuel/lazyjira/cmd/lazyjira@latest
```

Or build manually:

```
git clone https://github.com/textfuel/lazyjira.git
cd lazyjira
make build
```

</details>

## Setup

Run `lazyjira`. On first launch the setup wizard asks for your Jira type (Cloud or Server/Data Center), host, and credentials.

### Jira Cloud

Provide your email and an API token (also called Personal Access Token / PAT).

Create one at <https://id.atlassian.com/manage-profile/security/api-tokens>

### Jira Server / Data Center

Provide your Personal Access Token (PAT). No email needed.

Generate a PAT in Jira: Profile > Personal Access Tokens > Create token.

For environments that require client certificates (mTLS), see [Configuration](docs/Config.md#tls).

Credentials saved to `~/.config/lazyjira/auth.json`.

## Usage

```
lazyjira                 # start
lazyjira auth            # re-authenticate
lazyjira logout          # clear credentials
lazyjira --dry-run       # read-only mode (no writes to Jira)
lazyjira --log app.log   # log API requests to file
lazyjira --version       # show version
```

Press `?` inside the app for all keybindings.

## Documentation

- [Configuration](docs/Config.md) - config file, keybindings, issue tabs, custom fields, git integration
- [Keybindings](docs/Keybindings.md) - full list of default keys
- [Custom Fields](docs/Custom_Fields.md) - displaying Jira custom fields

## Roadmap

- [x] Robust JQL search
- [x] Git integration, create branches from issues, open issue from current branch
- [x] Jira Server and Data Center support
- [x] Client certificate authentication (mTLS)
- [ ] Create issues
- [ ] Robust issue type changer (handle subtask/parent unlinking, field validation)
- [ ] Clickable hyperlinks in terminal (OSC 8) for URLs in descriptions and comments
- [ ] Theming, borders, date format, language and other GUI options
- [ ] Mouse support toggle
- [ ] Show icons toggle
- [ ] Cache with configurable TTL
- [ ] Auto-refresh with configurable interval
- [ ] Custom field type handling (select, multiselect, user)
- [ ] Board ID per project
- [ ] Rich text editing, colors, panels, media in ADF descriptions
- [ ] Bulk operations, transition and assign multiple issues at once
- [ ] Notifications, watch for issue updates
- [ ] Offline mode, cached view when network is unavailable

## Star History

<a href="https://www.star-history.com/?repos=textfuel%2Flazyjira&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/image?repos=textfuel/lazyjira&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/image?repos=textfuel/lazyjira&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/image?repos=textfuel/lazyjira&type=date&legend=top-left" />
 </picture>
</a>

## License

MIT
