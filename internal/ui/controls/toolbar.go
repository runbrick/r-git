// Package controls holds r-git's shared toolbar presentation.
package controls

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// IconButton adapts Viva examples/recipes.IconButton to the page API. The base
// supplies button semantics and keyboard activation; the label is also its tip.
func IconButton(c ui.C, icon gfx.Icon, label string) ui.Box {
	t := c.Theme()
	button := ui.ButtonBase(c, label).Size(28, 28).Shrink(0).
		Radius(t.Metrics.RadiusControl).Tooltip(label).Transition().
		Hover(ui.Look{}.Background(t.Hover)).
		Pressed(ui.Look{}.Background(t.Muted)).
		Focus(ui.Look{}.Ring(2, t.Ring))
	return button.Children(func() {
		ui.Icon(c, icon).Size(16, 16).StateFrom(button).
			Hover(ui.Look{}.Color(t.Foreground))
	})
}
