package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/testutil"
)

func TestBuiltBinaryRunsInteractiveRebaseEditor(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "r-git")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build r-git: %v\n%s", err, output)
	}

	repo := testutil.NewRepo(t)
	repo.WriteFile("base.txt", "base\n")
	repo.Commit("base")
	repo.WriteFile("one.txt", "one\n")
	repo.Commit("first commit")
	repo.WriteFile("two.txt", "two\n")
	repo.Commit("second commit")

	runner, err := command.New()
	if err != nil {
		t.Fatal(err)
	}
	operations := ops.Operations{Dir: repo.Dir, Git: runner, Editor: binary}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan ops.RebaseResult, 1)
	go func() {
		done <- operations.InteractiveRebase(ctx, "HEAD~2", func(entries []ops.RebaseEntry) ([]ops.RebaseEntry, error) {
			if len(entries) != 2 {
				t.Errorf("todo has %d entries, want 2", len(entries))
				return entries, nil
			}
			entries[1].Action = "squash"
			return entries, nil
		})
	}()

	var result ops.RebaseResult
	select {
	case result = <-done:
	case <-ctx.Done():
		t.Fatalf("interactive rebase did not finish using built editor binary: %v", ctx.Err())
	}
	if result.Outcome != ops.OutcomeDone {
		t.Fatalf("rebase outcome = %v, error = %v", result.Outcome, result.Err)
	}
	if count := strings.TrimSpace(repo.Git("rev-list", "--count", "HEAD")); count != "2" {
		t.Fatalf("commit count = %s, want 2", count)
	}
	message := repo.Git("log", "-1", "--format=%B")
	if !strings.Contains(message, "first commit") || !strings.Contains(message, "second commit") {
		t.Fatalf("squash message = %q", message)
	}
}
