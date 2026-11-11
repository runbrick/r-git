package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/graph/printer"
	"github.com/runbrick/r-git/internal/testutil"
	apptheme "github.com/runbrick/r-git/internal/ui/theme"
	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestLogRowsMatchGraphAndHeadsHaveNoUpwardEdge(t *testing.T) {
	g := testutil.NewDateSkewedHistory(t)
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && !m.log.Loading && len(m.log.Commits) == 6 })
	ordered, err := m.repo.ParentsOrderedAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if m.log.Commits[0].Subject != "Start UI work" {
		t.Fatalf("unexpected first row %q", m.log.Commits[0].Subject)
	}
	for i := range ordered {
		commit, ok := m.log.VisibleCommit(i)
		if !ok {
			t.Fatalf("missing display row %d", i)
		}
		graphRow := m.log.GraphIndex[commit.Hash]
		cell := m.log.Graph.Row(graphRow)
		if graphRow != i || len(cell.Nodes) != 1 || !commit.Hash.Equal(ordered[cell.Nodes[0].NodeIndex].Commit) {
			t.Fatalf("display row %d (%s) does not match graph row %d (%+v)", i, commit.Hash, graphRow, cell)
		}
		for _, edge := range cell.Edges {
			if i == 0 && edge.EdgePrintType == printer.EdgeUp {
				t.Fatalf("first row has an upward edge: %+v", edge)
			}
		}
	}
	// Filtering keeps each displayed commit's own topological cell.
	m.log.SetFilter("Login returns bool")
	commit, ok := m.log.VisibleCommit(0)
	if !ok || commit.Subject != "Login returns bool" || m.log.GraphIndex[commit.Hash] == 0 {
		t.Fatal("filter must retain the selected commit's graph identity")
	}
}

func TestLogToolbarTextFitsAtWindowWidths(t *testing.T) {
	g := testutil.NewDateSkewedHistory(t)
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && !m.log.Loading })
	for _, width := range []int{1200, 1400, 1600} {
		for _, date := range []string{"Any date", "Before today", "Last 30 days"} {
			t.Run(fmt.Sprintf("%d/%s", width, date), func(t *testing.T) {
				m.log.Date = date
				if err := d.Resize(width, 900); err != nil {
					t.Fatal(err)
				}
				s := d.Snapshot()
				for _, tc := range []struct{ control, text string }{
					{"Filter", "Text or hash"}, {"Regex", ".*"}, {"Match case", "Cc"},
					{"Branch", "All branches"}, {"User", "User"}, {"Date", date},
				} {
					var bounds gfx.Rect
					for _, hit := range s.Hits {
						if hit.Label == tc.control {
							bounds = hit.Bounds
							break
						}
					}
					found := false
					for i, text := range s.Texts {
						if text.Value != tc.text || text.Bounds.Y < bounds.Y || text.Bounds.Y >= bounds.Y+bounds.Height {
							continue
						}
						found = true
						if s.TextWidths[i] > text.Bounds.Width+0.5 || text.Bounds.X < bounds.X || text.Bounds.X+s.TextWidths[i] > bounds.X+bounds.Width+0.5 {
							t.Errorf("%s text needs %.1fpx, text bounds %v, control %v", tc.control, s.TextWidths[i], text.Bounds, bounds)
						}
						if text.Bounds.X+s.TextWidths[i] > float32(width) {
							t.Errorf("%s extends past window", tc.control)
						}
						if text.HasClip && (text.Bounds.X < text.ClipOf.X || text.Bounds.X+s.TextWidths[i] > text.ClipOf.X+text.ClipOf.Width+0.5) {
							t.Errorf("%s text is clipped by %v", tc.control, text.ClipOf)
						}
					}
					if !found {
						t.Errorf("missing %s text %q", tc.control, tc.text)
					}
				}
			})
		}
	}
}

func TestLogColumnsAlignAtWindowWidths(t *testing.T) {
	g := testutil.NewRepo(t)
	g.WriteFile("base.txt", "base\n")
	subject := strings.TrimSpace(strings.Repeat("A long commit subject must leave room for metadata ", 8))
	g.Commit(subject)
	g.Git("commit", "--amend", "--no-edit", "--date=2001-02-03T12:00:00Z")
	m, d := openDriver(t, g.Dir)
	waitFor(t, d, func() bool { return m.repo != nil && !m.log.Loading && len(m.log.Details) > 0 })
	for _, showHash := range []bool{false, true} {
		m.log.ShowHash = showHash
		for _, width := range []int{1200, 1400, 1600, 1400, 1200} {
			t.Run(fmt.Sprintf("%d/hash=%v", width, showHash), func(t *testing.T) {
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
				var list gfx.Rect
				for _, hit := range snapshot.Hits {
					if hit.Container && hit.Role == gfx.RoleListBox &&
						row.X >= hit.Bounds.X && row.X < hit.Bounds.X+hit.Bounds.Width &&
						row.Y >= hit.Bounds.Y && row.Y < hit.Bounds.Y+hit.Bounds.Height {
						list = hit.Bounds
						break
					}
				}
				if list.Width == 0 {
					t.Fatal("log list bounds are missing")
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
				columns := []struct{ heading, cell string }{
					{"Author", m.log.Commits[0].Author.Name},
					{"Date", m.log.Commits[0].Author.When.Format("2/1/06, 3:04 pm")},
				}
				if showHash {
					columns = append(columns, struct{ heading, cell string }{"Hash", m.log.Commits[0].Hash.Short()})
				}
				for _, column := range columns {
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
					// The changed-files tree now starts alongside the log. Its
					// text belongs to a separate list and can share this row's Y.
					if cell.Bounds.X >= list.X+list.Width && cell.Value == "base.txt" {
						continue
					}
					if cell.Bounds.X < list.X-0.5 || cell.Bounds.X+cell.Bounds.Width > list.X+list.Width+0.5 {
						t.Errorf("cell %q (%v) extends past log list %v", cell.Value, cell.Bounds, list)
					}
					ellipsis = ellipsis || strings.HasSuffix(cell.Value, "\u2026")
				}
				if !ellipsis {
					t.Fatal("long subject must end with an ellipsis")
				}
			})
		}
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
