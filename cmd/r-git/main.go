package main

import (
	"log"

	"github.com/runbrick/r-git/internal/git/ops"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct{}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Children(func() {
		ui.Button(c, "Open a repository")
	})
}

func main() {
	if ops.HandleAskpass() {
		return
	}
	m := &model{}
	if err := ui.Run(app.Config{Title: "r-git"}, m.page); err != nil {
		log.Fatal(err)
	}
}
