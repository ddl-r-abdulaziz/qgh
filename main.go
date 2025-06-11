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

type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type viewState int

const (
	listView viewState = iota
	detailView
)

type model struct {
	repos        []GitRepo
	filteredRepos []GitRepo
	searchInput  string
	cursor       int
	minPaths     []string
	
	// Detail view state
	currentView    viewState
	selectedRepo   *GitRepo
	repoDetails    []PR
	detailCursor   int
	loadingPRs     bool
	prLoadError    string
	
	// Navigation state
	startedInDetailView bool // True if we opened directly in detail view
	
	// Terminal/scrolling state
	terminalHeight int
	scrollOffset   int
	detailScrollOffset int
}

type prLoadedMsg struct {
	prs []PR
	err error
}

type changeDirMsg struct {
	path string
}

func loadPRsCmd(repoURL string) tea.Cmd {
	return func() tea.Msg {
		prs, err := getRepositoryPRs(repoURL)
		return prLoadedMsg{prs: prs, err: err}
	}
}

func changeDirCmd(path string) tea.Cmd {
	return func() tea.Msg {
		return changeDirMsg{path: path}
	}
}

func (m model) Init() tea.Cmd {
	if m.currentView == detailView && m.selectedRepo != nil && m.loadingPRs {
		return loadPRsCmd(m.selectedRepo.GitHubURL)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalHeight = msg.Height
		return m, nil
		
	case prLoadedMsg:
		m.loadingPRs = false
		if msg.err != nil {
			m.prLoadError = msg.err.Error()
		} else {
			m.repoDetails = msg.prs
			m.prLoadError = ""
		}
		return m, nil
		
	case changeDirMsg:
		// Write the directory path to a temp file for the shell to read
		tmpFile := "/tmp/qgh_cd"
		if err := os.WriteFile(tmpFile, []byte(msg.path), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing cd path: %v\n", err)
		}
		return m, tea.Quit
		
	case tea.KeyMsg:
		if m.currentView == listView {
			return m.updateListView(msg)
		} else {
			return m.updateDetailView(msg)
		}
	}
	return m, nil
}

func (m model) updateListView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "c":
		if len(m.filteredRepos) > 0 {
			repo := m.filteredRepos[m.cursor]
			return m, changeDirCmd(repo.Directory)
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			// Scroll up if cursor goes above visible area
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.filteredRepos)-1 {
			m.cursor++
			// Calculate visible area height (terminal height minus header, search, footer)
			// Header(1) + 2 newlines(2) + search box with border(3) + 2 newlines(2) + newline before footer(1) + footer(1) = 10 lines
			visibleHeight := m.terminalHeight - 10
			if visibleHeight < 1 {
				visibleHeight = 1
			}
			// Scroll down if cursor goes below visible area
			if m.cursor >= m.scrollOffset+visibleHeight {
				m.scrollOffset = m.cursor - visibleHeight + 1
			}
		}
	case "pgup":
		// Calculate visible area height for page jumps
		visibleHeight := m.terminalHeight - 10
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		// Jump up by a page
		m.cursor -= visibleHeight
		if m.cursor < 0 {
			m.cursor = 0
		}
		// Update scroll offset to keep cursor visible
		if m.cursor < m.scrollOffset {
			m.scrollOffset = m.cursor
		}
	case "pgdown":
		// Calculate visible area height for page jumps
		visibleHeight := m.terminalHeight - 10
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		// Jump down by a page
		m.cursor += visibleHeight
		if m.cursor >= len(m.filteredRepos) {
			m.cursor = len(m.filteredRepos) - 1
		}
		// Update scroll offset to keep cursor visible
		if m.cursor >= m.scrollOffset+visibleHeight {
			m.scrollOffset = m.cursor - visibleHeight + 1
		}
	case "enter":
		if len(m.filteredRepos) > 0 {
			repo := m.filteredRepos[m.cursor]
			m.selectedRepo = &repo
			m.currentView = detailView
			m.detailCursor = 0
			m.detailScrollOffset = 0
			m.repoDetails = nil
			m.loadingPRs = true
			m.prLoadError = ""
			return m, loadPRsCmd(repo.GitHubURL)
		}
	case "backspace":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
			m.filterRepos()
		}
	default:
		if len(msg.String()) == 1 && msg.String() != "c" {
			m.searchInput += msg.String()
			m.filterRepos()
		}
	}
	return m, nil
}

