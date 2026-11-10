package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/testutil"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestOpenRepositoryShowsLogAndDetails(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("initial history entry")
	g.WriteFile("base.txt", "updated\n")
	g.WriteFile("new.txt", "new\n")
	g.Commit("add new file for log")

	m, err := New(filepath.Join(t.TempDir(), "config"), g.Dir)
	if err != nil {
		t.Fatal(err)
	}
	page := ui.NewPage(ui.PageConfig{View: m.Page})
	driver, err := app.NewDriver(page.Config(app.Config{Title: "r-git", Width: 1200, Height: 780}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if !hasLabel(driver, "add new file for log") || !hasLabel(driver, "Test User") {
		t.Fatalf("log row labels missing: %#v", driver.Snapshot().Labels)
	}
	if len(m.graphRows) != len(m.commits) || m.graph.Row(m.graphRows[0]) == nil {
		t.Fatal("graph row was not prepared for the loaded commits")
	}
	if !hasLabel(driver, "Author") || !hasLabel(driver, "Committer") || !hasLabel(driver, "Changed files") || !hasLabel(driver, "new.txt") {
		t.Fatalf("selected commit details missing: %#v", driver.Snapshot().Labels)
	}

	m.filter = "initial history entry"
	if got := m.visibleCommits(); len(got) != 1 || m.commits[got[0]].Subject != "initial history entry" {
		t.Fatal("text filter did not retain the matching commit")
	}
}

func hasLabel(driver *app.Driver, text string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, text) {
			return true
		}
	}
	return false
}
