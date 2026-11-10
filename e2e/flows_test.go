//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenRepoShowsLogGraphAndBranches(t *testing.T) {
	f := newFixture(t)
	f.write("base.txt", "base\n")
	f.commit(t, "initial history entry")
	f.git(t, "branch", "topic")
	f.write("base.txt", "updated\n")
	f.commit(t, "second history entry")

	a := f.open(t)
	a.waitFor("initial history entry")
	a.waitFor("second history entry")
	a.waitFor("topic")
	texts := a.textDump()
	for _, needle := range []string{"Graph", "Subject", "Author", "Hash", "topic"} {
		if !strings.Contains(texts, needle) {
			t.Fatalf("log missing %q\n%s", needle, texts)
		}
	}
}

func TestEditStageCommitAppearsInLog(t *testing.T) {
	f := newFixture(t)
	f.write("base.txt", "base\n")
	f.commit(t, "initial history entry")

	a := f.open(t)
	a.waitFor("initial history entry")

	f.write("notes.txt", "edited from the test\n")
	a.click("Refresh")
	a.click("Commit")
	a.waitFor("notes.txt")

	a.click("Stage")
	a.waitFor("Staged")
	a.click("textfield:Commit message")
	a.typeText("record the notes")
	a.clickLowest("Commit")
	a.waitFor("No changes")

	log := f.git(t, "log", "--oneline")
	if !strings.Contains(log, "record the notes") {
		t.Fatalf("commit missing from git log:\n%s", log)
	}
	if status := f.git(t, "status", "--porcelain"); status != "" {
		t.Fatalf("working tree not clean:\n%s", status)
	}
	a.click("Commit")
	a.waitFor("No changes")
}

func TestCreateBranchCommitAndPush(t *testing.T) {
	f := newFixture(t)
	f.write("base.txt", "base\n")
	f.commit(t, "initial history entry")
	main := f.addBareRemote(t)

	a := f.open(t)
	a.waitFor(main + " (current)")
	a.click(main + " (current)")
	a.click("Branch actions")
	a.click("New branch from")
	a.waitFor("Create branch")
	a.click("textfield:Branch name")
	a.typeText("topic")
	a.click("Create branch")
	a.waitFor("topic (current)")

	f.write("topic.txt", "on topic\n")
	a.click("Refresh")
	a.click("Commit")
	a.waitFor("topic.txt")
	a.click("Stage")
	a.waitFor("Staged")
	a.click("textfield:Commit message")
	a.typeText("topic work")
	a.clickLowest("Commit")
	a.waitFor("No changes")

	a.clickToolbar("Push")
	a.waitFor("Push commits")
	a.click("checkbox:topic")
	a.click("Push commits")
	a.waitFor("Pushed")

	remote := f.git(t, "--git-dir", f.bare, "branch")
	if !strings.Contains(remote, "topic") {
		t.Fatalf("remote branches:\n%s", remote)
	}
	tip := strings.TrimSpace(f.git(t, "rev-parse", "topic"))
	remoteTip := strings.TrimSpace(f.git(t, "--git-dir", f.bare, "rev-parse", "topic"))
	if tip != remoteTip {
		t.Fatalf("remote topic = %s, local = %s", remoteTip, tip)
	}
}

func TestPullRebaseShowsLinearHistory(t *testing.T) {
	f := newFixture(t)
	f.write("base.txt", "base\n")
	f.commit(t, "initial history entry")
	main := f.addBareRemote(t)

	// A commit that exists only on the bare remote.
	clone := filepath.Join(f.root, "other")
	f.git(t, "clone", "-q", f.bare, clone)
	f.gitIn(t, clone, "config", "user.name", "Test User")
	f.gitIn(t, clone, "config", "user.email", "test@example.invalid")
	writeFile(t, filepath.Join(clone, "remote.txt"), "from remote\n")
	f.gitIn(t, clone, "add", "-A")
	f.gitIn(t, clone, "commit", "-q", "-m", "remote only")
	f.gitIn(t, clone, "push", "-q", "origin", main)

	a := f.open(t)
	a.waitFor("initial history entry")
	a.clickToolbar("Pull")
	a.waitFor("Pull")
	a.click("Rebase")
	a.click("Pull now")
	a.waitFor("remote only")

	log := f.git(t, "log", "--format=%s")
	if strings.Contains(log, "Merge") {
		t.Fatalf("history is not linear:\n%s", log)
	}
	parents := strings.TrimSpace(f.git(t, "rev-list", "--max-parents=0", "HEAD"))
	count := strings.TrimSpace(f.git(t, "rev-list", "--count", "HEAD"))
	if count != "2" {
		t.Fatalf("commit count %s, log:\n%s\nroots:\n%s", count, log, parents)
	}
	if !a.contains("remote only") {
		t.Fatalf("log view missing remote commit:\n%s", strings.Join(a.texts(""), "\n"))
	}
}

