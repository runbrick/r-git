package ops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
)

// TestStashSaveAndList ports GitStashTest.`test list stashes` for the
// save side: a stash is created with the given message.
func TestStashSaveAndList(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "dirty\n")

	hash, err := svc.SaveStash(context.Background(), StashSaveOptions{Message: "wip"})
	if err != nil {
		t.Fatal(err)
	}
	if hash.IsZero() {
		t.Fatal("zero stash hash")
	}
	list := repo.Git("stash", "list")
	if !strings.Contains(list, "wip") {
		t.Fatalf("stash list = %q", list)
	}
	if fileText(t, repo, "a.txt") != "a\n" {
		t.Fatalf("work tree = %q", fileText(t, repo, "a.txt"))
	}
}

// TestStashIncludeUntracked ports createStashHandler(--include-untracked).
func TestStashIncludeUntracked(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("new.txt", "new\n")

	if _, err := svc.SaveStash(context.Background(), StashSaveOptions{Message: "with new", IncludeUntracked: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GitAllowError("cat-file", "-e", "HEAD:new.txt"); err == nil {
		t.Fatal("untracked file was committed")
	}
	if strings.Contains(repo.Git("status", "--porcelain"), "new.txt") {
		t.Fatalf("untracked file was not stashed: %s", repo.Git("status", "--porcelain"))
	}
	patch, err := svc.ShowStash(context.Background(), "stash@{0}")
	if err != nil {
		t.Fatal(err)
	}
	// git stash show -p shows the tracked parent diff. The untracked
	// commit is the third parent; confirm the file is gone from the tree
	// and comes back on pop.
	_ = patch
	if err := svc.PopStash(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if fileText(t, repo, "new.txt") != "new\n" {
		t.Fatalf("new.txt = %q", fileText(t, repo, "new.txt"))
	}
}

// TestStashKeepIndex ports createStashHandler(--keep-index): staged
// changes stay in the work tree and the index.
func TestStashKeepIndex(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "A\n")
	repo.Git("add", "a.txt")
	repo.WriteFile("b.txt", "B\n")

	if _, err := svc.SaveStash(context.Background(), StashSaveOptions{Message: "keep", KeepIndex: true}); err != nil {
		t.Fatal(err)
	}
	if fileText(t, repo, "a.txt") != "A\n" {
		t.Fatalf("staged content not kept: %q", fileText(t, repo, "a.txt"))
	}
	if fileText(t, repo, "b.txt") != "b\n" {
		t.Fatalf("unstaged content not stashed: %q", fileText(t, repo, "b.txt"))
	}
}

// TestStashPopAndApply ports GitStashOperations unstash with pop and apply.
func TestStashPopAndApply(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "one\n")
	if _, err := svc.SaveStash(context.Background(), StashSaveOptions{Message: "one"}); err != nil {
		t.Fatal(err)
	}
	repo.WriteFile("a.txt", "two\n")
	if _, err := svc.SaveStash(context.Background(), StashSaveOptions{Message: "two"}); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyStash(context.Background(), "stash@{1}"); err != nil {
		t.Fatal(err)
	}
	if fileText(t, repo, "a.txt") != "one\n" {
		t.Fatalf("after apply = %q", fileText(t, repo, "a.txt"))
	}
	if !strings.Contains(repo.Git("stash", "list"), "one") {
		t.Fatal("apply dropped the stash")
	}

	repo.WriteFile("a.txt", "a\n")
	if err := svc.PopStash(context.Background(), "stash@{0}"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(repo.Git("stash", "list"), "two") {
		t.Fatalf("pop left the stash: %s", repo.Git("stash", "list"))
	}
}

// TestStashDropAndClear ports GitStashOperations.dropStash and clearStashes.
func TestStashDropAndClear(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "one\n")
	svc.SaveStash(context.Background(), StashSaveOptions{Message: "one"})
	repo.WriteFile("a.txt", "two\n")
	svc.SaveStash(context.Background(), StashSaveOptions{Message: "two"})

	if err := svc.DropStash(context.Background(), "stash@{1}"); err != nil {
		t.Fatal(err)
	}
	list := repo.Git("stash", "list")
	if strings.Contains(list, "one") || !strings.Contains(list, "two") {
		t.Fatalf("after drop: %q", list)
	}
	if err := svc.ClearStashes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(repo.Git("stash", "list")) != "" {
		t.Fatalf("after clear: %q", repo.Git("stash", "list"))
	}
}

// TestStashPopConflict ports the unstash conflict detector.
func TestStashPopConflict(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "stashed\n")
	if _, err := svc.SaveStash(context.Background(), StashSaveOptions{Message: "s"}); err != nil {
		t.Fatal(err)
	}
	repo.WriteFile("a.txt", "other\n")
	repo.Git("add", "a.txt")
	repo.Git("commit", "-q", "-m", "other")

	err := svc.PopStash(context.Background(), "")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}

// TestFileIsRefreshedOnHardReset ports
// GitResetTest.`test file is refreshed on hard reset`.
func TestFileIsRefreshedOnHardReset(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "second\n")
	repo.Commit("second")

	hash, err := svc.Reset(context.Background(), ResetHard, first)
	if err != nil {
		t.Fatal(err)
	}
	if hash.String() != first {
		t.Fatalf("HEAD = %s, want %s", hash, first)
	}
	if fileText(t, repo, "a.txt") != "base\n" {
		t.Fatalf("content = %q", fileText(t, repo, "a.txt"))
	}
}

// TestFileStatusIsRefreshedOnSoftReset ports
// GitResetTest.`test file status is refreshed on soft reset`.
func TestFileStatusIsRefreshedOnSoftReset(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "second\n")
	repo.Commit("second")

	if _, err := svc.Reset(context.Background(), ResetSoft, first); err != nil {
		t.Fatal(err)
	}
	if fileText(t, repo, "a.txt") != "second\n" {
		t.Fatalf("work tree = %q", fileText(t, repo, "a.txt"))
	}
	cached := strings.TrimSpace(repo.Git("diff", "--cached", "--name-only"))
	if cached != "a.txt" {
		t.Fatalf("staged = %q", cached)
	}
}

// TestResetMixed ports git reset --mixed: the index matches the target and
// the work tree keeps the later content as an unstaged change.
func TestResetMixed(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "second\n")
	repo.Commit("second")

	if _, err := svc.Reset(context.Background(), ResetMixed, first); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(repo.Git("diff", "--cached", "--name-only")) != "" {
		t.Fatal("mixed reset left a staged change")
	}
	if fileText(t, repo, "a.txt") != "second\n" {
		t.Fatalf("work tree = %q", fileText(t, repo, "a.txt"))
	}
}

// TestResetKeepRefusesLocalChanges ports the local-changes detector on
// git reset --keep.
func TestResetKeepRefusesLocalChanges(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "base\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "second\n")
	repo.Commit("second")
	repo.WriteFile("a.txt", "dirty\n")

	_, err := svc.Reset(context.Background(), ResetKeep, first)
	var overwritten *command.ErrLocalChangesWouldBeOverwritten
	if !errors.As(err, &overwritten) {
		t.Fatalf("got %v, want local-changes error", err)
	}
	if headOf(t, repo) == first {
		t.Fatal("reset moved HEAD")
	}
}
