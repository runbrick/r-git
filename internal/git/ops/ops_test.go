package ops

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/testutil"
)

func TestMain(m *testing.M) {
	if handled, exitCode := RunEditorIfRequested(); handled {
		os.Exit(exitCode)
	}
	os.Exit(m.Run())
}

func testOps(t *testing.T) (*testutil.Repo, *Service) {
	t.Helper()
	repo := testutil.NewRepo(t)
	runner, err := command.New()
	if err != nil {
		t.Fatal(err)
	}
	return repo, New(runner, repo.Dir)
}

func TestApplyTodoModelFixupCannotBeFirst(t *testing.T) {
	_, err := ApplyTodoModel([]RebaseEntry{{Action: "fixup", Commit: "abc"}})
	if !errors.Is(err, ErrInvalidTodo) {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyTodoModelAllowsFixupAfterPick(t *testing.T) {
	want := []RebaseEntry{{Action: "pick", Commit: "abc", Subject: "first"}, {Action: "fixup", Commit: "def"}}
	got, err := ApplyTodoModel(want)
	if err != nil || len(got) != 2 || got[1].Action != "fixup" {
		t.Fatalf("got %#v, err %v", got, err)
	}
}

func TestParseTodoAndWriteTodo(t *testing.T) {
	entries, err := ParseTodo([]byte("# comment\npick abc First commit\nfixup def\n"))
	if err != nil || len(entries) != 2 || entries[0].Subject != "First commit" {
		t.Fatalf("entries %#v, err %v", entries, err)
	}
	path := t.TempDir() + "/todo"
	if err := WriteTodo(path, entries); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "pick abc First commit\nfixup def\n" {
		t.Fatalf("todo %q, err %v", data, err)
	}
}

func TestGitMergerConflictListingVersionsAndResolution(t *testing.T) {
	repo, ops := testOps(t)
	repo.WriteFile("file.txt", "base\n")
	repo.Commit("base")
	repo.Git("checkout", "-qb", "side")
	repo.WriteFile("file.txt", "side\n")
	repo.Commit("side")
	repo.Git("checkout", "-q", "master")
	repo.WriteFile("file.txt", "main\n")
	repo.Commit("main")
	result := ops.Merge(context.Background(), "side", MergeOptions{})
	if result.Outcome != OutcomeConflicts || len(result.Conflicts) != 1 || result.Conflicts[0].Path != "file.txt" {
		t.Fatalf("merge result %#v", result)
	}
	base, ours, theirs, err := ops.ReadVersions(context.Background(), "file.txt")
	if err != nil || string(base) != "base\n" || string(ours) != "main\n" || string(theirs) != "side\n" {
		t.Fatalf("versions %q %q %q err %v", base, ours, theirs, err)
	}
	if err := ops.Resolve(context.Background(), "file.txt", []byte("resolved\n")); err != nil {
		t.Fatal(err)
	}
	conflicts, err := ops.Conflicts(context.Background())
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("remaining %#v err %v", conflicts, err)
	}
	if got := ops.ContinueMerge(context.Background(), "resolved"); got.Outcome != OutcomeDone {
		t.Fatalf("continue %#v", got)
	}
}

func TestGitRebaserConflictContinueAndAbort(t *testing.T) {
	repo, ops := testOps(t)
	repo.WriteFile("file.txt", "base\n")
	repo.Commit("base")
	repo.Git("checkout", "-qb", "side")
	repo.WriteFile("file.txt", "side\n")
	repo.Commit("side")
	repo.Git("checkout", "-q", "master")
	repo.WriteFile("file.txt", "main\n")
	repo.Commit("main")
	result := ops.Rebase(context.Background(), "side", RebaseOptions{})
	if result.Outcome != OutcomeConflicts || len(result.Conflicts) != 1 {
		t.Fatalf("rebase %#v", result)
	}
	if err := ops.AcceptOurs(context.Background(), "file.txt"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(repo.Dir + "/file.txt")
	if err != nil || string(content) != "main\n" {
		t.Fatalf("rebase ours content %q, err %v", content, err)
	}
	if got := ops.ContinueRebase(context.Background()); got.Outcome != OutcomeDone {
		t.Fatalf("continue %#v", got)
	}
}

func TestHandleSequenceEditor(t *testing.T) {
	path := t.TempDir() + "/todo"
	if err := os.WriteFile(path, []byte("pick abc one\npick def two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := HandleSequenceEditor(context.Background(), path, func(entries []RebaseEntry) ([]RebaseEntry, error) {
		entries[0].Action = "drop"
		return entries, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "drop abc one\npick def two\n" {
		t.Fatalf("got %q, err %v", data, err)
	}
}

func TestInteractiveRebaseSquash(t *testing.T) {
	repo, ops := interactiveRepo(t)
	result := ops.InteractiveRebase(context.Background(), "HEAD~2", func(entries []RebaseEntry) ([]RebaseEntry, error) {
		entries[1].Action = "squash"
		return entries, nil
	})
	if result.Outcome != OutcomeDone {
		t.Fatalf("interactive rebase: %#v", result)
	}
	if got := strings.TrimSpace(repo.Git("rev-list", "--count", "HEAD")); got != "2" {
		t.Fatalf("commit count = %s, want 2 including base", got)
	}
}

func TestInteractiveRebaseDrop(t *testing.T) {
	repo, ops := interactiveRepo(t)
	result := ops.InteractiveRebase(context.Background(), "HEAD~2", func(entries []RebaseEntry) ([]RebaseEntry, error) {
		entries[0].Action = "drop"
		return entries, nil
	})
	if result.Outcome != OutcomeDone {
		t.Fatalf("interactive rebase: %#v", result)
	}
	if got := strings.TrimSpace(repo.Git("rev-list", "--count", "HEAD")); got != "2" {
		t.Fatalf("commit count = %s, want 2 including base", got)
	}
	if _, err := os.Stat(repo.Dir + "/first.txt"); !os.IsNotExist(err) {
		t.Fatalf("dropped file still exists or stat failed: %v", err)
	}
}

func TestInteractiveRebaseReorder(t *testing.T) {
	repo, ops := interactiveRepo(t)
	result := ops.InteractiveRebase(context.Background(), "HEAD~2", func(entries []RebaseEntry) ([]RebaseEntry, error) {
		entries[0], entries[1] = entries[1], entries[0]
		return entries, nil
	})
	if result.Outcome != OutcomeDone {
		t.Fatalf("interactive rebase: %#v", result)
	}
	if got := strings.Split(strings.TrimSpace(repo.Git("log", "-2", "--format=%s")), "\n"); len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("reordered log = %#v", got)
	}
}

func TestInteractiveRebaseReword(t *testing.T) {
	repo, ops := interactiveRepo(t)
	result := ops.InteractiveRebase(context.Background(), "HEAD~2", func(entries []RebaseEntry) ([]RebaseEntry, error) {
		entries[0].Action = "reword"
		entries[0].Subject = "renamed first"
		return entries, nil
	})
	if result.Outcome != OutcomeDone {
		t.Fatalf("interactive rebase: %#v", result)
	}
	if got := repo.Git("log", "-2", "--format=%s"); !strings.Contains(got, "renamed first") {
		t.Fatalf("reworded log = %q", got)
	}
}

func TestInteractiveRebaseFixupFirstDoesNotChangeRepo(t *testing.T) {
	repo, ops := interactiveRepo(t)
	head := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	result := ops.InteractiveRebase(context.Background(), "HEAD~2", func(entries []RebaseEntry) ([]RebaseEntry, error) {
		entries[0].Action = "fixup"
		return entries, nil
	})
	if result.Outcome != OutcomeError || !errors.Is(result.Err, ErrInvalidTodo) {
		t.Fatalf("interactive rebase = %#v, want invalid todo", result)
	}
	if got := strings.TrimSpace(repo.Git("rev-parse", "HEAD")); got != head {
		t.Fatalf("HEAD changed from %s to %s", head, got)
	}
	if got := strings.TrimSpace(repo.Git("status", "--porcelain")); got != "" {
		t.Fatalf("worktree changed: %q", got)
	}
}

func interactiveRepo(t *testing.T) (*testutil.Repo, *Service) {
	t.Helper()
	repo, ops := testOps(t)
	repo.WriteFile("base.txt", "base\n")
	repo.Commit("base")
	repo.WriteFile("first.txt", "first\n")
	repo.Commit("first")
	repo.WriteFile("second.txt", "second\n")
	repo.Commit("second")
	return repo, ops
}
