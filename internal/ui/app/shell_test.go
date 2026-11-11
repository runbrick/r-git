package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/testutil"
	apptheme "github.com/runbrick/r-git/internal/ui/theme"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestLogColumnsAlignAtWindowWidths(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	subject := strings.TrimSpace(strings.Repeat("A long commit subject must leave room for metadata ", 8))
	g.Commit(subject)
	g.Git("commit", "--amend", "--no-edit", "--date=2001-02-03T12:00:00Z")
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && !m.log.Loading && len(m.log.Details) > 0 })
	for _, width := range []int{1200, 1400, 1600, 1400, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			if err := d.Resize(width, 900); err != nil {
				t.Fatal(err)
			}
			snapshot := d.Snapshot()
			var row gfx.Rect
			for _, hit := range snapshot.Hits {
				if hit.Label == subject {
					row = hit.Bounds
					break
				}
			}
			if row.Width == 0 {
				t.Fatal("first log row is missing")
			}
			find := func(value string, inRow bool) viva.Label {
				t.Helper()
				for _, label := range snapshot.Labels {
					inside := label.Bounds.Y >= row.Y && label.Bounds.Y < row.Y+row.Height
					if label.Value == value && inside == inRow && (inRow || label.Bounds.Y < row.Y) {
						return label
					}
				}
				t.Fatalf("missing painted text %q (in row: %v)", value, inRow)
				return viva.Label{}
			}
			for _, column := range []struct{ heading, cell string }{
				{"Author", m.log.Commits[0].Author.Name},
				{"Date", "2001-02-03"},
				{"Hash", m.log.Commits[0].Hash.Short()},
			} {
				header, cell := find(column.heading, false), find(column.cell, true)
				if cell.Bounds.X < header.Bounds.X-0.5 || cell.Bounds.X+cell.Bounds.Width > header.Bounds.X+header.Bounds.Width+0.5 {
					t.Errorf("%s cell %v falls outside header column %v", column.heading, cell.Bounds, header.Bounds)
				}
			}
			ellipsis := false
			for _, cell := range snapshot.Labels {
				if cell.Bounds.Y < row.Y || cell.Bounds.Y >= row.Y+row.Height {
					continue
				}
				if cell.Bounds.X < row.X-0.5 || cell.Bounds.X+cell.Bounds.Width > row.X+row.Width+0.5 {
					t.Errorf("cell %q (%v) extends past log row %v", cell.Value, cell.Bounds, row)
				}
				ellipsis = ellipsis || strings.HasSuffix(cell.Value, "\u2026")
			}
			if !ellipsis {
				t.Fatal("long subject must end with an ellipsis")
			}
		})
	}
}

func TestToolWindowsKeepLogVisibleAndToggle(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		t.Run(theme, func(t *testing.T) {
			g := testutil.NewRepo(t)
			g.WriteFile("base.txt", "base\n")
			g.Commit("history stays visible")
			m, d := openDriverTheme(t, g.Dir, theme)
			waitFor(t, d, func() bool { return m.repo != nil && !m.log.Loading })
			if !m.showCommit || !hasLabel(d, "Commit changes") || !hasLogLabel(d, "history stays visible") {
				t.Fatal("default Commit tool window must sit beside the log")
			}
			clickControl(t, d, "Commit")
			if m.showCommit || !hasLogLabel(d, "history stays visible") {
				t.Fatal("Commit stripe must hide its pane and retain the log")
			}
			clickControl(t, d, "Branches")
			if !m.showBranches || !hasLabel(d, "Branch actions") || !hasLogLabel(d, "history stays visible") {
				t.Fatal("Branches tool window did not open beside log")
			}
			clickControl(t, d, "Branches")
			if m.showBranches {
				t.Fatal("Branches stripe did not toggle closed")
			}
			clickControl(t, d, "Commit")
			if !m.showCommit || m.showBranches || !hasLabel(d, "Commit changes") {
				t.Fatal("Commit tool window did not reopen")
			}
		})
	}
}

func TestSettingsAppliesAndPersistsTheme(t *testing.T) {
	m, d := openDriver(t, "")
	if m.appConfig.Theme != "dark" || m.theme.Background != apptheme.Dark().Background {
		t.Fatal("dark must be the default appearance")
	}
	clickControl(t, d, "Settings")
	clickControl(t, d, "Theme")
	clickControl(t, d, "light")
	clickControl(t, d, "OK")
	if m.theme.Background != apptheme.Light().Background {
		t.Fatal("appearance was not applied to the page")
	}
	saved, err := config.LoadApp(m.configDir)
	if err != nil || saved.Theme != "light" {
		t.Fatalf("appearance was not saved: %+v, %v", saved, err)
	}
	loaded, err := New(m.configDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.theme.Background != apptheme.Light().Background {
		t.Fatal("saved appearance was not applied at startup")
	}
}

func TestBranchWidgetPopupWorksBesideBranchesToolWindow(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	g.Commit("base")
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && !m.log.Loading })
	clickControl(t, d, "Branches")
	clickControl(t, d, m.branchName())
	if !m.branchOpen || !m.showBranches {
		t.Fatal("branch popup must coexist with the tool window")
	}
	clickControl(t, d, m.branchName()+" (current)")
	clickControl(t, d, "Branch actions")
	clickControl(t, d, "New branch from")
	if m.branchOpen || !m.branches.NewBranch.Open {
		t.Fatal("popup action did not open the branch dialog")
	}
	clickControl(t, d, "Cancel")
}
