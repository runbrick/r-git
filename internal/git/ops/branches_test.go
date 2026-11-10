package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/testutil"
)

func newService(t *testing.T) (*Service, *testutil.Repo) {
	t.Helper()
	repo := testutil.NewRepo(t)
	git, err := command.New()
	if err != nil {
		t.Fatal(err)
	}
	return New(git, repo.Dir), repo
}

func headOf(t *testing.T, repo *testutil.Repo) string {
	t.Helper()
	return strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
}

func branchOf(t *testing.T, repo *testutil.Repo) string {
	t.Helper()
	return strings.TrimSpace(repo.Git("branch", "--show-current"))
}

func fileText(t *testing.T, repo *testutil.Repo, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo.Dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestCheckoutWithoutProblems ports
// GitBranchWorkerTest.`test checkout without problems`.
func TestCheckoutWithoutProblems(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("branch", "feature")
	repo.WriteFile("a.txt", "b\n")
	repo.Commit("on master")

	hash, err := Branches{svc}.Checkout(context.Background(), "feature")
	if err != nil {
		t.Fatal(err)
	}
	if branchOf(t, repo) != "feature" {
		t.Fatalf("branch = %q", branchOf(t, repo))
	}
	if hash.String() != headOf(t, repo) {
		t.Fatalf("returned %s, HEAD is %s", hash, headOf(t, repo))
	}
	if fileText(t, repo, "a.txt") != "a\n" {
		t.Fatalf("content = %q", fileText(t, repo, "a.txt"))
	}
}

// TestCheckoutWithLocalChangesOverwrittenByCheckout ports
// GitBranchWorkerTest.`test checkout with local changes overwritten by checkout should show smart checkout dialog`.
// The dialog is a typed error listing the files, so the UI can offer stash-and-retry.
func TestCheckoutWithLocalChangesOverwrittenByCheckout(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("init")
	repo.Git("branch", "feature")
	repo.WriteFile("a.txt", "feature\n")
	repo.Commit("feature edit")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "dirty\n")

	_, err := Branches{svc}.Checkout(context.Background(), "feature")
	var overwritten *command.ErrLocalChangesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v, want ErrLocalChangesWouldBeOverwritten", err)
	}
	if len(overwritten.Files) != 1 || overwritten.Files[0] != "a.txt" {
		t.Fatalf("files = %#v", overwritten.Files)
	}
	if branchOf(t, repo) != "master" {
		t.Fatalf("branch changed to %q", branchOf(t, repo))
	}
}

// TestCheckoutWithSeveralLocalChangesOverwritten ports
// GitBranchWorkerTest.`test checkout with several local changes overwritten by checkout should show smart checkout dialog`.
func TestCheckoutWithSeveralLocalChangesOverwritten(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("init")
	repo.Git("branch", "feature")
	repo.WriteFile("a.txt", "A\n")
	repo.WriteFile("b.txt", "B\n")
	repo.Commit("feature")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "dirty-a\n")
	repo.WriteFile("b.txt", "dirty-b\n")

	_, err := Branches{svc}.Checkout(context.Background(), "feature")
	var overwritten *command.ErrLocalChangesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v", err)
	}
	if strings.Join(overwritten.Files, ",") != "a.txt,b.txt" {
		t.Fatalf("files = %#v", overwritten.Files)
	}
}

// TestAgreeToSmartCheckout ports
// GitBranchWorkerTest.`test agree to smart checkout should smart checkout`.
// a.txt differs on the destination and is dirty locally, so a plain
// checkout refuses. The stash/checkout/pop sequence switches the branch
// and restores the local edit on top of the destination content.
func TestAgreeToSmartCheckout(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.WriteFile("keep.txt", "keep\n")
	repo.Commit("init")
	repo.Git("branch", "feature")
	repo.WriteFile("a.txt", "feature\n")
	repo.Commit("feature edit")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "dirty\n")
	repo.WriteFile("keep.txt", "local\n")

	_, err := (Branches{svc}).Checkout(context.Background(), "feature")
	var overwritten *command.ErrLocalChangesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("plain checkout: got %v, want local-changes error", err)
	}

	hash, err := (Branches{svc}).CheckoutWithStash(context.Background(), "feature", "Checkout")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v, want a conflict from the unstash", err)
	}
	if hash.IsZero() {
		t.Fatal("checkout hash was not returned")
	}
	if branchOf(t, repo) != "feature" {
		t.Fatalf("branch = %q", branchOf(t, repo))
	}
	if fileText(t, repo, "keep.txt") != "local\n" {
		t.Fatalf("keep.txt = %q", fileText(t, repo, "keep.txt"))
	}
	if strings.TrimSpace(repo.Git("stash", "list")) == "" {
		t.Fatal("conflicting stash was dropped")
	}
}

