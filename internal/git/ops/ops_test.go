package ops

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/testutil"
)

func testOps(t *testing.T) (*testutil.Repo, Operations) {
	t.Helper()
	repo := testutil.NewRepo(t)
	runner, err := command.New()
	if err != nil {
		t.Fatal(err)
	}
	return repo, Operations{Dir: repo.Dir, Git: runner}
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
