package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConvertToGitHubURL(t *testing.T) {
	tests := map[string]struct {
		origin string
		want   string
	}{
		"ssh with .git suffix": {
			origin: "git@github.com:owner/repo.git",
			want:   "https://github.com/owner/repo",
		},
		"ssh without .git suffix": {
			origin: "git@github.com:owner/repo",
			want:   "https://github.com/owner/repo",
		},
		"ssh scheme": {
			origin: "ssh://git@github.com/owner/repo.git",
			want:   "https://github.com/owner/repo",
		},
		"https with .git suffix": {
			origin: "https://github.com/owner/repo.git",
			want:   "https://github.com/owner/repo",
		},
		"https without .git suffix": {
			origin: "https://github.com/owner/repo",
			want:   "https://github.com/owner/repo",
		},
		"git protocol falls through to raw passthrough": {
			origin: "git://github.com/owner/repo.git",
			want:   "git://github.com/owner/repo.git",
		},
		"empty origin": {
			origin: "",
			want:   "N/A",
		},
		"N/A origin": {
			origin: "N/A",
			want:   "N/A",
		},
		"non-github host": {
			origin: "git@gitlab.com:owner/repo.git",
			want:   "Non-GitHub",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := convertToGitHubURL(tc.origin)
			if got != tc.want {
				t.Errorf("convertToGitHubURL(%q) = %q, want %q", tc.origin, got, tc.want)
			}
		})
	}
}

func TestIsGitRepository(t *testing.T) {
	tests := map[string]struct {
		setup func(t *testing.T, dir string)
		want  bool
	}{
		"no .git entry": {
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		".git directory": {
			setup: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatalf("creating .git dir: %v", err)
				}
			},
			want: true,
		},
		".git file (worktree)": {
			setup: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
					t.Fatalf("creating .git file: %v", err)
				}
			},
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			if got := isGitRepository(dir); got != tc.want {
				t.Errorf("isGitRepository() = %v, want %v", got, tc.want)
			}
		})
	}
}

// initGitRepo creates a real git repository in dir, optionally adding an origin
// remote pointing at originURL (skipped when originURL is empty).
func initGitRepo(t *testing.T, dir, originURL string) {
	t.Helper()

	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}

	if originURL == "" {
		return
	}

	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", originURL).CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin %s: %v\n%s", originURL, err, out)
	}
}

func TestGetOriginRemote(t *testing.T) {
	tests := map[string]struct {
		origin  string
		wantErr bool
	}{
		"ssh style remote": {
			origin: "git@github.com:owner/repo.git",
		},
		"https style remote": {
			origin: "https://github.com/owner/repo.git",
		},
		"no origin remote": {
			origin:  "",
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			initGitRepo(t, dir, tc.origin)

			got, err := getOriginRemote(dir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("getOriginRemote() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("getOriginRemote() unexpected error: %v", err)
			}
			if got != tc.origin {
				t.Errorf("getOriginRemote() = %q, want %q", got, tc.origin)
			}
		})
	}
}

func TestGetCurrentRepoInfo(t *testing.T) {
	t.Run("git repo with github origin", func(t *testing.T) {
		dir := t.TempDir()
		origin := "git@github.com:owner/repo.git"
		initGitRepo(t, dir, origin)

		repo, err := getCurrentRepoInfo(dir)
		if err != nil {
			t.Fatalf("getCurrentRepoInfo() unexpected error: %v", err)
		}
		if repo.Directory != dir {
			t.Errorf("Directory = %q, want %q", repo.Directory, dir)
		}
		if repo.Origin != origin {
			t.Errorf("Origin = %q, want %q", repo.Origin, origin)
		}
		if want := "https://github.com/owner/repo"; repo.GitHubURL != want {
			t.Errorf("GitHubURL = %q, want %q", repo.GitHubURL, want)
		}
	})

	t.Run("git repo without origin", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir, "")

		repo, err := getCurrentRepoInfo(dir)
		if err != nil {
			t.Fatalf("getCurrentRepoInfo() unexpected error: %v", err)
		}
		if repo.Origin != "N/A" {
			t.Errorf("Origin = %q, want %q", repo.Origin, "N/A")
		}
		if repo.GitHubURL != "N/A" {
			t.Errorf("GitHubURL = %q, want %q", repo.GitHubURL, "N/A")
		}
	})

	t.Run("not a git repo", func(t *testing.T) {
		dir := t.TempDir()

		if _, err := getCurrentRepoInfo(dir); err == nil {
			t.Fatal("getCurrentRepoInfo() expected an error for a non-git directory")
		}
	})
}

