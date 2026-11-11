package app

import (
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/runbrick/r-git/internal/testutil"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestSuccessfulCommitResetsOpenForm(t *testing.T) {
	for _, amend := range []bool{false, true} {
		name := "new commit"
		if amend {
			name = "amend"
		}
		t.Run(name, func(t *testing.T) {
			g := testutil.NewRepo(t)
			g.WriteFile("base.txt", "base\n")
			g.Commit("initial")
			g.WriteFile("base.txt", "changed\n")
			m, driver := openDriver(t, g.Dir)
			waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
			clickControl(t, driver, "Commit")
			waitFor(t, driver, func() bool { return len(m.commit.Entries) > 0 })
			clickControl(t, driver, "Commit message")
			if err := driver.Type("next subject"); err != nil {
				t.Fatal(err)
			}
			if amend {
				clickControl(t, driver, "Amend")
			}
			clickControl(t, driver, "Sign-off")
			clickControl(t, driver, "Author")
			if err := driver.Type("Other Author <other@example.invalid>"); err != nil {
				t.Fatal(err)
			}
			clickControl(t, driver, "Commit changes")
			waitFor(t, driver, func() bool { return !m.commit.Busy && (hasLabel(driver, "Committed") || m.commit.Error != "") })
			if m.commit.Error != "" {
				t.Fatal(m.commit.Error)
			}
			if !m.showCommit || !hasLabel(driver, "Describe the change") || !hasLabel(driver, "No changes") {
				t.Fatalf("commit form was not ready for another commit: %v", labelValues(driver))
			}
			if m.commit.Message != "" || m.commit.Amend {
				t.Fatalf("form retained message=%q amend=%v", m.commit.Message, m.commit.Amend)
			}
			if !m.commit.SignOff || m.commit.Author != "Other Author <other@example.invalid>" {
				t.Fatalf("lost reusable options: signoff=%v author=%q", m.commit.SignOff, m.commit.Author)
			}
			if got := strings.TrimSpace(g.Git("log", "-1", "--format=%s")); got != "next subject" {
				t.Fatalf("subject = %q", got)
			}
			wantCount := "2"
			if amend {
				wantCount = "1"
			}
			if got := strings.TrimSpace(g.Git("rev-list", "--count", "HEAD")); got != wantCount {
				t.Fatalf("commit count = %s, want %s", got, wantCount)
			}
			// Amend still prefills the new HEAD; leaving it must restore an
			// empty draft, rather than resurrecting the committed message.
			clickControl(t, driver, "Amend")
			waitFor(t, driver, func() bool { return m.commit.Message == "next subject" })
			clickControl(t, driver, "Amend")
			waitFor(t, driver, func() bool { return m.commit.Message == "" && hasLabel(driver, "Describe the change") })
		})
	}
}

