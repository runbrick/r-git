package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/runbrick/r-git/internal/testutil"
	commitview "github.com/runbrick/r-git/internal/ui/commit"
)

func TestPartialCommitKeepsUncheckedStagedHunk(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("a.txt", "line1\nline2\n")
	g.WriteFile("b.txt", "b\n")
	g.Commit("initial")
	patch := "diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -1,2 +1,2 @@\n" +
		"-line1\n" +
		"+staged\n" +
		" line2\n"
	patchPath := filepath.Join(g.Dir, "hunk.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	g.Git("apply", "--cached", patchPath)
	g.WriteFile("a.txt", "unstaged\nline2\n")
	g.WriteFile("b.txt", "b changed\n")
	g.Git("add", "b.txt")

	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
	clickControl(t, driver, "Commit")
	waitFor(t, driver, func() bool { return groupOf(m, "b.txt") == repo.GroupStaged && groupOf(m, "a.txt") == repo.GroupStaged })
	m.commit.SetIncluded(repo.GroupStaged, "a.txt", false)
	m.commit.SetIncluded(repo.GroupUnstaged, "a.txt", false)
	m.commit.Message = "commit only b"
	m.handleCommitRequest(commitview.Request{Kind: commitview.RequestCommit, Paths: []string{"b.txt"}, Message: "commit only b"})
	m.workers.Wait()
	m.applyPending()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if m.commit.Error != "" {
		t.Fatal(m.commit.Error)
	}
	if strings.TrimSpace(g.Git("log", "-1", "--format=%s")) != "commit only b" {
		t.Fatalf("subject = %q", g.Git("log", "-1", "--format=%s"))
	}
	committed := strings.TrimSpace(g.Git("show", "--name-only", "--format=", "HEAD"))
	if committed != "b.txt" {
		t.Fatalf("commit files = %q", committed)
	}
	cached := strings.TrimSpace(g.Git("diff", "--cached", "--", "a.txt"))
	if !strings.Contains(cached, "+staged") {
		t.Fatalf("staged hunk lost:\n%s", cached)
	}
}

func TestFailedCommitLeavesIndexUnchanged(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("a.txt", "a\n")
	g.WriteFile("b.txt", "b\n")
	g.Commit("initial")
	hookDir := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookDir, "commit-msg"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g.Git("config", "core.hooksPath", filepath.ToSlash(hookDir))
	g.WriteFile("a.txt", "a staged\n")
	g.Git("add", "a.txt")
	g.WriteFile("b.txt", "b staged\n")
	g.Git("add", "b.txt")
	before := strings.TrimSpace(g.Git("diff", "--cached", "--name-only"))
	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
	clickControl(t, driver, "Commit")
	waitFor(t, driver, func() bool { return groupOf(m, "b.txt") == repo.GroupStaged })
	m.commit.SetIncluded(repo.GroupStaged, "a.txt", false)
	m.handleCommitRequest(commitview.Request{Kind: commitview.RequestCommit, Paths: []string{"b.txt"}, Message: "should fail"})
	m.workers.Wait()
	m.applyPending()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if m.commit.Error == "" {
		t.Fatal("failing hook did not surface an error")
	}
	if strings.TrimSpace(g.Git("log", "-1", "--format=%s")) != "initial" {
		t.Fatal("failed commit created a commit")
	}
	after := strings.TrimSpace(g.Git("diff", "--cached", "--name-only"))
	if after != before {
		t.Fatalf("index changed from %q to %q", before, after)
	}
}

func TestCommitAndPushRunsOnWake(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("a.txt", "a\n")
	g.Commit("initial")
	g.WriteFile("a.txt", "b\n")
	g.Git("add", "a.txt")
	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
	pushes := 0
	m.OnPush = func() { pushes++ }
	m.handleCommitRequest(commitview.Request{Kind: commitview.RequestCommit, Paths: []string{"a.txt"}, Message: "then push", Push: true})
	m.workers.Wait()
	if pushes != 0 {
		t.Fatal("OnPush ran on the worker")
	}
	m.applyPending()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if pushes != 1 {
		t.Fatalf("OnPush calls = %d", pushes)
	}
}
