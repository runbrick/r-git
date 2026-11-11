package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/testutil"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestOpenRepositoryShowsLogAndDetails(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("initial history entry")
	g.Git("branch", "topic")
	g.WriteFile("base.txt", "updated\n")
	g.WriteFile("new.txt", "new\n")
	second := g.Commit("add new file for log")

	for _, appearance := range []viva.Appearance{viva.AppearanceLight, viva.AppearanceDark} {
		t.Run(appearance.String(), func(t *testing.T) {
			viva.SetNativeAppearance(appearance)
			m, driver := openDriverTheme(t, g.Dir, appearance.String())
			waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading && hasLabel(driver, "new.txt") })
			if !hasLabel(driver, "add new file for log") || !hasLabel(driver, m.log.Commits[0].Hash.Short()) || !hasLabel(driver, "new.txt") {
				t.Fatalf("log row/details labels missing: %v", labelValues(driver))
			}
			if len(m.log.Commits) != 2 || m.log.Selected().String() != second {
				t.Fatalf("loaded selection = %d commits, %s", len(m.log.Commits), m.log.Selected())
			}
			builds := 0
			m.log.OnGraphBuilt = func() { builds++ }
			m.log.SetFilter("initial history")
			if m.log.VisibleCount() != 1 {
				t.Fatalf("text filter visible count = %d", m.log.VisibleCount())
			}
			m.changeBranch("topic")
			waitFor(t, driver, func() bool { return !m.log.Loading })
			if m.log.Branch != "topic" || m.log.VisibleCount() != 1 {
				t.Fatalf("branch filter = %q with %d visible commits", m.log.Branch, m.log.VisibleCount())
			}
			if builds != 1 {
				t.Fatalf("branch graph build count = %d, want 1", builds)
			}
			if !m.log.Selected().Equal(m.log.Commits[0].Hash) {
				t.Fatal("branch selection did not update details selection")
			}
			if !hasLabel(driver, "Committer") {
				t.Fatal("details pane is not painted")
			}
		})
	}
}

func TestDriverShowsOpenError(t *testing.T) {
	m, driver := openDriver(t, filepath.Join(t.TempDir(), "missing-repository"))
	waitFor(t, driver, func() bool { return m.err != "" })
	if !hasLabel(driver, "not a git repository") {
		t.Fatalf("error label missing: %v", labelValues(driver))
	}
}

func TestDriverTextFilterBranchFilterAndSelectionDetails(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("shared history")
	mainBranch := strings.TrimSpace(g.Git("branch", "--show-current"))
	g.Git("checkout", "-q", "-b", "topic")
	g.WriteFile("topic.txt", "topic\n")
	topicHash := g.Commit("topic exclusive")
	g.Git("checkout", "-q", mainBranch)
	g.WriteFile("main.txt", "main\n")
	g.Commit("main exclusive")

	for _, appearance := range []viva.Appearance{viva.AppearanceLight, viva.AppearanceDark} {
		t.Run(appearance.String(), func(t *testing.T) {
			viva.SetNativeAppearance(appearance)
			m, driver := openDriverTheme(t, g.Dir, appearance.String())
			waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading && len(m.log.Details) > 0 })
			if len(m.log.Commits) != 3 {
				t.Fatalf("all branches loaded %d commits, want 3", len(m.log.Commits))
			}
			clickControl(t, driver, "Filter")
			if err := driver.Type("topic exclusive"); err != nil {
				t.Fatal(err)
			}
			waitFor(t, driver, func() bool { return m.log.VisibleCount() == 1 })
			visible, ok := m.log.VisibleCommit(0)
			if !ok || visible.Hash.String() != topicHash {
				t.Fatalf("text filter returned %+v", visible)
			}
			clickControl(t, driver, "topic exclusive")
			m.workers.Wait()
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			waitFor(t, driver, func() bool { return m.log.Selected().String() == topicHash && hasLabel(driver, "added  topic.txt") })
			if hasLabel(driver, "added  main.txt") {
				t.Fatal("details still show previous commit's changed files")
			}
			clickControl(t, driver, "Filter")
			if _, err := driver.ShortcutChord(gfx.ModifierControl, 'a'); err != nil {
				t.Fatal(err)
			}
			if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
				t.Fatal(err)
			}
			clickControl(t, driver, "Branch")
			clickControl(t, driver, "topic")
			waitFor(t, driver, func() bool { return m.log.Branch == "topic" && !m.log.Loading })
			if len(m.log.Commits) != 2 || m.log.VisibleCount() != 2 {
				t.Fatalf("branch has metadata=%d visible=%d, want 2", len(m.log.Commits), m.log.VisibleCount())
			}
			for _, commit := range m.log.Commits {
				if commit.Subject == "main exclusive" {
					t.Fatal("branch filter included another branch's commit")
				}
			}
			if !hasLabel(driver, "topic exclusive") || hasLogLabel(driver, "main exclusive") {
				t.Fatal("branch row labels do not match selected branch")
			}
			clickControl(t, driver, "Branch")
			clickControl(t, driver, "All branches")
			waitFor(t, driver, func() bool { return m.log.Branch == "All branches" && !m.log.Loading })
			if len(m.log.Commits) != 3 || !hasLogLabel(driver, "main exclusive") {
				t.Fatal("All branches did not restore the other branch's history")
			}
		})
	}
}

