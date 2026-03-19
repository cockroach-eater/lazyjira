# lazyjira

Terminal UI for Jira. Like [lazygit](https://github.com/jesseduffield/lazygit) but for Jira.

| Wide | Narrow |
|------|--------|
| ![preview](e2e/golden/00_preview.gif) | ![preview vertical](e2e/golden/00_preview_vertical.gif) |

<details>
<summary><h2>Installation</h2></summary>

<details>
<summary><b>macOS</b></summary>

#### Homebrew

```
brew tap textfuel/tap
brew install lazyjira
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

</details>

## Setup

Run `lazyjira`. On first launch it asks for your Jira host, email, and API token.

Create an API token at <https://id.atlassian.com/manage-profile/security/api-tokens>

Credentials saved to `~/.config/lazyjira/auth.json`.

## Usage

```
lazyjira                 # start
lazyjira auth            # re-authenticate
lazyjira logout          # clear credentials
lazyjira --version       # show version
lazyjira --log app.log   # log API requests
lazyjira --dry-run       # read-only mode
```

Press `?` inside the app for all keybindings.

## License

MIT
