package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a fresh git repo with one commit on the default branch
// (renamed to "main"), and returns the directory.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README")
	run("commit", "-q", "-m", "init")
	return dir
}

// addRemoteRef pins refs/remotes/<ref> to current HEAD without any network.
func addRemoteRef(t *testing.T, dir, ref string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", "-C", dir, "update-ref", "refs/remotes/"+ref, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("update-ref %s failed: %v\n%s", ref, err, out)
	}
}

func TestResolveBranchAction_slashedNewName_isCreate(t *testing.T) {
	// Bug case from lj-0023: a name containing "/" that does not match any
	// remote branch must route to ActionCreate, not ActionCheckoutTracking.
	dir := initRepo(t)
	addRemoteRef(t, dir, "origin/main")

	got := ResolveBranchAction(dir, "feature/PROJ-1-foo")
	if got != ActionCreate {
		t.Errorf("ResolveBranchAction = %v, want ActionCreate", got)
	}
}

func TestResolveBranchAction_existingLocal_isCheckout(t *testing.T) {
	dir := initRepo(t)

	got := ResolveBranchAction(dir, "main")
	if got != ActionCheckout {
		t.Errorf("ResolveBranchAction = %v, want ActionCheckout", got)
	}
}

func TestResolveBranchAction_existingRemote_isTracking(t *testing.T) {
	dir := initRepo(t)
	// Use a name that does not exist locally.
	addRemoteRef(t, dir, "origin/feature-x")

	got := ResolveBranchAction(dir, "origin/feature-x")
	if got != ActionCheckoutTracking {
		t.Errorf("ResolveBranchAction = %v, want ActionCheckoutTracking", got)
	}
}

func TestResolveBranchAction_plainName_isCreate(t *testing.T) {
	dir := initRepo(t)

	got := ResolveBranchAction(dir, "PROJ-1-foo")
	if got != ActionCreate {
		t.Errorf("ResolveBranchAction = %v, want ActionCreate", got)
	}
}

func TestIsRemoteBranch_exactMatchRequired(t *testing.T) {
	dir := initRepo(t)
	addRemoteRef(t, dir, "origin/feature/x")

	if IsRemoteBranch(dir, "origin/feature/y") {
		t.Errorf("IsRemoteBranch matched non-existent sibling branch")
	}
	if !IsRemoteBranch(dir, "origin/feature/x") {
		t.Errorf("IsRemoteBranch did not match exact remote branch")
	}
}

func TestIsRemoteBranch_nonGitDir_returnsFalse(t *testing.T) {
	dir := t.TempDir()
	if IsRemoteBranch(dir, "origin/main") {
		t.Errorf("IsRemoteBranch returned true for non-git dir")
	}
}
