package git

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// CreateBranch creates and checks out a new branch
func CreateBranch(dir, name string) error {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "checkout", "-b", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Checkout switches to an existing branch
func Checkout(dir, name string) error {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "checkout", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CheckoutTracking creates a local tracking branch from a remote branch and checks it out
func CheckoutTracking(dir, remoteBranch string) error {
	_, localName, ok := strings.Cut(remoteBranch, "/")
	if !ok {
		return fmt.Errorf("invalid remote branch format: %s", remoteBranch)
	}

	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "checkout", "-b", localName, "--track", remoteBranch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// LocalBranches returns all local branch names
func LocalBranches(dir string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "branch", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseLines(out), nil
}

// RemoteBranches returns all remote branch names
func RemoteBranches(dir string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "branch", "-r", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := parseLines(out)
	var result []string
	for _, l := range lines {
		if !strings.Contains(l, "->") {
			result = append(result, l)
		}
	}
	return result, nil
}

// IsRemoteBranch reports whether name exactly matches a known remote branch.
// Returns false on error so callers can fall through to local creation.
func IsRemoteBranch(dir, name string) bool {
	remotes, err := RemoteBranches(dir)
	if err != nil {
		return false
	}
	return slices.Contains(remotes, name)
}

// BranchAction is the routing decision for a confirmed branch name.
type BranchAction int

const (
	ActionCreate BranchAction = iota
	ActionCheckout
	ActionCheckoutTracking
)

// ResolveBranchAction decides how to act on a confirmed branch name.
// Order: existing local -> Checkout; exact remote match -> CheckoutTracking;
// otherwise -> Create. See spec-lj-0023.
func ResolveBranchAction(dir, name string) BranchAction {
	switch {
	case BranchExists(dir, name):
		return ActionCheckout
	case IsRemoteBranch(dir, name):
		return ActionCheckoutTracking
	default:
		return ActionCreate
	}
}

// BranchExists returns true if a local branch with the given name exists
func BranchExists(dir, name string) bool {
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "rev-parse", "--verify", "refs/heads/"+name)
	return cmd.Run() == nil
}

func parseLines(data []byte) []string {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
