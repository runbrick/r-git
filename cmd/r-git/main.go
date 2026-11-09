package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

type model struct {
	view flex.View
}

func (m *model) page(canvas *ui.Canvas, size ui.Size) {
	theme := ui.LightTheme()
	bounds := ui.Rect{Width: size.Width, Height: size.Height}
	canvas.Fill(bounds, theme.Background)
	m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
		f.Column().Fill().Pad(24).Children(func() {
			f.Text("Open a repository")
		})
	})
}

func main() {
	m := &model{}
	if err := app.Run(app.Config{
		Title: "r-git",
		Paint: m.page,
	}); err != nil {
		log.Fatal(err)
	}
}
