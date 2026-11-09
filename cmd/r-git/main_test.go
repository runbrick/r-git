package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestPageShowsEmptyState(t *testing.T) {
	m := &model{}
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Title: "r-git", Width: 480, Height: 240}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Open a repository" {
			return
		}
	}
	t.Fatal("empty-state label was not present")
}
