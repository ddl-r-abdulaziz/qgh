# QGH - Quick GitHub

A fast, interactive CLI tool for exploring and managing git repositories with GitHub integration.

WARNING: I have no idea what this will do to your github api usage :D

## Features

- **Interactive Terminal UI**: Search and navigate repositories with a responsive interface
- **Mnemonic Search**: Type `oic` to match `operations-istio-cni-helm` using word boundaries
- **GitHub Integration**: Automatically detects GitHub repositories and shows open PR counts
- **Smart Path Display**: Shows minimal distinguishing paths for clean output
- **Browser Integration**: Open GitHub URLs directly from the terminal
- **Gitignore Aware**: Respects .gitignore files by default (skip with --skip-ignore)

## Installation

### Build from Source

```bash
make
```

### Install
```bash
make install # will prompt for sudo password
```

### Shell Integration (for 'c' shortcut)

To enable the 'c' shortcut to change directories, add this function to your shell configuration:

**Bash/Zsh (~/.bashrc or ~/.zshrc):**
```bash
qgh() {
    command qgh "$@"
    if [[ -f /tmp/qgh_cd ]]; then
        cd "$(<"/tmp/qgh_cd")"
        rm /tmp/qgh_cd
    fi
}
```

**Fish (~/.config/fish/functions/qgh.fish):**
```fish
function qgh
    command qgh $argv
    if test -f /tmp/qgh_cd
        cd (cat /tmp/qgh_cd)
        rm /tmp/qgh_cd
    end
end
```

### Prerequisites

- Go 1.24.3 or later
- [GitHub CLI](https://cli.github.com/) (for PR count features)

## Usage

```shell
qgh [search-term]
```

**Examples:**
```shell
qgh                    # Launch with no initial search
qgh redis              # Start with "redis" search
qgh oic                # Start with mnemonic search for "operations-istio-cni"
```

### Options

- `--skip-ignore` - Ignore .gitignore files and traverse all directories

## GitHub Integration

QGH integrates with GitHub CLI to provide:

- **Repository Detection**: Automatically identifies GitHub repositories
- **PR Tracking**: Shows open pull requests by the current user
- **Browser Opening**: Direct links to GitHub repositories

### GitHub CLI Setup

```bash
# Install GitHub CLI
brew install gh  # macOS
# or
sudo apt install gh  # Ubuntu/Debian

# Authenticate
gh auth login
```

## Search Features

### Substring Search
Search for any part of the repository path or GitHub URL:
- `docker` matches `operations-redis-docker`
- `istio` matches `sdlc/operations-istio-cni-helm`

### Mnemonic Search
Type the first letters of words separated by delimiters:
- `oic` matches `operations-istio-cni` 
- `sdlc` matches `sdlc/operations-istio-cni-helm`
- `rdc` matches `redis-docker-compose`

**Word Boundaries Recognized:**
- Hyphens: `my-app-name`
- Underscores: `my_app_name`
- Slashes: `path/to/repo`
- CamelCase: `myAppName`
- Dots: `com.example.app`

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [go-isatty](https://github.com/mattn/go-isatty) - Terminal detection

## License

MIT License - see LICENSE file for details.

## Changelog

### v1.0.0
- Interactive terminal UI with search
- GitHub integration with PR counts
- Mnemonic search capabilities
- Browser integration
- Gitignore awareness