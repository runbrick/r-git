package repo

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/testutil"
)

func openTestRepo(t *testing.T, dir string) *Repo {
	t.Helper()
	r, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOpenRefsDetachedAndPackedRefs(t *testing.T) {
	// Ported from GitRepositoryReaderTest.test branches.
	g := testutil.NewRepo(t)
	g.WriteFile("one.txt", "one\n")
	first := g.Commit("first")
	g.Git("branch", "feature")
	g.Git("tag", "v1")
	g.Git("pack-refs", "--all", "--prune")
	r := openTestRepo(t, g.Dir)
	if r.CurrentBranch.Name() != "master" && r.CurrentBranch.Name() != "main" {
		t.Fatalf("branch = %q", r.CurrentBranch.Name())
	}
	if r.HEAD.String() != first {
		t.Fatalf("HEAD = %s, want %s", r.HEAD, first)
	}
	if len(r.Branches) < 2 || len(r.Tags) != 1 || r.Tags[0].Ref.Name() != "v1" {
		t.Fatalf("refs: branches=%v tags=%v", r.Branches, r.Tags)
	}
	g.Git("checkout", "--detach", "-q", first)
	r = openTestRepo(t, g.Dir)
	if !r.CurrentBranch.IsZero() || r.State != model.StateDetached || r.HEAD.String() != first {
		t.Fatalf("detached state = %#v branch=%v", r.State, r.CurrentBranch)
	}
}

func TestOpenRejectsBareRepository(t *testing.T) {
	// Ported from GitBareWorkTreeTest repository-kind behavior.
	_ = testutil.NewRepo(t) // Install isolated Git environment for subprocesses.
	bare := filepath.Join(t.TempDir(), "bare.git")
	cmd := exec.Command("git", "init", "--bare", "-q", bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	_, err := Open(context.Background(), bare)
	if !errors.Is(err, ErrBareRepository) {
		t.Fatalf("Open bare err = %v", err)
	}
}

func TestStatusAndHistoryHandleArbitraryPathsAndRename(t *testing.T) {
	// Ported from GitLogParserTest.test_files_with_spaces.
	g := testutil.NewRepo(t)
	unicodePath := "\u76ee\u5f55/\u00e9.txt"
	g.WriteFile("old name.txt", "base\n")
	g.WriteFile(unicodePath, "unicode\n")
	g.Commit("base")
	g.Git("mv", "old name.txt", "new name.txt")
	newlinePath := "line\nbreak.txt"
	if runtime.GOOS != "windows" {
		g.WriteFile(newlinePath, "contents\n")
	}
	g.WriteFile(unicodePath, "changed\n")
	r := openTestRepo(t, g.Dir)
	status, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	foundRename, foundNewline, foundUnicode := false, false, false
	for _, change := range status.Changes {
		if change.Status == model.StatusRenamed && change.Path == "new name.txt" && change.OrigPath == "old name.txt" {
			foundRename = true
		}
		if change.Path == newlinePath {
			foundNewline = true
		}
		if change.Path == unicodePath {
			foundUnicode = true
		}
	}
	if runtime.GOOS == "windows" {
		parsed, err := ParseStatusV2([]byte("? " + newlinePath + "\x00"))
		if err != nil {
			t.Fatal(err)
		}
		foundNewline = len(parsed.Changes) == 1 && parsed.Changes[0].Path == newlinePath
	}
	if !foundRename || !foundNewline || !foundUnicode {
		t.Fatalf("status changes = %#v", status.Changes)
	}
	commitHash := g.Commit("rename and paths")
	commits, err := r.Log(context.Background(), LogQuery{MaxCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 || commits[0].Hash.String() != commitHash || commits[0].Subject != "rename and paths" {
		t.Fatalf("log = %#v", commits)
	}
	files, err := r.ChangedFiles(context.Background(), commits[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatalf("changed files = %#v", files)
	}
	parents, err := r.Parents(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(parents[commits[0].Hash]) != 1 {
		t.Fatalf("parents = %#v", parents)
	}
}

func TestOpenReadsMergeAndRebaseMarkers(t *testing.T) {
	// Ported from GitRepositoryReaderTest state detection for merge/rebase markers.
	for _, backend := range []string{"apply", "merge"} {
		t.Run(backend, func(t *testing.T) {
			g := testutil.NewRepo(t)
			g.WriteFile("conflict.txt", "base\n")
			g.Commit("base")
			g.Git("checkout", "-qb", "topic")
			g.WriteFile("conflict.txt", "topic\n")
			g.Commit("topic")
			g.Git("checkout", "-q", "master")
			g.WriteFile("conflict.txt", "main\n")
			g.Commit("main")
			g.Git("checkout", "-q", "topic")
			rebase := exec.Command("git", "-C", g.Dir, "-c", "rebase.backend="+backend, "rebase", "master")
			if output, err := rebase.CombinedOutput(); err == nil {
				t.Fatalf("expected conflicting rebase, got success: %s", output)
			}
			r := openTestRepo(t, g.Dir)
			if r.State != model.StateRebasing || r.CurrentBranch.Name() != "topic" {
				t.Fatalf("rebase %s: state=%v branch=%s", backend, r.State, r.CurrentBranch.Name())
			}
		})
	}
	g := testutil.NewRepo(t)
	g.WriteFile("x", "base")
	g.Commit("base")
	g.Git("checkout", "-qb", "side")
	g.WriteFile("x", "side")
	g.Commit("side")
	g.Git("checkout", "-q", "master")
	g.WriteFile("x", "main")
	g.Commit("main")
	merge := exec.Command("git", "-C", g.Dir, "merge", "side")
	if output, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("expected merge conflict, got success: %s", output)
	}
	r := openTestRepo(t, g.Dir)
	if r.State != model.StateMerging {
		t.Fatalf("merge state = %v", r.State)
	}
	status, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Changes) != 1 || status.Changes[0].Status != model.StatusUnmerged {
		t.Fatalf("merge status = %#v", status.Changes)
	}
}

func TestRemotesSlashNameAndLinkedWorktree(t *testing.T) {
	// Ported from GitConfigTest and GitWorkingTreeTest.
	g := testutil.NewRepo(t)
	g.WriteFile("x", "x")
	g.Commit("base")
	g.Git("remote", "add", "team/repo", "https://example.invalid/repo.git")
	g.Git("config", "branch.master.remote", "team/repo")
	g.Git("config", "branch.master.merge", "refs/heads/main")
	r := openTestRepo(t, g.Dir)
	if len(r.Remotes) != 1 || r.Remotes[0].Name != "team/repo" {
		t.Fatalf("remotes=%#v", r.Remotes)
	}
	if len(r.Tracking) != 1 || r.Tracking[0].Remote.Remote.Name != "team/repo" || r.Tracking[0].Remote.NameForRemoteOperations != "main" {
		t.Fatalf("tracking=%#v", r.Tracking)
	}
	other := filepath.Join(filepath.Dir(g.Dir), "linked tree")
	g.Git("worktree", "add", "-qb", "linked", other)
	trees, err := r.Worktrees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(trees) != 2 || !trees[0].Main && !trees[1].Main {
		t.Fatalf("worktrees = %#v", trees)
	}
}

func TestOpenDetectsCherryPick(t *testing.T) {
	// Ported from GitRepositoryReaderTest's GRAFTING state behavior.
	g := testutil.NewRepo(t)
	g.WriteFile("conflict.txt", "base\n")
	g.Commit("base")
	g.Git("checkout", "-qb", "topic")
	g.WriteFile("conflict.txt", "topic\n")
	cherry := g.Commit("topic change")
	g.Git("checkout", "-q", "master")
	g.WriteFile("conflict.txt", "main\n")
	g.Commit("main change")
	cmd := exec.Command("git", "-C", g.Dir, "cherry-pick", cherry)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected cherry-pick conflict, got success: %s", output)
	}
	r := openTestRepo(t, g.Dir)
	if r.State != model.StateGrafting {
		t.Fatalf("cherry-pick state = %v", r.State)
	}
}

func TestStashes(t *testing.T) {
	// Ported from GitStashTest.test list stashes.
	g := testutil.NewRepo(t)
	g.WriteFile("file.txt", "base\n")
	g.Commit("base")
	g.WriteFile("file.txt", "dirty\n")
	g.Git("stash", "push", "-m", "save this")
	r := openTestRepo(t, g.Dir)
	stashes, err := r.Stashes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stashes) != 1 || stashes[0].Index != 0 || stashes[0].Ref != "stash@{0}" || stashes[0].Branch == "" || !strings.Contains(stashes[0].Message, "save this") {
		t.Fatalf("stashes = %#v", stashes)
	}
}

func TestParseStatusV2UnmergedAndAheadBehind(t *testing.T) {
	// Ported from GitLogParserTest parser boundary cases and status conflict reporting.
	h := strings.Repeat("a", 40)
	input := []byte("# branch.head main\x00# branch.ab +2 -3\x00u UU N... 100644 100644 100644 100644 " + h + " " + h + " " + h + " conflict\nname\x00")
	got, err := ParseStatusV2(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ahead != 2 || got.Behind != 3 || len(got.Changes) != 1 || got.Changes[0].Status != model.StatusUnmerged || got.Changes[0].Path != "conflict\nname" {
		t.Fatalf("parsed status = %#v", got)
	}
}
