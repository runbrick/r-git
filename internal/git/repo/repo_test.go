package repo

import (
	"context"
	"errors"
	"os"
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

func TestPackedRefsWithPeeledAnnotatedTag(t *testing.T) {
	// Ported from GitRepositoryReaderTest's packed-refs testData handling.
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	commit := g.Commit("base")
	g.Git("tag", "-a", "release", "-m", "annotated")
	g.Git("pack-refs", "--all", "--prune")
	r := openTestRepo(t, g.Dir)
	if len(r.Tags) != 1 || r.Tags[0].Ref.Name() != "release" {
		t.Fatalf("packed tags = %#v", r.Tags)
	}
	peeled := strings.TrimSpace(g.Git("rev-parse", "refs/tags/release^{}"))
	if peeled != commit || r.Tags[0].Hash.String() == peeled {
		t.Fatalf("annotated tag hash=%s peeled=%s commit=%s", r.Tags[0].Hash, peeled, commit)
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
	// Ported from GitLogParserTest.test_files_with_spaces and GitChangeProviderVersionedTest status cases.
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
	if runtime.GOOS == "windows" {
		parsed, err := ParseNameStatusZ([]byte("M\x00" + newlinePath + "\x00"))
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed) != 1 || parsed[0].Path != newlinePath {
			t.Fatalf("newline ChangedFiles record = %#v", parsed)
		}
	} else {
		foundNewline = false
		for _, change := range files {
			if change.Path == newlinePath {
				foundNewline = true
			}
		}
		if !foundNewline {
			t.Fatalf("ChangedFiles omitted newline path: %#v", files)
		}
	}
	foundUnicodeChange := false
	for _, change := range files {
		if change.Path == unicodePath {
			foundUnicodeChange = true
		}
	}
	if !foundUnicodeChange {
		t.Fatalf("ChangedFiles omitted Unicode path %q: %#v", unicodePath, files)
	}
	parents, err := r.Parents(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(parents[commits[0].Hash]) != 1 {
		t.Fatalf("parents = %#v", parents)
	}
}

func TestStatusOmitsIgnoredPaths(t *testing.T) {
	// Ported from GitChangeProviderVersionedTest's untracked/ignored visibility behavior.
	g := testutil.NewRepo(t)
	g.WriteFile(".gitignore", "ignored/\n")
	g.WriteFile("tracked.txt", "tracked\n")
	g.Commit("base")
	g.WriteFile("ignored/hidden.txt", "ignored\n")
	r := openTestRepo(t, g.Dir)
	status, err := r.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range status.Changes {
		if strings.Contains(change.Path, "ignored") {
			t.Fatalf("ignored path reported: %#v", change)
		}
	}
}

func TestChangedFilesCopiedAndTypeChanged(t *testing.T) {
	// Ported from GitLogParserTest name-status changes and GitChangesParser type handling.
	t.Run("copy", func(t *testing.T) {
		g := testutil.NewRepo(t)
		g.WriteFile("source.txt", "same contents\n")
		g.Commit("source")
		g.WriteFile("copy.txt", "same contents\n")
		copyCommit := g.Commit("copy")
		r := openTestRepo(t, g.Dir)
		changes, err := r.ChangedFiles(context.Background(), model.MustParseHash(copyCommit))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, change := range changes {
			if change.Status == model.StatusCopied && change.OrigPath == "source.txt" && change.Path == "copy.txt" {
				found = true
			}
		}
		if !found {
			t.Fatalf("copy change missing: %#v", changes)
		}
	})
	t.Run("type change", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows symlink creation requires elevated/developer mode")
		}
		g := testutil.NewRepo(t)
		g.WriteFile("item.txt", "target")
		g.Commit("regular file")
		if err := os.Remove(filepath.Join(g.Dir, "item.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", filepath.Join(g.Dir, "item.txt")); err != nil {
			t.Fatal(err)
		}
		typeCommit := g.Commit("change file type")
		r := openTestRepo(t, g.Dir)
		changes, err := r.ChangedFiles(context.Background(), model.MustParseHash(typeCommit))
		if err != nil {
			t.Fatal(err)
		}
		if len(changes) != 1 || changes[0].Status != model.StatusTypeChanged {
			t.Fatalf("type change = %#v", changes)
		}
	})
}

func TestLogQueryFiltersAndParentsOrdered(t *testing.T) {
	// Ported from GitLogParserTest.generateRecords and GitLogUtilTest history ordering cases.
	g := testutil.NewRepo(t)
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-01T12:00:00Z")
	g.WriteFile("a.txt", "zero\n")
	initial := g.Commit("initial")
	g.WriteFile("a.txt", "one\n")
	a1 := g.Commit("a1")
	g.WriteFile("b.txt", "b\n")
	b1 := g.Commit("b1")
	g.WriteFile("a.txt", "two\n")
	a2 := g.Commit("a2")
	g.Git("checkout", "-qb", "side")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-03T12:00:00Z")
	g.WriteFile("side.txt", "side\n")
	side := g.Commit("side")
	g.Git("checkout", "-q", "master")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-02T12:00:00Z")
	g.WriteFile("main.txt", "main\n")
	main := g.Commit("main")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-04T12:00:00Z")
	g.Git("merge", "--no-ff", "-m", "merge", "side")
	merge := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	r := openTestRepo(t, g.Dir)
	tests := []struct {
		name  string
		query LogQuery
		want  []string
	}{
		{name: "max count", query: LogQuery{MaxCount: 2}, want: []string{merge, side}},
		{name: "skip", query: LogQuery{MaxCount: 1, Skip: 1}, want: []string{side}},
		{name: "range", query: LogQuery{Range: b1 + "..HEAD"}, want: []string{merge, side, main, a2}},
		{name: "branch", query: LogQuery{Branch: "side"}, want: []string{side, a2, b1, a1, initial}},
		{name: "path", query: LogQuery{Paths: []string{"a.txt"}}, want: []string{a2, a1, initial}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			commits, err := r.Log(context.Background(), tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(commits) != len(tc.want) {
				t.Fatalf("got %d commits (%v), want %d: %v", len(commits), commitIDs(commits), len(tc.want), tc.want)
			}
			for i, want := range tc.want {
				if commits[i].Hash.String() != want {
					t.Fatalf("commit[%d]=%s want %s; all=%v", i, commits[i].Hash, want, commitIDs(commits))
				}
			}
		})
	}
	ordered, err := r.ParentsOrdered(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) == 0 || ordered[0].Commit.String() != merge || len(ordered[0].Parents) != 2 {
		t.Fatalf("ordered parents first=%#v", ordered[0])
	}
	if ordered[0].Parents[0].String() != main || ordered[0].Parents[1].String() != side {
		t.Fatalf("merge parents = %#v", ordered[0].Parents)
	}
}

func commitIDs(commits []model.Commit) []string {
	ids := make([]string, len(commits))
	for i, commit := range commits {
		ids[i] = commit.Hash.String()
	}
	return ids
}

func TestOpenReadsMergeAndRebaseMarkers(t *testing.T) {
	// Ported from GitRepositoryReaderTest state detection; backend cases exercise rebase-apply and rebase-merge.
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

func TestRevertAndDetachedRebaseStates(t *testing.T) {
	// Ported from GitRepositoryReaderTest.readRepositoryState and findRebaseBranch.
	t.Run("revert in progress", func(t *testing.T) {
		g := testutil.NewRepo(t)
		g.WriteFile("conflict.txt", "base\n")
		g.Commit("base")
		g.Git("checkout", "-qb", "topic")
		g.WriteFile("conflict.txt", "topic\n")
		revert := g.Commit("topic change")
		g.Git("checkout", "-q", "master")
		g.WriteFile("conflict.txt", "main\n")
		g.Commit("main change")
		cmd := exec.Command("git", "-C", g.Dir, "revert", "--no-edit", revert)
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("expected revert conflict, got success: %s", output)
		}
		r := openTestRepo(t, g.Dir)
		if r.State != model.StateReverting {
			t.Fatalf("revert state = %v", r.State)
		}
	})
	t.Run("detached HEAD during rebase", func(t *testing.T) {
		g := testutil.NewRepo(t)
		g.WriteFile("base.txt", "base\n")
		commit := g.Commit("base")
		g.Git("checkout", "--detach", "-q", commit)
		gitDir := strings.TrimSpace(g.Git("rev-parse", "--absolute-git-dir"))
		marker := filepath.Join(gitDir, "rebase-merge")
		if err := os.MkdirAll(marker, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(marker, "head-name"), []byte("detached HEAD\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		r := openTestRepo(t, g.Dir)
		if r.State != model.StateRebasing || !r.CurrentBranch.IsZero() || r.HEAD.String() != commit {
			t.Fatalf("detached rebase: state=%v branch=%v HEAD=%s", r.State, r.CurrentBranch, r.HEAD)
		}
	})
}

func TestRemotesSlashNameAndLinkedWorktree(t *testing.T) {
	// Ported from GitConfigTest and GitWorkingTreeTest.
	g := testutil.NewRepo(t)
	g.WriteFile("x", "x")
	g.Commit("base")
	g.Git("remote", "add", "team/repo", "https://example.invalid/repo.git")
	g.Git("config", "branch.master.remote", "team/repo")
	g.Git("config", "branch.master.merge", "refs/heads/feature/topic")
	r := openTestRepo(t, g.Dir)
	if len(r.Remotes) != 1 || r.Remotes[0].Name != "team/repo" {
		t.Fatalf("remotes=%#v", r.Remotes)
	}
	if len(r.Tracking) != 1 || r.Tracking[0].Remote.Remote.Name != "team/repo" || r.Tracking[0].Remote.NameForRemoteOperations != "feature/topic" {
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

func TestParseNameStatusZCopyAndTypeChange(t *testing.T) {
	// Ported from GitLogParserTest name-status records and GitChangesParser type handling.
	tests := []struct {
		name string
		data []byte
		want model.FileStatus
	}{
		{name: "copy", data: []byte("C100\x00source name\x00copy\nname\x00"), want: model.StatusCopied},
		{name: "type change", data: []byte("T\x00unicode-\u00e9.txt\x00"), want: model.StatusTypeChanged},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes, err := ParseNameStatusZ(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if len(changes) != 1 || changes[0].Status != tc.want {
				t.Fatalf("parsed changes=%#v want status %s", changes, tc.want)
			}
		})
	}
}
