# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project (`github.com/ddl-r-abdulaziz/gh`) using Go 1.24.3. The repository is currently minimal with only a `go.mod` file.

## Common Commands

### Development
- `go run .` - Run the gh CLI application
- `make` - Build the gh executable to ./build/gh
- `make clean` - Remove build directory
- `./build/gh` - Run the built application to find git repositories
- `go test ./...` - Run all tests
- `go mod tidy` - Clean up module dependencies
- `go fmt ./...` - Format all Go files

### Testing
- `go test` - Run tests in current directory
- `go test -v` - Run tests with verbose output
- `go test -run TestName` - Run specific test

## Architecture

This is a CLI application that helps enumerate git repositories in subdirectories. The main functionality is contained in `main.go` with the following key components:

- **GitRepo struct**: Represents a git repository with directory path, origin remote, and GitHub URL
- **findGitRepositories()**: Walks through subdirectories to find .git folders
- **getOriginRemote()**: Executes git commands to get the origin remote URL
- **convertToGitHubURL()**: Converts various git remote formats to GitHub URLs
- **printRepositories()**: Formats output in a tabular format using tabwriter