func (m model) updateDetailView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "c":
		if m.selectedRepo != nil {
			return m, changeDirCmd(m.selectedRepo.Directory)
		}
	case "esc":
		if m.startedInDetailView {
			return m, tea.Quit
		}
		m.currentView = listView
		m.selectedRepo = nil
		m.repoDetails = nil
		m.detailCursor = 0
		m.detailScrollOffset = 0
	case "up", "k":
		if m.detailCursor > 0 {
			m.detailCursor--
			// Scroll up if cursor goes above visible area
			if m.detailCursor < m.detailScrollOffset {
				m.detailScrollOffset = m.detailCursor
			}
		}
	case "down", "j":
		maxItems := 1 // URL field
		if len(m.repoDetails) > 0 {
			maxItems += len(m.repoDetails)
		}
		if m.detailCursor < maxItems-1 {
			m.detailCursor++
			// Calculate visible area height for detail view
			// Header(1) + 2 newlines(2) + Name(1) + 2 newlines(2) + URL(1) + 2 newlines(2) + "Pull Requests:"(1) + newline before footer(1) + footer(1) = 11 lines
			visibleHeight := m.terminalHeight - 11
			if visibleHeight < 1 {
				visibleHeight = 1
			}
			// Scroll down if cursor goes below visible area
			if m.detailCursor >= m.detailScrollOffset+visibleHeight {
				m.detailScrollOffset = m.detailCursor - visibleHeight + 1
			}
		}
	case "pgup":
		// Calculate visible area height for page jumps
		visibleHeight := m.terminalHeight - 11
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		// Jump up by a page
		m.detailCursor -= visibleHeight
		if m.detailCursor < 0 {
			m.detailCursor = 0
		}
		// Update scroll offset to keep cursor visible
		if m.detailCursor < m.detailScrollOffset {
			m.detailScrollOffset = m.detailCursor
		}
	case "pgdown":
		// Calculate visible area height for page jumps
		visibleHeight := m.terminalHeight - 11
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		// Calculate max items (URL field + PRs)
		maxItems := 1 // URL field
		if len(m.repoDetails) > 0 {
			maxItems += len(m.repoDetails)
		}
		// Jump down by a page
		m.detailCursor += visibleHeight
		if m.detailCursor >= maxItems {
			m.detailCursor = maxItems - 1
		}
		// Update scroll offset to keep cursor visible
		if m.detailCursor >= m.detailScrollOffset+visibleHeight {
			m.detailScrollOffset = m.detailCursor - visibleHeight + 1
		}
	case "enter":
		if m.selectedRepo != nil {
			if m.detailCursor == 0 {
				// Open repository URL
				if m.selectedRepo.GitHubURL != "N/A" && m.selectedRepo.GitHubURL != "Non-GitHub" {
					openURL(m.selectedRepo.GitHubURL)
				}
			} else if len(m.repoDetails) > 0 && m.detailCursor-1 < len(m.repoDetails) {
				// Open PR URL
				pr := m.repoDetails[m.detailCursor-1]
				openURL(pr.URL)
			}
		}
	}
	return m, nil
}

