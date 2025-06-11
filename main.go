package main

import (
	"bufio"
	"flag"
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
	skipIgnore := flag.Bool("skip-ignore", false, "Skip .gitignore files and traverse all directories")
	flag.Parse()

	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	repos, err := findGitRepositories(workingDir, *skipIgnore)
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

func findGitRepositories(rootDir string, skipIgnore bool) ([]GitRepo, error) {
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

		if !skipIgnore && info.IsDir() {
			if shouldSkipDirectory(path) {
				return filepath.SkipDir
			}
		}

		return nil
	})

	return repos, err
}

func shouldSkipDirectory(dirPath string) bool {
	parentDir := filepath.Dir(dirPath)
	gitignorePath := filepath.Join(parentDir, ".gitignore")
	
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return false
	}

	file, err := os.Open(gitignorePath)
	if err != nil {
		return false
	}
	defer file.Close()

	dirName := filepath.Base(dirPath)
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		if strings.HasSuffix(line, "/") {
			line = strings.TrimSuffix(line, "/")
		}
		
		if line == dirName || line == "*" {
			return true
		}
		
		matched, err := filepath.Match(line, dirName)
		if err == nil && matched {
			return true
		}
	}
	
	return false
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

func calculateMinimalPaths(repos []GitRepo) []string {
	if len(repos) == 0 {
		return []string{}
	}

	// Convert all paths to relative and split into components
	paths := make([][]string, len(repos))
	for i, repo := range repos {
		relPath, err := filepath.Rel(".", repo.Directory)
		if err != nil {
			relPath = repo.Directory
		}
		paths[i] = strings.Split(relPath, string(filepath.Separator))
	}

	// Find common prefix length
	commonPrefixLen := findCommonPrefix(paths)

	// Remove common prefix from all paths
	result := make([]string, len(repos))
	for i, path := range paths {
		if commonPrefixLen >= len(path) {
			// If common prefix is entire path, use the last component
			result[i] = path[len(path)-1]
		} else {
			result[i] = strings.Join(path[commonPrefixLen:], string(filepath.Separator))
		}
	}

	return result
}

func findCommonPrefix(paths [][]string) int {
	if len(paths) == 0 {
		return 0
	}

	// Find minimum path length
	minLen := len(paths[0])
	for _, path := range paths {
		if len(path) < minLen {
			minLen = len(path)
		}
	}

	// Find common prefix length
	commonLen := 0
	for i := 0; i < minLen; i++ {
		first := paths[0][i]
		allMatch := true
		for _, path := range paths[1:] {
			if path[i] != first {
				allMatch = false
				break
			}
		}
		if allMatch {
			commonLen++
		} else {
			break
		}
	}

	return commonLen
}

func printRepositories(repos []GitRepo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "DIRECTORY\tGITHUB URL")
	fmt.Fprintln(w, "---------\t----------")

	// Calculate minimal distinguishing paths
	minPaths := calculateMinimalPaths(repos)

	for i, repo := range repos {
		fmt.Fprintf(w, "%s\t%s\n", minPaths[i], repo.GitHubURL)
	}
}