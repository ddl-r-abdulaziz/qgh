# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project (`github.com/ddl-r-abdulaziz/qgh`) using Go 1.24.3. QGH (Quick GitHub) is a small CLI: run `qgh` with no flags inside a git repository and it prints the GitHub URL of `origin`, then lists the current user's open pull requests against that repository. This plain-print behavior is the default and must not change. The `-i` flag opts into an interactive Bubble Tea terminal UI for the current repository.

## Common Commands

### Development
- `go run .` - Run the qgh CLI application
- `make` - Show available make targets (self-documenting help)
- `make build` - Build the qgh executable to ./build/qgh (preferred build method)
- `make test` - Run all tests
- `make clean` - Remove build directory
- `./build/qgh` - Run the built application in the current git repository
- `./build/qgh -i` - Launch interactive mode (Bubble Tea terminal UI) for the current repo
- `go mod tidy` - Clean up module dependencies
- `go fmt ./...` - Format all Go files

**Note**: Use `make build` instead of `go build .` for building the application.

### Testing
- `make test` - Run all tests via make
- `go test` - Run tests in current directory
- `go test -v` - Run tests with verbose output
- `go test -run TestName` - Run specific test

## Architecture

QGH is a single-repository CLI tool. All functionality lives in `main.go`:

- **GitRepo struct**: Directory path, origin remote, and GitHub URL of the current repository
- **PR struct**: Pull request number, title, and URL
- **main()**: Resolves the current directory's GitHub URL and prints it, then prints the user's open PRs
- **isGitRepository()**: Checks for a `.git` directory or file in the given directory
- **getCurrentRepoInfo()**: Builds a GitRepo for the given directory
- **getOriginRemote()**: Executes git commands to get the origin remote URL
- **convertToGitHubURL()**: Converts various git remote formats to GitHub URLs
- **checkGitHubAuth()**: Verifies the GitHub CLI is authenticated
- **getRepositoryPRs()**: Lists the current user's open PRs for a repository via `gh pr list --author @me`
- **model / Init() / Update() / View()**: The Bubble Tea loop behind the `-i` flag. It implements single-repo interactive browsing only — not multi-repo scanning, no PR-mode toggle, and no cd/shell integration (that was deliberately dropped and should not be reintroduced). PRs load asynchronously via `loadPRsCmd`; `openURL()` opens the selected URL in the browser.

Exit-code policy: a non-git working directory is fatal (exit 1). A PR-fetch failure is a
warning on stderr with exit 0, since the GitHub URL has already been printed.

Known limitation: `isGitRepository` only checks `<cwd>/.git` and does not walk up to the
repository root.
