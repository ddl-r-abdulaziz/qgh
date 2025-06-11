# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project (`github.com/ddl-r-abdulaziz/qgh`) using Go 1.24.3. QGH (Quick GitHub) is a CLI application for quickly exploring and managing git repositories with GitHub integration.

## Common Commands

### Development
- `go run .` - Run the qgh CLI application
- `go run . --pr` - Run in PR mode to search through user's GitHub PRs
- `make` - Build the qgh executable to ./build/qgh
- `make clean` - Remove build directory
- `./build/qgh` - Run the built application to find git repositories
- `./build/qgh --pr` - Run in PR mode
- `go test ./...` - Run all tests
- `go mod tidy` - Clean up module dependencies
- `go fmt ./...` - Format all Go files

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
- **getPRCount()**: Uses GitHub CLI to get open PR count by current user
- **searchUserPRs()**: Searches through user's GitHub PRs using GitHub CLI (PR mode)
- **Interactive UI**: Bubble Tea-based terminal UI with search and navigation
- **PR Mode**: Special mode that searches through user's GitHub PRs and shows matching local repositories
- **printRepositories()**: Formats output in a tabular format using tabwriter for non-interactive mode

## PR Mode

When using the `--pr` flag, QGH switches to PR mode where:
- The search box searches through your GitHub PRs (title and content)
- Only local repositories that have matching GitHub repositories with your PRs are shown
- The UI indicates "PR Mode" in the header and shows "PR Search:" in the search box
- All other interactions remain the same (navigation, details, cd functionality)