// TestCheckoutWithUntrackedFilesOverwritten ports
// GitBranchWorkerTest.`test checkout with untracked files overwritten by checkout in first repo should show notification`.
func TestCheckoutWithUntrackedFilesOverwritten(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("new.txt", "from feature\n")
	repo.Commit("add new")
	repo.Git("checkout", "master")
	repo.WriteFile("new.txt", "untracked\n")

	_, err := Branches{svc}.Checkout(context.Background(), "feature")
	var overwritten *command.ErrUntrackedFilesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v, want ErrUntrackedFilesWouldBeOverwritten", err)
	}
	if len(overwritten.Files) != 1 || overwritten.Files[0] != "new.txt" {
		t.Fatalf("files = %#v", overwritten.Files)
	}
	if fileText(t, repo, "new.txt") != "untracked\n" {
		t.Fatalf("untracked file was overwritten: %q", fileText(t, repo, "new.txt"))
	}
}

// TestCheckoutWithUnmergedFiles ports
// GitBranchWorkerTest.`test checkout with unmerged files in first repo should show notification`.
func TestCheckoutWithUnmergedFiles(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("init")
	repo.Git("checkout", "-b", "other")
	repo.WriteFile("a.txt", "other\n")
	repo.Commit("other")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "master\n")
	repo.Commit("master")
	repo.Git("branch", "side")
	// Leave the merge conflict in the index.
	if out, err := repo.GitAllowError("merge", "other"); err == nil {
		t.Fatalf("expected a merge conflict, got %s", out)
	}

	_, err := Branches{svc}.Checkout(context.Background(), "side")
	if !errors.Is(err, ErrUnmergedFiles) {
		t.Fatalf("got %v, want ErrUnmergedFiles", err)
	}
}

// TestCreateNewBranchWithoutProblems ports
// GitBranchWorkerTest.`test create new branch without problems`.
func TestCreateNewBranchWithoutProblems(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	base := repo.Commit("init")

	hash, err := Branches{svc}.CheckoutNew(context.Background(), "feature", "")
	if err != nil {
		t.Fatal(err)
	}
	if branchOf(t, repo) != "feature" {
		t.Fatalf("branch = %q", branchOf(t, repo))
	}
	if hash.String() != base {
		t.Fatalf("new branch points at %s, want %s", hash, base)
	}
}

// TestCreateNewBranchNotAtHEAD ports
// GitBranchWorkerTest.`test create new branch without checkout not at HEAD`
// for the checkout form: git checkout -b name <start-point>.
func TestCreateNewBranchNotAtHEAD(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "b\n")
	repo.Commit("second")

	hash, err := Branches{svc}.CheckoutNew(context.Background(), "from-first", first)
	if err != nil {
		t.Fatal(err)
	}
	if hash.String() != first {
		t.Fatalf("branch at %s, want %s", hash, first)
	}
	if branchOf(t, repo) != "from-first" {
		t.Fatalf("branch = %q", branchOf(t, repo))
	}
}

// TestDeleteBranchThatIsFullyMerged ports
// GitBranchWorkerTest.`test delete branch that is fully merged`.
func TestDeleteBranchThatIsFullyMerged(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("branch", "feature")

	if err := (Branches{Service: svc}).Delete(context.Background(), "feature", false); err != nil {
		t.Fatal(err)
	}
	out := repo.Git("branch", "--list", "feature")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("branch still exists: %q", out)
	}
}

// TestDeleteUnmergedBranch ports
// GitBranchWorkerTest.`test delete unmerged branch should restore on link click`
// and `test ok in unmerged branch dialog should force delete branch`.
// Force is false, so the unmerged commits are returned and the branch stays.
func TestDeleteUnmergedBranch(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "feature\n")
	tip := repo.Commit("feature commit")
	repo.Git("checkout", "master")

	err := Branches{svc}.Delete(context.Background(), "feature", false)
	var unmerged *ErrNotFullyMerged
	if !errors.As(err, &unmerged) {
		t.Fatalf("got %v, want ErrNotFullyMerged", err)
	}
	if unmerged.Branch != "feature" {
		t.Fatalf("branch = %q", unmerged.Branch)
	}
	if len(unmerged.Commits) != 1 || unmerged.Commits[0].String() != tip {
		t.Fatalf("commits = %#v, want %s", unmerged.Commits, tip)
	}
	if strings.TrimSpace(repo.Git("branch", "--list", "feature")) == "" {
		t.Fatal("branch was deleted")
	}

	if err := (Branches{Service: svc}).Delete(context.Background(), "feature", true); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(repo.Git("branch", "--list", "feature")) != "" {
		t.Fatal("force delete left the branch")
	}
}

