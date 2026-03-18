# lazyjira

Terminal UI for Jira. Like [lazygit](https://github.com/jesseduffield/lazygit) but for Jira.

## Install

### Homebrew

```
brew tap cockroach-eater/tap
brew install lazyjira
```

### Go

```
go install github.com/cockroach-eater/lazyjira/cmd/lazyjira@latest
```

### From source

```
git clone https://github.com/cockroach-eater/lazyjira.git
cd lazyjira
go build -o lazyjira ./cmd/lazyjira
```

## Setup

Run `lazyjira`. On first launch it asks for your Jira host, email, and API token.

Create an API token at https://id.atlassian.com/manage-profile/security/api-tokens

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
