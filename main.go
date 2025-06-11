package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

type GitRepo struct {
	Directory string
	Origin    string
	GitHubURL string
	PRCount   int
}

type model struct {
	repos        []GitRepo
	filteredRepos []GitRepo
	searchInput  string
	cursor       int
	minPaths     []string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filteredRepos)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.filteredRepos) > 0 {
				repo := m.filteredRepos[m.cursor]
				if repo.GitHubURL != "N/A" && repo.GitHubURL != "Non-GitHub" {
					openURL(repo.GitHubURL)
				}
			}
		case "backspace":
			if len(m.searchInput) > 0 {
				m.searchInput = m.searchInput[:len(m.searchInput)-1]
				m.filterRepos()
			}
		default:
			if len(msg.String()) == 1 {
				m.searchInput += msg.String()
				m.filterRepos()
			}
		}
	}
	return m, nil
}

func (m *model) filterRepos() {
	if m.searchInput == "" {
		m.filteredRepos = m.repos
		return
	}
	
	var filtered []GitRepo
	searchLower := strings.ToLower(m.searchInput)
	
	for _, repo := range m.repos {
		dirLower := strings.ToLower(repo.Directory)
		urlLower := strings.ToLower(repo.GitHubURL)
		
		if strings.Contains(dirLower, searchLower) ||
		   strings.Contains(urlLower, searchLower) ||
		   matchesMnemonic(dirLower, searchLower) ||
		   matchesMnemonic(urlLower, searchLower) {
			filtered = append(filtered, repo)
		}
	}
	m.filteredRepos = filtered
	
	if m.cursor >= len(m.filteredRepos) {
		m.cursor = len(m.filteredRepos) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func matchesMnemonic(text, query string) bool {
	if len(query) == 0 {
		return true
	}
	
	words := extractWords(text)
	
	queryIndex := 0
	for _, word := range words {
		if queryIndex >= len(query) {
			break
		}
		
		if len(word) > 0 && strings.ToLower(string(word[0])) == strings.ToLower(string(query[queryIndex])) {
			queryIndex++
		}
	}
	
	return queryIndex == len(query)
}

func extractWords(text string) []string {
	var words []string
	var currentWord strings.Builder
	
	for i, r := range text {
		if isWordBoundary(text, i) {
			if currentWord.Len() > 0 {
				words = append(words, currentWord.String())
				currentWord.Reset()
			}
		}
		
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			currentWord.WriteRune(r)
		}
	}
	
	if currentWord.Len() > 0 {
		words = append(words, currentWord.String())
	}
	
	return words
}

func isWordBoundary(text string, pos int) bool {
	if pos == 0 {
		return true
	}
	
	if pos >= len(text) {
		return false
	}
	
	current := rune(text[pos])
	prev := rune(text[pos-1])
	
	if prev == '-' || prev == '_' || prev == '/' || prev == '\\' || prev == '.' {
		return true
	}
	
	if (prev >= 'a' && prev <= 'z') && (current >= 'A' && current <= 'Z') {
		return true
	}
	
	return false
}

func (m model) View() string {
	var b strings.Builder
	
	searchStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)
	
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230"))
	
	b.WriteString(headerStyle.Render("Git Repository Explorer"))
	b.WriteString("\n\n")
	
	searchBox := fmt.Sprintf("Search: %s", m.searchInput)
	b.WriteString(searchStyle.Render(searchBox))
	b.WriteString("\n\n")
	
	if len(m.filteredRepos) == 0 {
		b.WriteString("No repositories found matching your search.\n")
	} else {
		minPaths := calculateMinimalPaths(m.filteredRepos)
		
		// Find the longest path to determine column width
		maxPathLen := 0
		for _, path := range minPaths {
			if len(path) > maxPathLen {
				maxPathLen = len(path)
			}
		}
		
		githubPillStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("2")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true)
			
		prPillStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("5")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true)
		
		for i, repo := range m.filteredRepos {
			pathColumn := fmt.Sprintf("%-*s", maxPathLen, minPaths[i])
			line := pathColumn
			
			if repo.GitHubURL != "N/A" && repo.GitHubURL != "Non-GitHub" {
				githubPill := githubPillStyle.Render("github")
				line = fmt.Sprintf("%s  %s", line, githubPill)
			}
			
			if repo.PRCount > 0 {
				prPill := prPillStyle.Render(fmt.Sprintf("PR[%d]", repo.PRCount))
				line = fmt.Sprintf("%s  %s", line, prPill)
			}
			
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	
	b.WriteString("\n")
	b.WriteString("Use ↑/↓ or j/k to navigate, Enter to open GitHub URL, q/Esc/Ctrl+C to quit")
	
	return b.String()
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

	if isInteractive() {
		m := model{
			repos:         repos,
			filteredRepos: repos,
			searchInput:   "",
			cursor:        0,
		}
		
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running interactive mode: %v\n", err)
			os.Exit(1)
		}
	} else {
		printRepositories(repos)
	}
}

func isInteractive() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd())
}

func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
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
			prCount := getPRCount(githubURL)

			repos = append(repos, GitRepo{
				Directory: repoDir,
				Origin:    origin,
				GitHubURL: githubURL,
				PRCount:   prCount,
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

	sshRegex := regexp.MustCompile(`^git@github\.com:(.+)/(.+?)(?:\.git)?$`)
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

func getPRCount(repoURL string) int {
	if repoURL == "N/A" || repoURL == "Non-GitHub" {
		return 0
	}

	// Extract owner/repo from GitHub URL
	re := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)`)
	matches := re.FindStringSubmatch(repoURL)
	if len(matches) != 3 {
		return 0
	}

	owner := matches[1]
	repo := matches[2]

	// Get current user
	userCmd := exec.Command("gh", "api", "user", "--jq", ".login")
	userOutput, err := userCmd.Output()
	if err != nil {
		return 0
	}
	currentUser := strings.TrimSpace(string(userOutput))

	// Get PR count for current user
	prCmd := exec.Command("gh", "pr", "list", "--repo", fmt.Sprintf("%s/%s", owner, repo), "--author", currentUser, "--json", "number")
	prOutput, err := prCmd.Output()
	if err != nil {
		return 0
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal(prOutput, &prs); err != nil {
		return 0
	}

	return len(prs)
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

	fmt.Fprintln(w, "DIRECTORY\tGITHUB\tPRS")
	fmt.Fprintln(w, "---------\t------\t---")

	// Calculate minimal distinguishing paths
	minPaths := calculateMinimalPaths(repos)

	for i, repo := range repos {
		githubStatus := "No"
		if repo.GitHubURL != "N/A" && repo.GitHubURL != "Non-GitHub" {
			githubStatus = "Yes"
		}
		
		prStatus := ""
		if repo.PRCount > 0 {
			prStatus = strconv.Itoa(repo.PRCount)
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\n", minPaths[i], githubStatus, prStatus)
	}
}