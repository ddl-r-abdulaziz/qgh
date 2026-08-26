package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type GitRepo struct {
	Directory string
	Origin    string
	GitHubURL string
}

type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "qgh: error getting working directory: %v\n", err)
		os.Exit(1)
	}

	if !isGitRepository(cwd) {
		fmt.Fprintf(os.Stderr, "qgh: not a git repository: %s\n", cwd)
		os.Exit(1)
	}

	repo, err := getCurrentRepoInfo(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "qgh: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(repo.GitHubURL)

	if repo.GitHubURL == "N/A" || repo.GitHubURL == "Non-GitHub" {
		fmt.Println("Not a GitHub repository; skipping PR lookup.")
		return
	}

	prs, err := getRepositoryPRs(repo.GitHubURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "qgh: could not fetch PRs: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'gh auth login' to enable PR listing.")
		return
	}

	if len(prs) == 0 {
		fmt.Println("No open PRs authored by you.")
		return
	}
	fmt.Println("\nYour open PRs:")
	for _, pr := range prs {
		fmt.Printf("  #%d  %s\n  %s\n", pr.Number, pr.Title, pr.URL)
	}
}

func isGitRepository(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	if stat, err := os.Stat(gitDir); err == nil {
		return stat.IsDir() || stat.Mode().IsRegular() // .git can be a file (worktree/submodule)
	}
	return false
}

func getCurrentRepoInfo(dir string) (*GitRepo, error) {
	if !isGitRepository(dir) {
		return nil, fmt.Errorf("not a git repository")
	}

	origin, err := getOriginRemote(dir)
	if err != nil {
		origin = "N/A"
	}

	githubURL := convertToGitHubURL(origin)

	return &GitRepo{
		Directory: dir,
		Origin:    origin,
		GitHubURL: githubURL,
	}, nil
}

func getOriginRemote(repoDir string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func convertToGitHubURL(origin string) string {
	if origin == "N/A" || origin == "" {
		return "N/A"
	}

	sshRegex := regexp.MustCompile(`^(?:ssh://)?git@github\.com[:/](.+)/(.+?)(?:\.git)?$`)
	httpsRegex := regexp.MustCompile(`^https://github\.com/(.+)/(.+?)(?:\.git)?$`)

	if matches := sshRegex.FindStringSubmatch(origin); len(matches) == 3 {
		return fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2])
	}

	if matches := httpsRegex.FindStringSubmatch(origin); len(matches) == 3 {
		return fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2])
	}

	if strings.Contains(origin, "github.com") {
		return origin
	}

	return "Non-GitHub"
}

func checkGitHubAuth() bool {
	cmd := exec.Command("gh", "auth", "status")
	err := cmd.Run()
	return err == nil
}

func getRepositoryPRs(repoURL string) ([]PR, error) {
	if repoURL == "N/A" || repoURL == "Non-GitHub" {
		return nil, fmt.Errorf("not a GitHub repository")
	}
	if !checkGitHubAuth() {
		return nil, fmt.Errorf("GitHub CLI not authenticated")
	}

	re := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)`)
	matches := re.FindStringSubmatch(repoURL)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid GitHub URL format")
	}
	owner, repo := matches[1], matches[2]

	cmd := exec.Command("gh", "pr", "list", "--repo", fmt.Sprintf("%s/%s", owner, repo),
		"--author", "@me", "--json", "number,title,url")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PRs: %w", err)
	}

	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}
	return prs, nil
}
