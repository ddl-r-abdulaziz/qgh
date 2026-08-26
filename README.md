# QGH - Quick GitHub

A small CLI tool that prints the GitHub URL of the current git repository, along with your own open pull requests against it.

## Features

- **Origin URL**: Resolves the `origin` remote (ssh, `ssh://`, or https) to its GitHub web URL
- **GitHub Integration**: Lists your open pull requests for the current repository via the GitHub CLI
- **Interactive Mode** (`-i`): Browse the current repository's URL and your open PRs in a terminal UI and open them in your browser

## Installation

### Build from Source

```bash
make
```

### Install
```bash
make install # will prompt for sudo password
```

### Prerequisites

- Go 1.24.3 or later
- [GitHub CLI](https://cli.github.com/) (for PR listing)

## Usage

```shell
cd /path/to/repo
qgh
```

Run it inside a git repository and it prints the GitHub URL followed by your open PRs.

If the current directory is not a git repository, qgh exits with status 1. If the PR lookup fails (for example, the GitHub CLI is not authenticated), the URL is still printed and a warning goes to stderr.

### Interactive mode

```shell
qgh -i
```

Launch an interactive terminal UI to browse the current repo's URL and your open PRs — arrow keys to navigate, PgUp/PgDn to page, Enter to open the selected item in your browser, Esc/q to quit.

Interactive mode covers the current repository only: there is no multi-repo scanning, no PR-mode toggle, and no shell `cd` integration. It requires a real terminal; with a non-tty stdin it exits with an error.

## GitHub Integration

QGH integrates with GitHub CLI to provide:

- **Repository Detection**: Automatically identifies GitHub repositories
- **PR Tracking**: Shows open pull requests by the current user

### GitHub CLI Setup

```bash
# Install GitHub CLI
brew install gh  # macOS
# or
sudo apt install gh  # Ubuntu/Debian

# Authenticate
gh auth login
```

## License

MIT License - see LICENSE file for details.

## Changelog

### v2.0.0
- Removed the interactive TUI, multi-repo scanning, and PR-mode/search — qgh now only prints the current repository's GitHub URL and your open PRs against it

### v1.0.0
- Interactive terminal UI with search
- GitHub integration with PR counts
- Mnemonic search capabilities
- Browser integration
- Gitignore awareness
