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
