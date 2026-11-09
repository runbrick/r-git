package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestPageShowsEmptyState(t *testing.T) {
	m := &model{}
	driver, err := app.NewDriver(app.Config{
		Title: "r-git",
		Paint: m.page,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, "Open a repository") {
			return
		}
	}
	t.Fatal("empty-state label was not present")
}