// TestDeleteBranchMergedToHeadButUnmergedToUpstream ports
// GitBranchWorkerTest.`test delete branch merged to head but unmerged to upstream should mention this in notification`.
func TestDeleteBranchMergedToHeadButUnmergedToUpstream(t *testing.T) {
	svc, repo := newService(t)
	remote := testutil.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("remote", "add", "origin", remote.Dir)
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "feature\n")
	repo.Commit("feature")
	repo.Git("push", "-u", "origin", "feature")
	// Point the upstream back at master so the local tip is merged to HEAD
	// after the merge below, but not contained in origin/feature.
	repo.Git("push", "--force", "origin", "master:feature")
	repo.Git("checkout", "master")
	repo.Git("merge", "--ff-only", "feature")

	err := Branches{svc}.Delete(context.Background(), "feature", false)
	var unmerged *ErrNotFullyMerged
	if !errors.As(err, &unmerged) {
		t.Fatalf("got %v, want ErrNotFullyMerged", err)
	}
	if !strings.Contains(unmerged.Base, "origin/feature") && unmerged.Base != "origin/feature" {
		t.Fatalf("base = %q, want the upstream", unmerged.Base)
	}
}

// TestRenameBranchShouldKeepUpstream ports
// GitBranchWorkerTest.`test rename branch should keep upstream`.
func TestRenameBranchShouldKeepUpstream(t *testing.T) {
	svc, repo := newService(t)
	remote := testutil.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("remote", "add", "origin", remote.Dir)
	repo.Git("checkout", "-b", "feature")
	repo.Git("push", "-u", "origin", "feature")

	if err := (Branches{Service: svc}).Rename(context.Background(), "feature", "renamed", false); err != nil {
		t.Fatal(err)
	}
	if branchOf(t, repo) != "renamed" {
		t.Fatalf("branch = %q", branchOf(t, repo))
	}
	upstream := strings.TrimSpace(repo.Git("rev-parse", "--abbrev-ref", "renamed@{upstream}"))
	if upstream != "origin/feature" {
		t.Fatalf("upstream = %q", upstream)
	}
}

// TestRenameBranchUnsetUpstream ports
// GitBranchWorkerTest.`test rename branch unset upstream should remove upstream`.
func TestRenameBranchUnsetUpstream(t *testing.T) {
	svc, repo := newService(t)
	remote := testutil.NewRepo(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("remote", "add", "origin", remote.Dir)
	repo.Git("checkout", "-b", "feature")
	repo.Git("push", "-u", "origin", "feature")

	if err := (Branches{Service: svc}).Rename(context.Background(), "feature", "renamed", true); err != nil {
		t.Fatal(err)
	}
	out, err := repo.GitAllowError("rev-parse", "--abbrev-ref", "renamed@{upstream}")
	if err == nil {
		t.Fatalf("upstream still set: %s", out)
	}
}

// TestCheckoutBranchAlreadyCheckedOutInAnotherWorktree ports
// GitBranchWorkerTest.`test checkout branch already checked out in another worktree should confirm and retry`.
// The confirmation is the caller's: Checkout returns the typed error.
func TestCheckoutBranchAlreadyCheckedOutInAnotherWorktree(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.Git("branch", "feature")
	other := filepath.Join(filepath.Dir(repo.Dir), "other")
	repo.Git("worktree", "add", other, "feature")

	_, err := Branches{svc}.Checkout(context.Background(), "feature")
	var inUse *command.ErrBranchCheckedOutInWorktree
	if !errors.As(err, &inUse) {
		t.Fatalf("got %v, want ErrBranchCheckedOutInWorktree", err)
	}
	if inUse.Branch != "feature" {
		t.Fatalf("branch = %q", inUse.Branch)
	}
	if inUse.Path == "" {
		t.Fatal("empty worktree path")
	}
}