func TestMergeConflictResolveAndCommit(t *testing.T) {
	f := newFixture(t)
	base := "first\nkeep 1\nkeep 2\nkeep 3\nkeep 4\nlast\n"
	f.write("conflict.txt", base)
	f.commit(t, "base")
	main := strings.TrimSpace(f.git(t, "branch", "--show-current"))
	f.git(t, "checkout", "-q", "-b", "side")
	f.write("conflict.txt", strings.ReplaceAll(strings.ReplaceAll(base, "first", "theirs first"), "last", "theirs last"))
	f.commit(t, "side edit")
	f.git(t, "checkout", "-q", main)
	f.write("conflict.txt", strings.ReplaceAll(strings.ReplaceAll(base, "first", "ours first"), "last", "ours last"))
	f.commit(t, "ours edit")

	a := f.open(t)
	a.waitFor("ours edit")
	a.click("side")
	a.click("Branch actions")
	a.click("Merge into current")
	a.waitFor("Merge conflicts in side")
	a.clickToolbar("Conflicts")
	a.waitFor("Close conflicts")
	a.clickWhenEnabled("conflict.txt")
	a.clickWhenEnabled("Accept Theirs")
	a.clickWhenEnabled("Mark resolved")
	a.waitFor("Select a conflicted file")
	a.click("Close conflicts")
	a.clickToolbar("Commit")
	a.waitFor("Describe the change")

	a.click("textfield:Commit message")
	a.typeText("resolve the conflict")
	before := strings.TrimSpace(f.git(t, "rev-parse", "HEAD"))
	a.clickLowest("Commit")
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && strings.TrimSpace(f.git(t, "rev-parse", "HEAD")) == before {
		time.Sleep(100 * time.Millisecond)
	}

	got := f.read(t, "conflict.txt")
	want := "theirs first\nkeep 1\nkeep 2\nkeep 3\nkeep 4\ntheirs last\n"
	if got != want {
		t.Fatalf("resolved content:\n%s\nwant:\n%s", got, want)
	}
	if status := f.git(t, "status", "--porcelain"); status != "" {
		t.Fatalf("still conflicted:\n%s\nlog:\n%s", status, f.git(t, "log", "--oneline", "--stat", "-2"))
	}
}

func TestStashAndPopAroundCheckout(t *testing.T) {
	f := newFixture(t)
	f.write("base.txt", "base\n")
	f.commit(t, "initial history entry")
	main := strings.TrimSpace(f.git(t, "branch", "--show-current"))
	f.git(t, "checkout", "-q", "-b", "other")
	f.write("other.txt", "other line\n")
	f.commit(t, "other side")
	f.git(t, "checkout", "-q", main)
	f.write("base.txt", "wip\n")

	a := f.open(t)
	a.waitFor("initial history entry")
	a.click("Open stash")
	a.waitFor("Save stash")
	a.click("textfield:Message")
	a.typeText("keep this")
	a.clickLowest("Save stash")
	a.waitFor("stash@")
	if body := f.read(t, "base.txt"); body != "base\n" {
		t.Fatalf("file after stash = %q\n%s", body, a.textDump())
	}

	f.git(t, "checkout", "-q", "other")
	f.git(t, "checkout", "-q", main)
	a.clickLowest("Pop")
	a.waitFor("Pop now")
	a.clickLowest("Pop now")
	a.waitFor("No stashes.")

	if body := f.read(t, "base.txt"); body != "wip\n" {
		t.Fatalf("file after pop = %q\n%s", body, a.textDump())
	}
}

func TestInteractiveRebaseSquash(t *testing.T) {
	f := newFixture(t)
	f.write("base.txt", "base\n")
	f.commit(t, "base commit")
	f.write("one.txt", "one\n")
	f.commit(t, "first commit")
	f.write("two.txt", "two\n")
	f.commit(t, "second commit")

	a := f.open(t)
	a.waitFor("second commit")
	a.click("Interactive rebase")
	a.waitFor("Load commits")
	a.click("Load commits")
	a.waitFor("Move up 2")
	a.click("combobox:Action 2")
	a.clickLowest("squash")
	a.click("Start rebase")
	a.waitFor("2 commits")

	count := strings.TrimSpace(f.git(t, "rev-list", "--count", "HEAD"))
	if count != "2" {
		t.Fatalf("commit count %s want 2\n%s", count, f.git(t, "log", "--oneline"))
	}
	message := f.git(t, "log", "-1", "--format=%B")
	if !strings.Contains(message, "first commit") || !strings.Contains(message, "second commit") {
		t.Fatalf("squash message:\n%s", message)
	}
	if status := f.git(t, "status", "--porcelain"); status != "" {
		t.Fatalf("dirty after rebase:\n%s", status)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
