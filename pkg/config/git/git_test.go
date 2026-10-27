package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeLinkedWorktree(t *testing.T) {
	t.Setenv("STREAMPLACE_VERSION_OVERRIDE", "")
	t.Setenv("CI_COMMIT_TAG", "")
	repo := filepath.Join(t.TempDir(), "repo")
	linked := filepath.Join(t.TempDir(), "linked")
	runGit := func(t *testing.T, dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	runGit(t, "", "init", repo)
	runGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	runGit(t, repo, "tag", "v0.0.1")
	runGit(t, repo, "worktree", "add", "-b", "test-linked", linked)
	runGit(t, linked, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "linked commit")

	for _, dir := range []string{repo, linked} {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			r, err := PlainOpen(dir)
			require.NoError(t, err)
			head, err := r.Head()
			require.NoError(t, err)
			expected := runGit(t, dir, "rev-parse", "HEAD")
			require.Equal(t, expected, head.Hash().String())
			description, err := r.Describe(head)
			require.NoError(t, err)
			require.Equal(t, "v0.0.1-"+expected[:8], description)
		})
	}
}
