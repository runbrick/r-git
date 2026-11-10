package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/testutil"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestMain(m *testing.M) {
	if handled, exitCode := ops.RunEditorIfRequested(); handled {
		os.Exit(exitCode)
	}
	os.Exit(m.Run())
}

func TestDriverModifiedFileDiffNavigation(t *testing.T) {
	g := testutil.NewRepo(t)
	var base, changed strings.Builder
	for i := 0; i < 180; i++ {
		fmt.Fprintf(&base, "line %d\n", i)
		if i == 2 || i == 120 {
			fmt.Fprintf(&changed, "changed %d\n", i)
		} else {
			fmt.Fprintf(&changed, "line %d\n", i)
		}
	}
	g.WriteFile("file.txt", base.String())
	g.Commit("base")
	g.WriteFile("file.txt", changed.String())
	for _, appearance := range []viva.Appearance{viva.AppearanceLight, viva.AppearanceDark} {
		t.Run(appearance.String(), func(t *testing.T) {
			viva.SetNativeAppearance(appearance)
			m, d := openDriver(t, g.Dir)
			waitFor(t, d, func() bool { return m.repo != nil && len(m.log.Details) > 0 })
			// This is the exact callback contract used by a changes tree.
			m.OnShowDiff(model.Hash{}, "file.txt")
			waitFor(t, d, func() bool { return m.tools.screen == "diff" })
			if len(m.tools.diff.Result.Changes) != 2 {
				t.Fatal(m.tools.diff.Result.Changes)
			}
			first, last := m.tools.diff.VisibleRange()
			if first != 0 || last >= 180 {
				t.Fatalf("visible %d..%d: not virtualized", first, last)
			}
			clickControl(t, d, "Next change")
			if m.tools.diff.CurrentChange() != 0 {
				t.Fatal("next did not select first change")
			}
			clickControl(t, d, "Diff content (F7 next, Shift+F7 previous)")
			if err := d.Key(gfx.KeyF7, 0); err != nil {
				t.Fatal(err)
			}
			if m.tools.diff.CurrentChange() != 1 {
				t.Fatal("F7 did not navigate")
			}
			first, _ = m.tools.diff.VisibleRange()
			if first < 100 {
				t.Fatalf("next change not scrolled into view: %d", first)
			}
			if err := d.Key(gfx.KeyF7, gfx.ModifierShift); err != nil {
				t.Fatal(err)
			}
			if m.tools.diff.CurrentChange() != 0 {
				t.Fatal("Shift F7 did not navigate back")
			}
			clickControl(t, d, "Unified")
			if !m.tools.diff.Unified {
				t.Fatal("unified not selected")
			}
			clickControl(t, d, "Close diff")
			// The commit-details file button is wired through OnShowDiff too.
			clickControl(t, d, "added  file.txt")
			waitFor(t, d, func() bool { return m.tools.screen == "diff" })
			if strings.Join(m.tools.diff.Result.A, "\n") != "" || strings.Join(m.tools.diff.Result.B, "\n") != base.String() {
				t.Fatal("root commit diff did not compare empty tree")
			}
		})
	}
}

func createConflict(t *testing.T) (*testutil.Repo, ops.Operations) {
	t.Helper()
	g := testutil.NewRepo(t)
	base := "first\nkeep 1\nkeep 2\nkeep 3\nkeep 4\nlast\n"
	g.WriteFile("conflict.txt", base)
	g.Commit("base")
	main := strings.TrimSpace(g.Git("branch", "--show-current"))
	g.Git("checkout", "-qb", "side")
	g.WriteFile("conflict.txt", strings.ReplaceAll(strings.ReplaceAll(base, "first", "theirs first"), "last", "theirs last"))
	g.Commit("side")
	g.Git("checkout", "-q", main)
	g.WriteFile("conflict.txt", strings.ReplaceAll(strings.ReplaceAll(base, "first", "ours first"), "last", "ours last"))
	g.Commit("ours")
	runner, err := command.New()
	if err != nil {
		t.Fatal(err)
	}
	o := ops.Operations{Dir: g.Dir, Git: runner}
	if result := o.Merge(context.Background(), "side", ops.MergeOptions{}); result.Outcome != ops.OutcomeConflicts {
		t.Fatalf("merge %#v", result)
	}
	return g, o
}

func TestDriverResolveMixedConflictChunks(t *testing.T) {
	g, o := createConflict(t)
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && len(m.log.Details) > 0 })
	clickControl(t, d, "Conflicts")
	waitFor(t, d, func() bool { return m.tools.merge.Merge != nil && !m.tools.busy })
	if len(m.tools.merge.Merge.Chunks) != 2 {
		t.Fatal(m.tools.merge.Merge.Chunks)
	}
	clickControl(t, d, "Accept Theirs 1")
	clickControl(t, d, "Accept Yours 2")
	clickControl(t, d, "Mark resolved")
	waitFor(t, d, func() bool { return !m.tools.busy && m.tools.merge.Merge == nil })
	data, err := os.ReadFile(filepath.Join(g.Dir, "conflict.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "theirs first\nkeep 1\nkeep 2\nkeep 3\nkeep 4\nours last\n"
	if string(data) != want {
		t.Fatalf("result %q want %q", data, want)
	}
	conflicts, err := o.Conflicts(context.Background())
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("not staged resolved: %v %v", conflicts, err)
	}
}

func TestDriverInteractiveRebaseSquash(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("base")
	g.WriteFile("one.txt", "one\n")
	g.Commit("first commit")
	g.WriteFile("two.txt", "two\n")
	g.Commit("second commit")
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && len(m.log.Details) > 0 })
	before := strings.TrimSpace(g.Git("rev-parse", "HEAD"))
	clickControl(t, d, "Interactive rebase")
	clickControl(t, d, "Load commits")
	waitFor(t, d, func() bool { return len(m.tools.rebase.Entries) == 2 && !m.tools.busy })
	if got := strings.TrimSpace(g.Git("rev-parse", "HEAD")); got != before {
		t.Fatal("preview changed HEAD")
	}
	clickControl(t, d, "Action 2")
	clickControl(t, d, "squash")
	clickControl(t, d, "Start rebase")
	waitFor(t, d, func() bool { return !m.tools.busy && !m.tools.rebase.Open || m.tools.rebase.Error != "" })
	if m.tools.rebase.Error != "" {
		t.Fatal(m.tools.rebase.Error)
	}
	if count := strings.TrimSpace(g.Git("rev-list", "--count", "HEAD")); count != "2" {
		t.Fatalf("commit count %s want 2", count)
	}
	message := g.Git("log", "-1", "--format=%B")
	if !strings.Contains(message, "first commit") || !strings.Contains(message, "second commit") {
		t.Fatalf("squash message %q", message)
	}
	if got := g.Git("status", "--porcelain"); got != "" {
		t.Fatalf("dirty after rebase: %s", got)
	}
}
