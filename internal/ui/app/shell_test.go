package app

import (
	"testing"

	"github.com/runbrick/r-git/internal/config"
	"github.com/runbrick/r-git/internal/testutil"
	apptheme "github.com/runbrick/r-git/internal/ui/theme"
)

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
