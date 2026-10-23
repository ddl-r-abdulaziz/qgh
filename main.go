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

type Artifact struct {
	ShortSHA  string
	CommitURL string
	Label     string
	ImageRef  string
}

// ArtifactGroup is one build (commit) and the artifacts it produced.
type ArtifactGroup struct {
	ShortSHA  string
	CommitURL string
	Artifacts []Artifact
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
	owner, repoName, _ := parseGitHubURL(repo.GitHubURL)
	m := model{
		repo:           repo,
		owner:          owner,
		repoName:       repoName,
		loadingPRs:     true,
		terminalHeight: 24,
		terminalWidth:  80,
		artifacts:      make(map[int]artifactState),
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "qgh: error running interactive mode: %v\n", err)
		os.Exit(1)
	}
}

type artifactState struct {
	loading   bool
	artifacts []Artifact
	err       error
}

type model struct {
	repo     *GitRepo
	owner    string
	repoName string

	prs         []PR
	cursor      int
	loadingPRs  bool
	prLoadError string

	artifacts map[int]artifactState // keyed by PR number

	terminalHeight int
	terminalWidth  int
	scrollOffset   int
	statusMsg      string
}

type prLoadedMsg struct {
	prs []PR
	err error
}

type artifactsLoadedMsg struct {
	prNumber  int
	artifacts []Artifact
	err       error
}

func loadPRsCmd(repoURL string) tea.Cmd {
	return func() tea.Msg {
		prs, err := getRepositoryPRs(repoURL)
		return prLoadedMsg{prs: prs, err: err}
	}
}

func loadArtifactsCmd(owner, repoName string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		artifacts, err := getPRArtifacts(owner, repoName, prNumber)
		return artifactsLoadedMsg{prNumber: prNumber, artifacts: artifacts, err: err}
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

// selectedPR returns the PR under the cursor, or nil if the cursor is on the URL row.
func (m model) selectedPR() *PR {
	if m.cursor == 0 || m.cursor-1 >= len(m.prs) {
		return nil
	}
	return &m.prs[m.cursor-1]
}

func (m *model) ensureArtifactsLoading() tea.Cmd {
	pr := m.selectedPR()
	if pr == nil {
		return nil
	}
	if _, ok := m.artifacts[pr.Number]; ok {
		return nil
	}
	m.artifacts[pr.Number] = artifactState{loading: true}
	return loadArtifactsCmd(m.owner, m.repoName, pr.Number)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalHeight = msg.Height
		m.terminalWidth = msg.Width
		return m, nil

	case prLoadedMsg:
		m.loadingPRs = false
		if msg.err != nil {
			m.prLoadError = msg.err.Error()
		} else {
			m.prs = msg.prs
		}
		cmd := m.ensureArtifactsLoading()
		return m, cmd

	case artifactsLoadedMsg:
		m.artifacts[msg.prNumber] = artifactState{artifacts: msg.artifacts, err: msg.err}
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
				return m, m.ensureArtifactsLoading()
			}
		case "down":
			maxItems := 1 + len(m.prs)
			if m.cursor < maxItems-1 {
				m.cursor++
				visibleHeight := m.visibleHeight()
				if m.cursor >= m.scrollOffset+visibleHeight {
					m.scrollOffset = m.cursor - visibleHeight + 1
				}
				return m, m.ensureArtifactsLoading()
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
			return m, m.ensureArtifactsLoading()
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
			return m, m.ensureArtifactsLoading()
		case "enter":
			m.statusMsg = ""
			if m.cursor == 0 {
				if m.repo.GitHubURL != "N/A" && m.repo.GitHubURL != "Non-GitHub" {
					if err := openURL(m.repo.GitHubURL); err != nil {
						m.statusMsg = fmt.Sprintf("could not open browser: %v", err)
					}
				}
			} else if pr := m.selectedPR(); pr != nil {
				if err := openURL(pr.URL); err != nil {
					m.statusMsg = fmt.Sprintf("could not open browser: %v", err)
				}
			}
		case "c":
			m.statusMsg = ""
			if pr := m.selectedPR(); pr != nil {
				if state, ok := m.artifacts[pr.Number]; ok && len(state.artifacts) > 0 {
					if err := openURL(state.artifacts[0].CommitURL); err != nil {
						m.statusMsg = fmt.Sprintf("could not open browser: %v", err)
					}
				}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	left := m.renderList()
	right := m.renderArtifactPanel()

	leftWidth := m.terminalWidth * 3 / 5
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := m.terminalWidth - leftWidth - 3
	if rightWidth < 20 {
		rightWidth = 20
	}

	leftStyle := lipgloss.NewStyle().Width(leftWidth)
	rightStyle := lipgloss.NewStyle().Width(rightWidth).PaddingLeft(2).
		BorderStyle(lipgloss.NormalBorder()).BorderLeft(true).
		BorderTop(false).BorderRight(false).BorderBottom(false)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftStyle.Render(left), rightStyle.Render(right))

	footer := "↑/↓ navigate  PgUp/PgDn page  Enter open  c open commit  Esc/q quit"
	if m.statusMsg != "" {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.statusMsg) + "\n" + footer
	}
	return body + "\n\n" + footer
}