func TestDriverAppliesPageAndDetailsPublishedBeforeOneWake(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("base")
	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && len(m.log.Details) > 0 })
	m.workers.Wait()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	selected := m.log.Selected()
	newCommit := model.Commit{Hash: model.MustParseHash(strings.Repeat("a", 40)), Subject: "queued page"}
	m.log.Loading = true
	var publishers sync.WaitGroup
	publishers.Add(2)
	go func() {
		defer publishers.Done()
		m.publish(&loadResult{kind: resultPage, generation: m.generation, commits: []model.Commit{newCommit}, done: true})
	}()
	go func() {
		defer publishers.Done()
		m.publish(&loadResult{kind: resultDetails, generation: m.generation, hash: selected, details: []model.FileChange{{Status: model.StatusAdded, Path: "queued-details.txt"}}})
	}()
	publishers.Wait()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(m.log.Commits) != 2 || m.log.Loading {
		t.Fatalf("page result not applied: commits=%d loading=%v", len(m.log.Commits), m.log.Loading)
	}
	if !hasLabel(driver, "queued-details.txt") {
		t.Fatal("details result was lost before the wake")
	}
}

func clickControl(t *testing.T, driver *viva.Driver, label string) {
	t.Helper()
	var found bool
	var x, y float32
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label && (!found || hit.Bounds.X+hit.Bounds.Width/2 >= x) {
			found = true
			x, y = hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
		}
	}
	if found {
		if err := driver.Click(x, y); err != nil {
			t.Fatal(err)
		}
		return
	}
	var labels []string
	for _, hit := range driver.Snapshot().Hits {
		labels = append(labels, hit.Label)
	}
	t.Fatalf("no control %q among %v", label, labels)
}

func hasLogLabel(driver *viva.Driver, value string) bool {
	for _, label := range driver.Snapshot().Labels {
		if label.Value == value && label.Bounds.X < 800 {
			return true
		}
	}
	return false
}

func TestDriverScrollLoadsNextPageWithoutRebuildingGraph(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("history.txt", "0\n")
	g.Commit("entry 0")
	for i := 1; i < pageSize+2; i++ {
		g.Git("commit", "--allow-empty", "-q", "-m", fmt.Sprintf("entry %d", i))
	}
	m, driver := openDriver(t, g.Dir)
	waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading && len(m.log.Commits) == pageSize })
	builds := 0
	m.log.OnGraphBuilt = func() { builds++ }
	requests := 0
	loadPage := m.log.OnPage
	m.log.OnPage = func(branch string, skip int) { requests++; loadPage(branch, skip) }
	if err := driver.Scroll(gfx.ScrollEvent{X: 400, Y: 400, DeltaY: 100000, Viewport: gfx.Size{Width: 1200, Height: 780}}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, driver, func() bool { return len(m.log.Commits) > pageSize && !m.log.Loading || m.log.Error != "" })
	if m.log.Error != "" {
		t.Fatalf("page error: %s", m.log.Error)
	}
	if requests == 0 {
		t.Fatal("scroll did not request another page")
	}
	if len(m.log.Commits) != pageSize+2 {
		t.Fatalf("paged commits = %d", len(m.log.Commits))
	}
	if builds != 0 {
		t.Fatalf("page append rebuilt graph %d times", builds)
	}
}

func openDriver(t *testing.T, path string) (*Model, *viva.Driver) {
	t.Helper()
	return openDriverTheme(t, path, "dark")
}

func openDriverTheme(t *testing.T, path, theme string) (*Model, *viva.Driver) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "config")
	settings := config.DefaultAppSettings()
	settings.Theme = theme
	if err := config.SaveApp(dir, settings); err != nil {
		t.Fatal(err)
	}
	m, err := New(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	driver, err := viva.NewDriver(m.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close(); m.stopWorkers() })
	return m, driver
}

func waitFor(t *testing.T, driver *viva.Driver, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		// A fast initial error can publish before NewDriver binds its wake
		// handler. Deliver the loading spinner's scheduled frame as the real
		// window and headless runner do, in addition to background wakes.
		if err := driver.Advance(16 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for Viva wake result; labels=%v", labelValues(driver))
}

func hasLabel(driver *viva.Driver, value string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, value) {
			return true
		}
	}
	for _, hit := range driver.Snapshot().Hits {
		if strings.Contains(hit.Label, value) {
			return true
		}
	}
	return false
}

func labelValues(driver *viva.Driver) []string {
	labels := driver.Snapshot().Labels
	values := make([]string, len(labels))
	for i, label := range labels {
		values[i] = label.Value
	}
	return values
}
