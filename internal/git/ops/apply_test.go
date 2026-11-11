package ops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
)

// TestSimpleCherryPick ports
// GitCherryPickAutoCommitTest.`test simple cherry-pick`.
func TestSimpleCherryPick(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "feature\n")
	picked := repo.Commit("feature commit")
	repo.Git("checkout", "master")

	created, err := svc.CherryPick(context.Background(), []string{picked})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("created = %#v", created)
	}
	if fileText(t, repo, "a.txt") != "feature\n" {
		t.Fatalf("content = %q", fileText(t, repo, "a.txt"))
	}
	if strings.TrimSpace(repo.Git("log", "-1", "--format=%s")) != "feature commit" {
		t.Fatalf("subject = %q", repo.Git("log", "-1", "--format=%s"))
	}
}

// TestCherryPickTwoCommits ports
// GitCherryPickAutoCommitTest.`test cherry-pick 2 commits`.
func TestCherryPickTwoCommits(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "1\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "2\n")
	first := repo.Commit("first")
	repo.WriteFile("b.txt", "b\n")
	second := repo.Commit("second")
	repo.Git("checkout", "master")

	created, err := svc.CherryPick(context.Background(), []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created %d commits", len(created))
	}
	subjects := strings.TrimSpace(repo.Git("log", "-2", "--format=%s"))
	if subjects != "second\nfirst" {
		t.Fatalf("subjects = %q", subjects)
	}
}

// TestDirtyTreeConflictingWithCommit ports
// GitCherryPickAutoCommitTest.`test dirty tree conflicting with commit`.
func TestDirtyTreeConflictingWithCommit(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "feature\n")
	picked := repo.Commit("feature")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "dirty\n")

	_, err := svc.CherryPick(context.Background(), []string{picked})
	var overwritten *command.ErrLocalChangesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v, want local-changes error", err)
	}
}

// TestUntrackedFileConflictingWithCommit ports
// GitCherryPickAutoCommitTest.`test untracked file conflicting with commit`.
func TestUntrackedFileConflictingWithCommit(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("new.txt", "new\n")
	picked := repo.Commit("add new")
	repo.Git("checkout", "master")
	repo.WriteFile("new.txt", "untracked\n")

	_, err := svc.CherryPick(context.Background(), []string{picked})
	var overwritten *command.ErrUntrackedFilesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v, want untracked error", err)
	}
}

// TestConflictWithCherryPickedCommit ports
// GitCherryPickAutoCommitTest.`test conflict with cherry-picked commit should show merge dialog`.
func TestConflictWithCherryPickedCommit(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "feature\n")
	picked := repo.Commit("feature")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "master\n")
	repo.Commit("master")

	_, err := svc.CherryPick(context.Background(), []string{picked})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
	if _, statErr := repo.GitAllowError("rev-parse", "-q", "--verify", "CHERRY_PICK_HEAD"); statErr != nil {
		t.Fatal("expected CHERRY_PICK_HEAD after a conflict")
	}
}

// TestCherryPickThreeCommitsWhereSecondConflicts ports
// GitCherryPickAutoCommitTest.`test cherry-pick 3 commits, where second conflicts with master`.
// The first commit is applied; the second stops the sequence.
func TestCherryPickThreeCommitsWhereSecondConflicts(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("b.txt", "B\n")
	first := repo.Commit("touch b")
	repo.WriteFile("a.txt", "feature\n")
	second := repo.Commit("touch a")
	repo.WriteFile("c.txt", "c\n")
	third := repo.Commit("add c")
	repo.Git("checkout", "master")
	repo.WriteFile("a.txt", "master\n")
	repo.Commit("master a")

	created, err := svc.CherryPick(context.Background(), []string{first, second, third})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
	if len(created) != 1 {
		t.Fatalf("created = %d, want the first commit only", len(created))
	}
	if fileText(t, repo, "b.txt") != "B\n" {
		t.Fatalf("b.txt = %q", fileText(t, repo, "b.txt"))
	}
}

// TestSeveralCommitsOneAlreadyApplied ports
// GitCherryPickAutoCommitTest.`test several commits one of which have already been applied`.
func TestSeveralCommitsOneAlreadyApplied(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "1\n")
	repo.Commit("base")
	repo.Git("checkout", "-b", "feature")
	repo.WriteFile("a.txt", "2\n")
	first := repo.Commit("first")
	repo.WriteFile("b.txt", "b\n")
	second := repo.Commit("second")
	repo.Git("checkout", "master")
	if _, err := svc.CherryPick(context.Background(), []string{first}); err != nil {
		t.Fatal(err)
	}

	created, err := svc.CherryPick(context.Background(), []string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("created = %d, want only the new commit", len(created))
	}
	if strings.TrimSpace(repo.Git("log", "-1", "--format=%s")) != "second" {
		t.Fatalf("HEAD subject = %q", repo.Git("log", "-1", "--format=%s"))
	}
}

