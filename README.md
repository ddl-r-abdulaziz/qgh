# QGH - Quick GitHub

A small CLI tool that prints the GitHub URL of the current git repository, along with your own open pull requests against it.

## Features

- **Origin URL**: Resolves the `origin` remote (ssh, `ssh://`, or https) to its GitHub web URL
- **GitHub Integration**: Lists your open pull requests for the current repository via the GitHub CLI

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

No flags, no arguments. Run it inside a git repository and it prints the GitHub URL followed by your open PRs.

If the current directory is not a git repository, qgh exits with status 1. If the PR lookup fails (for example, the GitHub CLI is not authenticated), the URL is still printed and a warning goes to stderr.

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
