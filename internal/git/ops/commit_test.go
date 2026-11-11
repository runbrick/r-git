package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAddOneFile ports GitAddTest.`test add one file`.
func TestAddOneFile(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("b.txt", "b\n")

	if err := svc.Stage(context.Background(), []string{"b.txt"}); err != nil {
		t.Fatal(err)
	}
	staged := repo.Git("diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) != "b.txt" {
		t.Fatalf("staged = %q", staged)
	}
}

// TestAddDirectory ports GitAddTest.`test add directory`.
func TestAddDirectory(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("dir/one.txt", "1\n")
	repo.WriteFile("dir/two.txt", "2\n")

	if err := svc.Stage(context.Background(), []string{"dir"}); err != nil {
		t.Fatal(err)
	}
	staged := strings.TrimSpace(repo.Git("diff", "--cached", "--name-only"))
	if !strings.Contains(staged, "dir/one.txt") || !strings.Contains(staged, "dir/two.txt") {
		t.Fatalf("staged = %q", staged)
	}
}

// TestUnstage ports the staging-area reset operation (GitResetOperation in
// index/actions/StagingAreaOperation.kt): a staged change leaves the index.
func TestUnstage(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "changed\n")
	repo.Git("add", "a.txt")

	if err := svc.Unstage(context.Background(), []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	cached := strings.TrimSpace(repo.Git("diff", "--cached", "--name-only"))
	if cached != "" {
		t.Fatalf("still staged: %q", cached)
	}
	if fileText(t, repo, "a.txt") != "changed\n" {
		t.Fatalf("work tree changed: %q", fileText(t, repo, "a.txt"))
	}
}

// TestDiscardTrackedChanges ports the staging-area revert of an unstaged
// modification (GitRevertOperation).
func TestDiscardTrackedChanges(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "dirty\n")

	if err := svc.Discard(context.Background(), []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	if fileText(t, repo, "a.txt") != "a\n" {
		t.Fatalf("content = %q", fileText(t, repo, "a.txt"))
	}
}

// TestDiscardUntrackedRemovesTheFile ports the untracked removal rule:
// discarding an untracked path deletes it, and an ignored path is kept.
func TestDiscardUntrackedRemovesTheFile(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.WriteFile(".gitignore", "ignored.txt\n")
	repo.Commit("init")
	repo.WriteFile("new.txt", "new\n")
	repo.WriteFile("ignored.txt", "keep\n")

	if err := svc.Discard(context.Background(), []string{"new.txt", "ignored.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file still present: %v", err)
	}
	if fileText(t, repo, "ignored.txt") != "keep\n" {
		t.Fatalf("ignored file = %q", fileText(t, repo, "ignored.txt"))
	}
}

// TestDiscardAddedFile ports discarding a newly staged file: it leaves
// both the index and the work tree.
func TestDiscardAddedFile(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("new.txt", "new\n")
	repo.Git("add", "new.txt")

	if err := svc.Discard(context.Background(), []string{"new.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("added file still present: %v", err)
	}
	cached := strings.TrimSpace(repo.Git("diff", "--cached", "--name-only"))
	if cached != "" {
		t.Fatalf("still staged: %q", cached)
	}
}

// TestCommit ports GitRepositoryCommitter.commitStaged: the message goes
// through a file (-F) and the new hash is returned.
func TestCommit(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "b\n")
	repo.Git("add", "a.txt")

	msg := []byte("subject line\n\nbody with exact bytes\n")
	hash, err := svc.Commit(context.Background(), CommitOptions{Message: msg})
	if err != nil {
		t.Fatal(err)
	}
	if hash.String() != headOf(t, repo) {
		t.Fatalf("hash = %s, HEAD = %s", hash, headOf(t, repo))
	}
	got := repo.Git("log", "-1", "--format=%B")
	// git log adds a trailing newline after %B. The recorded message is
	// the bytes passed through -F.
	if strings.TrimRight(got, "\n") != strings.TrimRight(string(msg), "\n") {
		t.Fatalf("message = %q, want %q", got, msg)
	}
}

