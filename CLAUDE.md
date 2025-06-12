# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project (`github.com/ddl-r-abdulaziz/qgh`) using Go 1.24.3. QGH (Quick GitHub) is a CLI application for quickly exploring and managing git repositories with GitHub integration.

## Common Commands

### Development
- `go run .` - Run the qgh CLI application
- `go run . --pr` - Run in PR mode to search through user's GitHub PRs
- `make` - Build the qgh executable to ./build/qgh (preferred build method)
- `make clean` - Remove build directory
- `./build/qgh` - Run the built application to find git repositories
- `./build/qgh --pr` - Run in PR mode
- `go test ./...` - Run all tests
- `go mod tidy` - Clean up module dependencies
- `go fmt ./...` - Format all Go files

**Note**: Use `make` instead of `go build .` for building the application.

### Testing
- `go test` - Run tests in current directory
- `go test -v` - Run tests with verbose output
- `go test -run TestName` - Run specific test

## Architecture

QGH is a CLI application that helps enumerate git repositories in subdirectories with GitHub integration and interactive UI. The main functionality is contained in `main.go` with the following key components:

- **GitRepo struct**: Represents a git repository with directory path, origin remote, GitHub URL, and PR count
- **findGitRepositories()**: Walks through subdirectories to find .git folders
- **getOriginRemote()**: Executes git commands to get the origin remote URL
- **convertToGitHubURL()**: Converts various git remote formats to GitHub URLs
- **loadAllUserPRs()**: Loads all user PRs at startup into an in-memory cache using GitHub CLI
- **PRCache**: In-memory cache structure that stores all user PRs for fast filtering
- **Interactive UI**: Bubble Tea-based terminal UI with search and navigation (↑/↓ arrows, PgUp/PgDn, Enter, Ctrl+D for cd, Ctrl+P to switch modes)
- **PR Mode**: Special mode that searches through user's GitHub PRs and shows matching local repositories
- **printRepositories()**: Formats output in a tabular format using tabwriter for non-interactive mode

## PR Mode

QGH supports two search modes that you can switch between dynamically:

**Local Mode (default):**
- Searches through local repository names and paths
- Immediate filtering without API calls
- Use `Ctrl+P` to switch to PR mode

**PR Mode:**
- The search box searches through your GitHub PRs with partial matching support
- Searches match PR titles and support mnemonic matching
- Only local repositories that have matching GitHub repositories with your PRs are shown
- All PR data is cached at startup for instant search results (no API delays)
- The UI indicates "PR Mode" in the header and shows "PR Search:" in the search box
- Use `Esc` twice (clear search, then exit mode) to return to Local mode

**Mode Switching:**
- Start in PR mode: `./build/qgh --pr`
- Switch to PR mode: Press `Ctrl+P` (clears current search)
- Exit PR mode: Press `Esc` to clear search, then `Esc` again to exit PR mode

### PR Search Features:
- **Partial matching**: Search for "bug" to find PRs with titles like "Fix bug in authentication"
- **Mnemonic matching**: Search for "fb" to match "Fix Bug" or "Frontend Backend"
- **Instant search**: All PRs are cached at startup for immediate filtering without API delays
- **PR name display**: Shows matching PR titles next to repositories (e.g., "myrepo ✓ → Fix authentication bug")
- **Multiple PR indication**: Shows count when multiple PRs match (e.g., "myrepo ✓ → 3 PRs")
- **PR counts**: All repositories show their total PR count from the cache