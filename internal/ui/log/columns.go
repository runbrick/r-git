package log

import "github.com/vivaui/viva/ui"

// commitColumns is shared by the heading and every virtualized row. Only the
// subject absorbs spare width; a long subject must never move the metadata.
// Viva calls PlaceWidth after the pane's actual available width is known.
type commitColumns struct {
	width         float32
	showHash      bool
	hadHash       bool
	graphWidth    float32
	hadGraphWidth float32
	widths        [5]float32
	gap           float32
}

func (*commitColumns) MeasureWidth(*ui.LayoutContext) float32 { return 0 }

func (l *commitColumns) PlaceWidth(c *ui.LayoutContext, bounds ui.Rect) {
	if l.width != bounds.Width || l.widths[0] == 0 || l.hadHash != l.showHash || l.graphWidth != l.hadGraphWidth {
		l.width = bounds.Width
		l.hadHash = l.showHash
		l.hadGraphWidth = l.graphWidth
		l.gap = min(float32(12), bounds.Width/4)
		l.widths = [5]float32{max(16, l.graphWidth), 0, 120, 146, 0}
		if l.showHash {
			l.widths[4] = 76
		}
		gaps := float32(3)
		if l.showHash {
			gaps = 4
		}
		room := max(float32(0), bounds.Width-gaps*l.gap)
		fixed := l.widths[0] + l.widths[2] + l.widths[3] + l.widths[4]
		if room >= fixed {
			l.widths[1] = room - fixed
		} else {
			// Even below the supported window sizes keep all cells inside the
			// pane, with the same compact widths in the heading and rows.
			for i := range l.widths {
				l.widths[i] *= room / fixed
			}
		}
	}
	x := bounds.X
	for i := 0; i < c.Count(); i++ {
		width := l.widths[i]
		c.SetX(i, x)
		c.SetWidth(i, width)
		x += width + l.gap
	}
}

func (*commitColumns) MeasureHeight(c *ui.LayoutContext) float32 {
	var height float32
	for i := 0; i < c.Count(); i++ {
		height = max(height, c.Height(i))
	}
	return height
}

func (*commitColumns) PlaceHeight(c *ui.LayoutContext, bounds ui.Rect) {
	for i := 0; i < c.Count(); i++ {
		height := min(c.Height(i), bounds.Height)
		c.SetY(i, bounds.Y+(bounds.Height-height)/2)
		c.SetHeight(i, height)
	}
}