func TestParseArtifactComment(t *testing.T) {
	const commitURL = "https://github.com/cerebrotech/catalog-platform/commit/8beeab48dffaabddb934c75bdb54538d060d3396"

	// The real comment body posted on cerebrotech/catalog-platform#268, including
	// the marker, heading and hint lines that must not be parsed as artifacts.
	realBody := "<!-- catalog:artifacts -->\n" +
		"Artifacts\n" +
		"Hint: You can edit this comment and clean up old commits, add the `silence-artifact-comments` label to prevent updates, or add `verbose-artifact-comments` to print all image variations (ie ecr, floating)..\n" +
		"- [`8beeab4`](" + commitURL + "): agent image: quay.io/domino/fleetcommand-agent:6.4.0.catalog-platform-kustomize-helmfile.8beeab4\n" +
		"- [`8beeab4`](" + commitURL + "): deployer image: quay.io/domino/deployer:6.4.0.catalog-platform-kustomize-helmfile.8beeab4\n"

	tests := map[string]struct {
		body string
		want []Artifact
	}{
		"real two-artifact comment": {
			body: realBody,
			want: []Artifact{
				{
					ShortSHA:  "8beeab4",
					CommitURL: commitURL,
					Label:     "agent image",
					ImageRef:  "quay.io/domino/fleetcommand-agent:6.4.0.catalog-platform-kustomize-helmfile.8beeab4",
				},
				{
					ShortSHA:  "8beeab4",
					CommitURL: commitURL,
					Label:     "deployer image",
					ImageRef:  "quay.io/domino/deployer:6.4.0.catalog-platform-kustomize-helmfile.8beeab4",
				},
			},
		},
		"empty body": {
			body: "",
			want: nil,
		},
		"single well-formed artifact line": {
			body: "- [`abc1234`](" + commitURL + "): deployer image: quay.io/domino/deployer:1.2.3\n",
			want: []Artifact{
				{
					ShortSHA:  "abc1234",
					CommitURL: commitURL,
					Label:     "deployer image",
					ImageRef:  "quay.io/domino/deployer:1.2.3",
				},
			},
		},
		"well-formed line alongside junk lines": {
			body: "Artifacts\n" +
				"just some prose that mentions an image: quay.io/domino/nope:1.0\n" +
				"- [`abc1234`](" + commitURL + "): deployer image: quay.io/domino/deployer:1.2.3\n" +
				"- [`deadbee`] malformed line missing the parenthesised URL\n",
			want: []Artifact{
				{
					ShortSHA:  "abc1234",
					CommitURL: commitURL,
					Label:     "deployer image",
					ImageRef:  "quay.io/domino/deployer:1.2.3",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := parseArtifactComment(tc.body)

			if len(got) != len(tc.want) {
				t.Fatalf("parseArtifactComment() returned %d artifacts, want %d: %+v", len(got), len(tc.want), got)
			}

			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("artifact[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestGroupArtifactsByCommit(t *testing.T) {
	a := Artifact{ShortSHA: "aaa1111", CommitURL: "https://example.com/commit/aaa1111", Label: "agent image", ImageRef: "img:a"}
	a2 := Artifact{ShortSHA: "aaa1111", CommitURL: "https://example.com/commit/aaa1111", Label: "deployer image", ImageRef: "img:a2"}
	b := Artifact{ShortSHA: "bbb2222", CommitURL: "https://example.com/commit/bbb2222", Label: "agent image", ImageRef: "img:b"}

	tests := map[string]struct {
		artifacts []Artifact
		want      []ArtifactGroup
	}{
		"no artifacts": {
			artifacts: nil,
			want:      nil,
		},
		"single build, multiple artifacts": {
			artifacts: []Artifact{a, a2},
			want: []ArtifactGroup{
				{ShortSHA: "aaa1111", CommitURL: "https://example.com/commit/aaa1111", Artifacts: []Artifact{a, a2}},
			},
		},
		"multiple builds stay in first-seen order": {
			artifacts: []Artifact{a, a2, b},
			want: []ArtifactGroup{
				{ShortSHA: "aaa1111", CommitURL: "https://example.com/commit/aaa1111", Artifacts: []Artifact{a, a2}},
				{ShortSHA: "bbb2222", CommitURL: "https://example.com/commit/bbb2222", Artifacts: []Artifact{b}},
			},
		},
		"interleaved shas still merge into one group per sha": {
			artifacts: []Artifact{a, b, a2},
			want: []ArtifactGroup{
				{ShortSHA: "aaa1111", CommitURL: "https://example.com/commit/aaa1111", Artifacts: []Artifact{a, a2}},
				{ShortSHA: "bbb2222", CommitURL: "https://example.com/commit/bbb2222", Artifacts: []Artifact{b}},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := groupArtifactsByCommit(tc.artifacts)

			if len(got) != len(tc.want) {
				t.Fatalf("groupArtifactsByCommit() returned %d groups, want %d: %+v", len(got), len(tc.want), got)
			}

			for i := range tc.want {
				if got[i].ShortSHA != tc.want[i].ShortSHA || got[i].CommitURL != tc.want[i].CommitURL {
					t.Errorf("group[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
				if len(got[i].Artifacts) != len(tc.want[i].Artifacts) {
					t.Fatalf("group[%d] has %d artifacts, want %d: %+v", i, len(got[i].Artifacts), len(tc.want[i].Artifacts), got[i])
				}
				for j := range tc.want[i].Artifacts {
					if got[i].Artifacts[j] != tc.want[i].Artifacts[j] {
						t.Errorf("group[%d].artifacts[%d] = %+v, want %+v", i, j, got[i].Artifacts[j], tc.want[i].Artifacts[j])
					}
				}
			}
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	tests := map[string]struct {
		url       string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		"valid github url": {
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantOK:    true,
		},
		"N/A url": {
			url:    "N/A",
			wantOK: false,
		},
		"non-github url": {
			url:    "Non-GitHub",
			wantOK: false,
		},
		"empty url": {
			url:    "",
			wantOK: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			owner, repo, ok := parseGitHubURL(tc.url)
			if ok != tc.wantOK {
				t.Fatalf("parseGitHubURL(%q) ok = %v, want %v", tc.url, ok, tc.wantOK)
			}
			if owner != tc.wantOwner {
				t.Errorf("parseGitHubURL(%q) owner = %q, want %q", tc.url, owner, tc.wantOwner)
			}
			if repo != tc.wantRepo {
				t.Errorf("parseGitHubURL(%q) repo = %q, want %q", tc.url, repo, tc.wantRepo)
			}
		})
	}
}