func TestCommitPanelGroupsStageCommitAmendAndDiscard(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("keep.txt", "keep\n")
	g.Commit("initial")
	g.WriteFile("keep.txt", "staged\n")
	g.Git("add", "keep.txt")
	g.WriteFile("keep.txt", "changed\n")
	g.WriteFile("dir/added.txt", "added\n")
	g.WriteFile("fresh.txt", "fresh\n")

	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
	clickControl(t, driver, "Commit")
	waitFor(t, driver, func() bool {
		return hasLabel(driver, "Staged") && hasLabel(driver, "Unversioned") && hasLabel(driver, "added.txt")
	})
	if !hasLabel(driver, "keep.txt") || !hasLabel(driver, "fresh.txt") {
		t.Fatalf("groups missing: %v", labelValues(driver))
	}
	if groupOf(m, "keep.txt") != repo.GroupStaged || groupOf(m, "fresh.txt") != repo.GroupUnversioned {
		t.Fatalf("grouping = %+v", m.commit.Entries)
	}

	clickNearLabel(t, driver, "added.txt", "Stage")
	waitFor(t, driver, func() bool {
		return (groupOf(m, "dir/added.txt") == repo.GroupStaged && !m.commit.Busy) || m.commit.Error != ""
	})
	if m.commit.Error != "" {
		t.Fatal(m.commit.Error)
	}

	clickControl(t, driver, "Commit message")
	if err := driver.Type("commit the staged files"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Commit changes")
	waitFor(t, driver, func() bool { return !m.commit.Busy && (m.commit.Message == "" || m.commit.Error != "") })
	if m.commit.Error != "" {
		t.Fatal(m.commit.Error)
	}
	if strings.TrimSpace(g.Git("log", "-1", "--format=%s")) != "commit the staged files" {
		t.Fatalf("log subject = %q", g.Git("log", "-1", "--format=%s"))
	}
	if groupOf(m, "dir/added.txt") == repo.GroupStaged {
		t.Fatal("staged files still listed after commit")
	}

	g.WriteFile("keep.txt", "amended\n")
	clickControl(t, driver, "Refresh")
	waitFor(t, driver, func() bool { return !m.log.Loading && m.repo != nil })
	if !m.showCommit {
		clickControl(t, driver, "Commit")
	} else {
		m.refreshStatus()
	}
	waitFor(t, driver, func() bool { return groupOf(m, "keep.txt") == repo.GroupUnstaged || m.commit.Error != "" })
	if m.commit.Error != "" {
		t.Fatal(m.commit.Error)
	}
	clickNearLabel(t, driver, "keep.txt", "Stage")
	waitFor(t, driver, func() bool { return groupOf(m, "keep.txt") == repo.GroupStaged && !m.commit.Busy })
	clickControl(t, driver, "Amend")
	waitFor(t, driver, func() bool {
		return strings.TrimSpace(m.commit.Message) == "commit the staged files" || m.commit.Error != ""
	})
	if m.commit.Message != "commit the staged files" {
		t.Fatalf("amend message = %q err=%q", m.commit.Message, m.commit.Error)
	}
	clickControl(t, driver, "Commit message")
	if _, err := driver.ShortcutChord(gfx.ModifierControl, 'a'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("amended subject"); err != nil {
		t.Fatal(err)
	}
	clickControl(t, driver, "Commit changes")
	m.workers.Wait()
	m.applyPending()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if m.commit.Error != "" || m.commit.Busy {
		t.Fatalf("amend busy=%v err=%q", m.commit.Busy, m.commit.Error)
	}
	if strings.TrimSpace(g.Git("log", "-1", "--format=%s")) != "amended subject" {
		t.Fatalf("amended subject = %q", g.Git("log", "-1", "--format=%s"))
	}

	g.WriteFile("gone.txt", "discard me\n")
	m.refreshStatus()
	m.workers.Wait()
	m.applyPending()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if groupOf(m, "gone.txt") != repo.GroupUnversioned {
		t.Fatalf("gone.txt missing err=%q entries=%+v", m.commit.Error, m.commit.Entries)
	}
	clickNearLabel(t, driver, "gone.txt", "Rollback")
	if !hasLabel(driver, "Discard") {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if !hasLabel(driver, "Discard") {
		t.Fatal("discard confirmation did not open")
	}
	clickControl(t, driver, "Cancel")
	if groupOf(m, "gone.txt") != repo.GroupUnversioned {
		t.Fatal("cancel removed the file")
	}
	clickNearLabel(t, driver, "gone.txt", "Rollback")
	clickControl(t, driver, "Discard")
	m.workers.Wait()
	m.applyPending()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if groupOf(m, "gone.txt") != repo.ChangeGroup(-1) {
		t.Fatalf("gone.txt still present: %+v", m.commit.Entries)
	}
}

func TestEmptyCommitMessageIsBlocked(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("a.txt", "a\n")
	g.Commit("initial")
	g.WriteFile("a.txt", "b\n")
	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading })
	clickControl(t, driver, "Commit")
	waitFor(t, driver, func() bool { return m.showCommit && len(m.commit.Entries) > 0 })
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(driver, "a.txt") {
		t.Fatalf("file missing: %v entries=%+v err=%s", labelValues(driver), m.commit.Entries, m.commit.Error)
	}
	clickControl(t, driver, "Commit changes")
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(driver, "Enter a commit message.") {
		t.Fatalf("validation missing: %v", labelValues(driver))
	}
	if strings.TrimSpace(g.Git("log", "-1", "--format=%s")) != "initial" {
		t.Fatal("empty message created a commit")
	}
}

func groupOf(m *Model, path string) repo.ChangeGroup {
	for _, entry := range m.commit.Entries {
		if entry.Path == path {
			return entry.Group
		}
	}
	return repo.ChangeGroup(-1)
}

func clickNearLabel(t *testing.T, driver *viva.Driver, fileLabel, action string) {
	t.Helper()
	var fileY float32
	found := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == fileLabel {
			fileY = hit.Bounds.Y
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no file %q among %v", fileLabel, labelValues(driver))
	}
	var best *gfx.Rect
	bestDist := float32(1e9)
	for i := range driver.Snapshot().Hits {
		hit := driver.Snapshot().Hits[i]
		if hit.Label != action {
			continue
		}
		dist := hit.Bounds.Y - fileY
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			r := hit.Bounds
			best = &r
		}
	}
	if best == nil || bestDist > 40 {
		t.Fatalf("no %q near %q (dist %v)", action, fileLabel, bestDist)
	}
	if err := driver.Click(best.X+best.Width/2, best.Y+best.Height/2); err != nil {
		t.Fatal(err)
	}
}
