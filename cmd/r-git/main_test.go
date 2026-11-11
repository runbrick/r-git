package main

import (
	"path/filepath"
	"testing"

	uiapp "github.com/runbrick/r-git/internal/ui/app"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestPageShowsEmptyState(t *testing.T) {
	m, err := uiapp.New(filepath.Join(t.TempDir(), "config"), "")
	if err != nil {
		t.Fatal(err)
	}
	page := ui.NewPage(ui.PageConfig{View: m.Page})
	driver, err := app.NewDriver(page.Config(app.Config{Title: "r-git", Width: 720, Height: 480}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Repository path" {
			return
		}
	}
	t.Fatal("empty-state label was not present")
}