func (m *model) filterRepos() {
	if m.searchInput == "" {
		m.filteredRepos = m.repos
	} else {
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
	}
	
	// Reset cursor and scroll position
	if m.cursor >= len(m.filteredRepos) {
		m.cursor = len(m.filteredRepos) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.scrollOffset = 0
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
	if m.currentView == listView {
		return m.renderListView()
	} else {
		return m.renderDetailView()
	}
}

func (m model) renderListView() string {
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
		
		githubCheckStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true)
		
		// Calculate visible area height (terminal height minus header, search, footer)
		// Header(1) + 2 newlines(2) + search box with border(3) + 2 newlines(2) + newline before footer(1) + footer(1) = 10 lines
		visibleHeight := m.terminalHeight - 10
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		
		// Calculate the range of items to display
		startIdx := m.scrollOffset
		endIdx := m.scrollOffset + visibleHeight
		if endIdx > len(m.filteredRepos) {
			endIdx = len(m.filteredRepos)
		}
		
		// Show scroll indicators if needed
		if m.scrollOffset > 0 {
			b.WriteString("↑ (more above)\n")
		}
		
		for i := startIdx; i < endIdx; i++ {
			repo := m.filteredRepos[i]
			pathColumn := fmt.Sprintf("%-*s", maxPathLen, minPaths[i])
			line := pathColumn
			
			if repo.GitHubURL != "N/A" && repo.GitHubURL != "Non-GitHub" {
				githubCheck := githubCheckStyle.Render("✓")
				line = fmt.Sprintf("%s  %s", line, githubCheck)
			}
			
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		
		// Show scroll indicator if there are more items below
		if endIdx < len(m.filteredRepos) {
			b.WriteString("↓ (more below)\n")
		}
	}
	
	b.WriteString("\n")
	b.WriteString("Use ↑/↓ or j/k to navigate, PgUp/PgDn for pages, Enter for details, c to cd and exit, q/Ctrl+C to quit")
	
	return b.String()
}

func (m model) renderDetailView() string {
	var b strings.Builder
	
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230"))
		
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14"))
		
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11"))
		
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9"))
	
	if m.selectedRepo == nil {
		return "No repository selected"
	}
	
	b.WriteString(headerStyle.Render("Repository Details"))
	b.WriteString("\n\n")
	
	b.WriteString(labelStyle.Render("Name: "))
	b.WriteString(m.selectedRepo.Directory)
	b.WriteString("\n\n")
	
	b.WriteString(labelStyle.Render("URL: "))
	urlLine := m.selectedRepo.GitHubURL
	if m.detailCursor == 0 {
		urlLine = selectedStyle.Render(urlLine)
	}
	b.WriteString(urlLine)
	b.WriteString("\n\n")
	
	b.WriteString(labelStyle.Render("Pull Requests:"))
	b.WriteString("\n")
	
	if m.loadingPRs {
		b.WriteString(loadingStyle.Render("Loading PRs..."))
		b.WriteString("\n")
	} else if m.prLoadError != "" {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.prLoadError)))
		b.WriteString("\n")
	} else if len(m.repoDetails) == 0 {
		b.WriteString("No open PRs by current user")
		b.WriteString("\n")
	} else {
		// Calculate visible area height for PR list
		// Header(1) + 2 newlines(2) + Name(1) + 2 newlines(2) + URL(1) + 2 newlines(2) + "Pull Requests:"(1) + newline before footer(1) + footer(1) = 11 lines
		visibleHeight := m.terminalHeight - 11
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		
		// Calculate total items (URL field + PRs)
		totalItems := 1 + len(m.repoDetails)
		
		// Calculate which PRs to show (accounting for URL field at index 0)
		startPRIdx := 0
		endPRIdx := len(m.repoDetails)
		
		if totalItems > visibleHeight {
			// Determine the visible range considering the cursor position
			if m.detailScrollOffset > 0 {
				// If we're scrolled past the URL field, show "more above" indicator
				b.WriteString("↑ (more above)\n")
			}
			
			// Calculate PR range to display
			prStartOffset := m.detailScrollOffset - 1 // Subtract 1 for URL field
			if prStartOffset < 0 {
				prStartOffset = 0
			}
			
			prVisibleCount := visibleHeight
			if m.detailScrollOffset == 0 {
				prVisibleCount-- // Account for URL field being visible
			}
			
			startPRIdx = prStartOffset
			endPRIdx = prStartOffset + prVisibleCount
			if endPRIdx > len(m.repoDetails) {
				endPRIdx = len(m.repoDetails)
			}
		}
		
		for i := startPRIdx; i < endPRIdx; i++ {
			pr := m.repoDetails[i]
			prLine := fmt.Sprintf("#%d: %s", pr.Number, pr.Title)
			if m.detailCursor == i+1 {
				prLine = selectedStyle.Render(prLine)
			}
			b.WriteString(prLine)
			b.WriteString("\n")
		}
		
		// Show "more below" indicator if needed
		if endPRIdx < len(m.repoDetails) {
			b.WriteString("↓ (more below)\n")
		}
	}
	
	b.WriteString("\n")
	b.WriteString("Use ↑/↓ or j/k to navigate, PgUp/PgDn for pages, Enter to open, c to cd and exit, Esc to go back, q/Ctrl+C to quit")
	
	return b.String()
}

