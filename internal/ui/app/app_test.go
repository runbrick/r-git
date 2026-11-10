package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/r-git/internal/testutil"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
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
			m, driver := openDriver(t, g.Dir)
			waitFor(t, driver, func() bool { return m.repo != nil && !m.log.Loading && hasLabel(driver, "new.txt") })
			if !hasLabel(driver, "add new file for log") || !hasLabel(driver, "Hash") || !hasLabel(driver, "new.txt") {
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
	m, err := New(filepath.Join(t.TempDir(), "config"), path)
	if err != nil {
		t.Fatal(err)
	}
	page := ui.NewPage(ui.PageConfig{View: m.Page})
	driver, err := viva.NewDriver(page.Config(m.config()), nil)
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
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for Viva wake result")
}

func hasLabel(driver *viva.Driver, value string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, value) {
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