// TestCommitPartialPaths commits only the selected file. The other staged
// change stays staged, matching a partial commit of selected files.
func TestCommitPartialPaths(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.WriteFile("b.txt", "b\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "A\n")
	repo.WriteFile("b.txt", "B\n")

	hash, err := svc.Commit(context.Background(), CommitOptions{Message: []byte("only a\n"), Paths: []string{"a.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	if hash.IsZero() {
		t.Fatal("zero hash")
	}
	if fileText(t, repo, "a.txt") != "A\n" {
		t.Fatalf("a.txt = %q", fileText(t, repo, "a.txt"))
	}
	// b.txt is still an unstaged modification.
	names := repo.Git("diff", "--name-only")
	if strings.TrimSpace(names) != "b.txt" {
		t.Fatalf("unstaged = %q", names)
	}
	show := repo.Git("show", "--name-only", "--format=", "HEAD")
	if strings.TrimSpace(show) != "a.txt" {
		t.Fatalf("commit touched %q", show)
	}
}

// TestCommitAmend ports the --amend option of GitRepositoryCommitter.
func TestCommitAmend(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	first := repo.Commit("first")
	repo.WriteFile("a.txt", "b\n")
	second := repo.Commit("second")
	repo.WriteFile("a.txt", "c\n")
	repo.Git("add", "a.txt")

	hash, err := svc.Commit(context.Background(), CommitOptions{Message: []byte("amended\n"), Amend: true})
	if err != nil {
		t.Fatal(err)
	}
	if hash.String() == second {
		t.Fatal("amend did not create a new commit")
	}
	if got := strings.TrimSpace(repo.Git("log", "-1", "--format=%s")); got != "amended" {
		t.Fatalf("subject = %q", got)
	}
	parent := strings.TrimSpace(repo.Git("rev-parse", "HEAD^"))
	if parent != first {
		t.Fatalf("parent = %s, want %s", parent, first)
	}
}

// TestCommitSignOff ports the --signoff option.
func TestCommitSignOff(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "b\n")
	repo.Git("add", "a.txt")

	if _, err := svc.Commit(context.Background(), CommitOptions{Message: []byte("signed\n"), SignOff: true}); err != nil {
		t.Fatal(err)
	}
	msg := repo.Git("log", "-1", "--format=%B")
	if !strings.Contains(msg, "Signed-off-by: Test User <test@example.invalid>") {
		t.Fatalf("message = %q", msg)
	}
}

// TestCommitAuthorOverride ports --author.
func TestCommitAuthorOverride(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")
	repo.WriteFile("a.txt", "b\n")
	repo.Git("add", "a.txt")

	if _, err := svc.Commit(context.Background(), CommitOptions{
		Message: []byte("other author\n"),
		Author:  "Other Person <other@example.invalid>",
	}); err != nil {
		t.Fatal(err)
	}
	author := strings.TrimSpace(repo.Git("log", "-1", "--format=%an <%ae>"))
	if author != "Other Person <other@example.invalid>" {
		t.Fatalf("author = %q", author)
	}
	committer := strings.TrimSpace(repo.Git("log", "-1", "--format=%cn <%ce>"))
	if committer != "Test User <test@example.invalid>" {
		t.Fatalf("committer = %q", committer)
	}
}

// TestCommitNothingToCommit ports the empty-commit detector in
// GitRepositoryCommitter.
func TestCommitNothingToCommit(t *testing.T) {
	svc, repo := newService(t)
	repo.WriteFile("a.txt", "a\n")
	repo.Commit("init")

	_, err := svc.Commit(context.Background(), CommitOptions{Message: []byte("empty\n")})
	if !errors.Is(err, ErrEmptyCommit) {
		t.Fatalf("got %v, want ErrEmptyCommit", err)
	}
}