func main() {
	skipIgnore := flag.Bool("skip-ignore", false, "Skip .gitignore files and traverse all directories")
	flag.Parse()

	// Get optional search term from positional arguments
	var initialSearch string
	if len(flag.Args()) > 0 {
		initialSearch = flag.Args()[0]
	}

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

	// Check if we're in a git repo with no subdirectories
	if len(repos) == 0 && isGitRepository(workingDir) {
		currentRepo, err := getCurrentRepoInfo(workingDir)
		if err == nil && isInteractive() {
			// Show detail view for current repository
			m := model{
				repos:         []GitRepo{*currentRepo},
				filteredRepos: []GitRepo{*currentRepo},
				searchInput:   "",
				cursor:        0,
				currentView:   detailView,
				selectedRepo:  currentRepo,
				repoDetails:   nil,
				detailCursor:  0,
				loadingPRs:    true,
				prLoadError:   "",
				startedInDetailView: true,
				terminalHeight: 24, // Default height, will be updated by WindowSizeMsg
			}
			
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running interactive mode: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	if len(repos) == 0 {
		fmt.Println("No git repositories found in subdirectories.")
		return
	}

	if isInteractive() {
		m := model{
			repos:         repos,
			filteredRepos: repos,
			searchInput:   initialSearch,
			cursor:        0,
			currentView:   listView,
			selectedRepo:  nil,
			repoDetails:   nil,
			detailCursor:  0,
			loadingPRs:    false,
			prLoadError:   "",
			startedInDetailView: false,
			terminalHeight: 24, // Default height, will be updated by WindowSizeMsg
		}
		
		// Apply initial filter if search term provided
		if initialSearch != "" {
			m.filterRepos()
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
		PRCount:   0,
	}, nil
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

			repos = append(repos, GitRepo{
				Directory: repoDir,
				Origin:    origin,
				GitHubURL: githubURL,
				PRCount:   0, // Will be loaded on-demand in detail view
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

var ghAuthWarningShown = false

func checkGitHubAuth() bool {
	cmd := exec.Command("gh", "auth", "status")
	err := cmd.Run()
	return err == nil
}

func getPRCount(repoURL string) int {
	if repoURL == "N/A" || repoURL == "Non-GitHub" {
		return 0
	}

	// Check GitHub CLI authentication once
	if !ghAuthWarningShown {
		if !checkGitHubAuth() {
			fmt.Fprintf(os.Stderr, "Warning: GitHub CLI not authenticated. PR counts will be unavailable.\n")
			fmt.Fprintf(os.Stderr, "Run 'gh auth login' to enable PR count features.\n\n")
			ghAuthWarningShown = true
			return 0
		}
		ghAuthWarningShown = true
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

func getRepositoryPRs(repoURL string) ([]PR, error) {
	if repoURL == "N/A" || repoURL == "Non-GitHub" {
		return nil, fmt.Errorf("not a GitHub repository")
	}

	// Check GitHub CLI authentication
	if !checkGitHubAuth() {
		return nil, fmt.Errorf("GitHub CLI not authenticated")
	}

	// Extract owner/repo from GitHub URL
	re := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)`)
	matches := re.FindStringSubmatch(repoURL)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid GitHub URL format")
	}

	owner := matches[1]
	repo := matches[2]

	// Get current user
	userCmd := exec.Command("gh", "api", "user", "--jq", ".login")
	userOutput, err := userCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	currentUser := strings.TrimSpace(string(userOutput))

	// Get PRs for current user with full details
	prCmd := exec.Command("gh", "pr", "list", "--repo", fmt.Sprintf("%s/%s", owner, repo), "--author", currentUser, "--json", "number,title,url")
	prOutput, err := prCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PRs: %w", err)
	}

	var prs []PR
	if err := json.Unmarshal(prOutput, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}

	return prs, nil
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