func (m model) renderList() string {
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

	return b.String()
}

func (m model) renderArtifactPanel() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))

	b.WriteString(headerStyle.Render("Artifacts"))
	b.WriteString("\n\n")

	pr := m.selectedPR()
	if pr == nil {
		b.WriteString("Select a PR to see its build artifacts.")
		return b.String()
	}

	state, ok := m.artifacts[pr.Number]
	switch {
	case !ok || state.loading:
		b.WriteString(loadingStyle.Render("Loading artifacts..."))
	case state.err != nil:
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", state.err)))
	case len(state.artifacts) == 0:
		b.WriteString("No artifact info for this PR.")
	default:
		groups := groupArtifactsByCommit(state.artifacts)
		for gi, g := range groups {
			if gi > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(labelStyle.Render("Commit: "))
			b.WriteString(g.ShortSHA)
			b.WriteString("\n")
			for _, a := range g.Artifacts {
				b.WriteString(fmt.Sprintf("  %s: %s\n", a.Label, a.ImageRef))
			}
		}
		b.WriteString("\n")
		b.WriteString("(press c to open the first commit above)")
	}

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

func parseGitHubURL(url string) (owner, repo string, ok bool) {
	re := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) != 3 {
		return "", "", false
	}
	return matches[1], matches[2], true
}

func getRepositoryPRs(repoURL string) ([]PR, error) {
	if repoURL == "N/A" || repoURL == "Non-GitHub" {
		return nil, fmt.Errorf("not a GitHub repository")
	}
	if !checkGitHubAuth() {
		return nil, fmt.Errorf("GitHub CLI not authenticated")
	}

	owner, repo, ok := parseGitHubURL(repoURL)
	if !ok {
		return nil, fmt.Errorf("invalid GitHub URL format")
	}

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

var artifactLineRegex = regexp.MustCompile("^- \\[`([0-9a-f]+)`\\]\\(([^)]+)\\): (.+)$")

func parseArtifactComment(body string) []Artifact {
	var artifacts []Artifact
	for _, line := range strings.Split(body, "\n") {
		matches := artifactLineRegex.FindStringSubmatch(strings.TrimSpace(line))
		if matches == nil {
			continue
		}
		label, ref := matches[3], ""
		if idx := strings.Index(matches[3], ": "); idx != -1 {
			label, ref = matches[3][:idx], matches[3][idx+2:]
		}
		artifacts = append(artifacts, Artifact{
			ShortSHA:  matches[1],
			CommitURL: matches[2],
			Label:     label,
			ImageRef:  ref,
		})
	}
	return artifacts
}

// groupArtifactsByCommit groups artifacts by ShortSHA, preserving the order
// each SHA first appears in.
func groupArtifactsByCommit(artifacts []Artifact) []ArtifactGroup {
	var groups []ArtifactGroup
	index := make(map[string]int)
	for _, a := range artifacts {
		if i, ok := index[a.ShortSHA]; ok {
			groups[i].Artifacts = append(groups[i].Artifacts, a)
			continue
		}
		index[a.ShortSHA] = len(groups)
		groups = append(groups, ArtifactGroup{
			ShortSHA:  a.ShortSHA,
			CommitURL: a.CommitURL,
			Artifacts: []Artifact{a},
		})
	}
	return groups
}

func getPRArtifacts(owner, repoName string, prNumber int) ([]Artifact, error) {
	cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", prNumber),
		"--repo", fmt.Sprintf("%s/%s", owner, repoName), "--json", "comments")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PR comments: %w", err)
	}

	var result struct {
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse PR comments: %w", err)
	}

	for _, c := range result.Comments {
		if strings.Contains(c.Body, "<!-- catalog:artifacts -->") {
			return parseArtifactComment(c.Body), nil
		}
	}
	return nil, nil
}