// TestSimpleRevert ports GitRevertTest.`test simple revert`.
func TestSimpleRevert(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.WriteFile("a.txt", "changed\n")
	target := repo.Commit("change")

	created, err := svc.Revert(context.Background(), []string{target})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("created = %#v", created)
	}
	if fileText(t, repo, "a.txt") != "base\n" {
		t.Fatalf("content = %q", fileText(t, repo, "a.txt"))
	}
	subject := strings.TrimSpace(repo.Git("log", "-1", "--format=%s"))
	if !strings.HasPrefix(subject, "Revert ") {
		t.Fatalf("subject = %q", subject)
	}
}

// TestRevertTwoCommits ports GitRevertTest.`test revert two commits`.
func TestRevertTwoCommits(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "1\n")
	repo.Commit("base")
	repo.WriteFile("a.txt", "2\n")
	first := repo.Commit("first")
	repo.WriteFile("b.txt", "b\n")
	second := repo.Commit("second")

	created, err := svc.Revert(context.Background(), []string{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created = %d", len(created))
	}
	if _, err := repo.GitAllowError("cat-file", "-e", "HEAD:b.txt"); err == nil {
		t.Fatal("b.txt still present")
	}
	if fileText(t, repo, "a.txt") != "1\n" {
		t.Fatalf("a.txt = %q", fileText(t, repo, "a.txt"))
	}
}

// TestLocalChangesWouldBeOverwrittenByRevert ports
// GitRevertTest.`test local changes would be overwritten by revert`.
func TestLocalChangesWouldBeOverwrittenByRevert(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.WriteFile("a.txt", "changed\n")
	target := repo.Commit("change")
	repo.WriteFile("a.txt", "dirty\n")

	_, err := svc.Revert(context.Background(), []string{target})
	var overwritten *command.ErrLocalChangesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v, want local-changes error", err)
	}
}

// TestEmptyRevert ports GitRevertTest.`test empty revert`: reverting a
// commit whose change is already gone is skipped.
func TestEmptyRevert(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("base")
	repo.WriteFile("a.txt", "changed\n")
	target := repo.Commit("change")
	if _, err := svc.Revert(context.Background(), []string{target}); err != nil {
		t.Fatal(err)
	}
	before := headOf(t, repo)

	created, err := svc.Revert(context.Background(), []string{target})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 {
		t.Fatalf("empty revert created %d commits", len(created))
	}
	if headOf(t, repo) != before {
		t.Fatal("HEAD moved on an empty revert")
	}
}

// TestOneCommitRevertedSecondFails ports
// GitRevertTest.`test one commit reverted, second fails with error`.
func TestOneCommitRevertedSecondFails(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("base")
	repo.WriteFile("b.txt", "B\n")
	first := repo.Commit("touch b")
	repo.WriteFile("a.txt", "changed\n")
	second := repo.Commit("touch a")
	repo.WriteFile("a.txt", "dirty\n")

	// Revert newest first. The second commit touches a.txt, which is dirty,
	// so it fails before the first commit is reverted.
	created, err := svc.Revert(context.Background(), []string{second, first})
	if err == nil {
		t.Fatal("expected the second revert to fail")
	}
	if len(created) != 0 {
		t.Fatalf("created = %d, want none", len(created))
	}
}

// TestCreateAndDeleteTag ports GitImpl.createNewTag and deleteTag.
func TestCreateAndDeleteTag(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "b\n")
	repo.Commit("second")

	got, err := Tag{svc}.Create(context.Background(), "v1", first)
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatalf("tag points at %s, want %s", got, first)
	}
	if _, err := (Tag{Service: svc}).Create(context.Background(), "v2", ""); err != nil {
		t.Fatal(err)
	}
	head := headOf(t, repo)
	points := strings.TrimSpace(repo.Git("rev-parse", "v2^{commit}"))
	if points != head {
		t.Fatalf("v2 points at %s, want %s", points, head)
	}
	if err := (Tag{Service: svc}).Delete(context.Background(), "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GitAllowError("rev-parse", "v1"); err == nil {
		t.Fatal("v1 still exists")
	}
}
