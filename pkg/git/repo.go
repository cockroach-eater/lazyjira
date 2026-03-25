package git

import (
	"context"
	"os/exec"
	"strings"
)

// GitAvailable returns true if git is found in PATH.
func GitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo returns true if dir is inside a git repository.
func IsRepo(dir string) bool {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// CurrentBranch returns the current branch name, or "" for detached HEAD.
func CurrentBranch(dir string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// Detached HEAD — not an error, just no branch name.
		return "", nil //nolint:nilerr // detached HEAD is expected
	}
	return strings.TrimSpace(string(out)), nil
}
