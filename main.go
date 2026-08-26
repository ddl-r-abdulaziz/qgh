package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	interactive := flag.Bool("i", false, "interactive mode: browse this repo's PRs in a terminal UI")
	flag.Parse()

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

	if *interactive {
		runInteractive(repo)
		return
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

func runInteractive(repo *GitRepo) {
	m := model{
		repo:           repo,
		loadingPRs:     true,
		terminalHeight: 24,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "qgh: error running interactive mode: %v\n", err)
		os.Exit(1)
	}
}

type model struct {
	repo        *GitRepo
	prs         []PR
	cursor      int
	loadingPRs  bool
	prLoadError string

	terminalHeight int
	scrollOffset   int
}

type prLoadedMsg struct {
	prs []PR
	err error
}

func loadPRsCmd(repoURL string) tea.Cmd {
	return func() tea.Msg {
		prs, err := getRepositoryPRs(repoURL)
		return prLoadedMsg{prs: prs, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return loadPRsCmd(m.repo.GitHubURL)
}

func (m model) visibleHeight() int {
	h := m.terminalHeight - 13
	if h < 1 {
		h = 1
	}
	return h
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
			m.prs = msg.prs
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scrollOffset {
					m.scrollOffset = m.cursor
				}
			}
		case "down":
			maxItems := 1 + len(m.prs)
			if m.cursor < maxItems-1 {
				m.cursor++
				visibleHeight := m.visibleHeight()
				if m.cursor >= m.scrollOffset+visibleHeight {
					m.scrollOffset = m.cursor - visibleHeight + 1
				}
			}
		case "pgup":
			visibleHeight := m.visibleHeight()
			m.cursor -= visibleHeight
			if m.cursor < 0 {
				m.cursor = 0
			}
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		case "pgdown":
			visibleHeight := m.visibleHeight()
			maxItems := 1 + len(m.prs)
			m.cursor += visibleHeight
			if m.cursor >= maxItems {
				m.cursor = maxItems - 1
			}
			if m.cursor >= m.scrollOffset+visibleHeight {
				m.scrollOffset = m.cursor - visibleHeight + 1
			}
		case "enter":
			if m.cursor == 0 {
				if m.repo.GitHubURL != "N/A" && m.repo.GitHubURL != "Non-GitHub" {
					openURL(m.repo.GitHubURL)
				}
			} else if m.cursor-1 < len(m.prs) {
				openURL(m.prs[m.cursor-1].URL)
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	b.WriteString(headerStyle.Render("Repository Details"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Name: "))
	b.WriteString(m.repo.Directory)
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("URL: "))
	urlLine := m.repo.GitHubURL
	if m.cursor == 0 {
		urlLine = selectedStyle.Render(urlLine)
	}
	b.WriteString(urlLine)
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Pull Requests:"))
	b.WriteString("\n")

	switch {
	case m.loadingPRs:
		b.WriteString(loadingStyle.Render("Loading PRs..."))
		b.WriteString("\n")
	case m.prLoadError != "":
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.prLoadError)))
		b.WriteString("\n")
	case len(m.prs) == 0:
		b.WriteString("No open PRs by current user\n")
	default:
		visibleHeight := m.visibleHeight()
		totalItems := 1 + len(m.prs)

		startIdx, endIdx := 0, len(m.prs)
		if totalItems > visibleHeight {
			if m.scrollOffset > 0 {
				b.WriteString("↑ (more above)\n")
			}
			prStart := m.scrollOffset - 1
			if prStart < 0 {
				prStart = 0
			}
			prVisible := visibleHeight
			if m.scrollOffset == 0 {
				prVisible--
			}
			startIdx = prStart
			endIdx = prStart + prVisible
			if endIdx > len(m.prs) {
				endIdx = len(m.prs)
			}
		}

		for i := startIdx; i < endIdx; i++ {
			pr := m.prs[i]
			line := fmt.Sprintf("#%d: %s", pr.Number, pr.Title)
			if m.cursor == i+1 {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		if endIdx < len(m.prs) {
			b.WriteString("↓ (more below)\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("↑/↓ navigate  PgUp/PgDn page  Enter open  Esc/q quit")

	return b.String()
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
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
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
