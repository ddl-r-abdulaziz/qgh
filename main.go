package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
)

type GitRepo struct {
	Directory string
	Origin    string
	GitHubURL string
}

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	repos, err := findGitRepositories(workingDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding git repositories: %v\n", err)
		os.Exit(1)
	}

	if len(repos) == 0 {
		fmt.Println("No git repositories found in subdirectories.")
		return
	}

	printRepositories(repos)
}

func findGitRepositories(rootDir string) ([]GitRepo, error) {
	var repos []GitRepo

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && info.Name() == ".git" {
			repoDir := filepath.Dir(path)
			
			if repoDir == rootDir {
				return filepath.SkipDir
			}

			origin, err := getOriginRemote(repoDir)
			if err != nil {
				origin = "N/A"
			}

			githubURL := convertToGitHubURL(origin)

			repos = append(repos, GitRepo{
				Directory: repoDir,
				Origin:    origin,
				GitHubURL: githubURL,
			})

			return filepath.SkipDir
		}

		return nil
	})

	return repos, err
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

	sshRegex := regexp.MustCompile(`^git@github\.com:(.+)/(.+)\.git$`)
	httpsRegex := regexp.MustCompile(`^https://github\.com/(.+)/(.+)\.git$`)
	httpsNoGitRegex := regexp.MustCompile(`^https://github\.com/(.+)/(.+)$`)

	if matches := sshRegex.FindStringSubmatch(origin); len(matches) == 3 {
		return fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2])
	}

	if matches := httpsRegex.FindStringSubmatch(origin); len(matches) == 3 {
		return fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2])
	}

	if matches := httpsNoGitRegex.FindStringSubmatch(origin); len(matches) == 3 {
		return fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2])
	}

	if strings.Contains(origin, "github.com") {
		return origin
	}

	return "Non-GitHub"
}

func printRepositories(repos []GitRepo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "DIRECTORY\tORIGIN\tGITHUB URL")
	fmt.Fprintln(w, "---------\t------\t----------")

	for _, repo := range repos {
		relativeDir, err := filepath.Rel(".", repo.Directory)
		if err != nil {
			relativeDir = repo.Directory
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\n", relativeDir, repo.Origin, repo.GitHubURL)
	}
}