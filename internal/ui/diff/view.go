// Package diff presents text comparisons as a virtualized Viva control.
package diff

import (
	"github.com/runbrick/r-git/internal/i18n"
	"strconv"
	"strings"
	"sync/atomic"

	textdiff "github.com/runbrick/r-git/internal/diff"
	"github.com/runbrick/r-git/internal/ui/controls"
	"github.com/runbrick/r-git/internal/ui/icons"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const lineHeight float32 = 24

var owners atomic.Uint32
var policies = []string{"Default", "Ignore whitespace", "Ignore trailing whitespace"}

type line struct {
	text, number string
	runs         []gfx.TextRun
	marks        []mark
}
type mark struct{ column, cells float32 }
type row struct {
	a, b    int
	changed bool
}

// View owns content, alignment, cached glyph runs and scroll state. SetContent
// invalidates all text caches. Stable TextIdentity values let Viva retain shaped
// and rasterized text across scrolling; Draw never tokenizes or shapes lines.
type View struct {
	i18n.Locale
	Title            string
	Unified          bool
	Policy           textdiff.Policy
	Result           textdiff.Result
	OnClose          func()
	a, b             []line
	rows             []row
	changes          []int
	current          int
	scrollX, scrollY float32
	height           float32
	owner            uint32
	input            ui.Handlers
	foreground       gfx.Color
	status           string
	first, last      int
}

func (v *View) SetContent(title, a, b string) {
	v.foreground = gfx.Color{}
	v.Title = title
	v.Result = textdiff.Compare(a, b, v.Policy)
	v.a = makeLines(v.Result.A)
	v.b = makeLines(v.Result.B)
	for pair, words := range v.Result.Inner {
		v.a[pair[0]].marks = makeMarks(v.Result.A[pair[0]], words.A)
		v.b[pair[1]].marks = makeMarks(v.Result.B[pair[1]], words.B)
	}
	v.owner = owners.Add(1)
	v.input.Key = v.key
	v.input.Scroll = v.scroll
	v.rebuildRows()
}

// Content embeds just the virtualized text surface, without a diff toolbar.
func (v *View) Content(c ui.C, label string) {
	ui.Control(c, label, &v.input).Fill().Clip().Draw(v.draw)
}

func makeLines(values []string) []line {
	result := make([]line, len(values))
	var cache gfx.Canvas
	style := gfx.TextStyle{Size: 13, Mono: true}
	for i, value := range values {
		value = strings.ReplaceAll(value, "\t", "    ")
		cache.Reset()
		cache.Label(value, gfx.Rect{Width: 1e8, Height: lineHeight}, style)
		text := cache.Texts()[0]
		result[i] = line{text: text.Value, number: strconv.Itoa(i + 1), runs: append([]gfx.TextRun(nil), cache.TextRuns(text)...)}
	}
	return result
}

func makeMarks(value string, spans []textdiff.Span) []mark {
	var result []mark
	var cache gfx.Canvas
	style := gfx.TextStyle{Size: 13, Mono: true}
	cell, _ := cache.MonoCellSize(style.Size, gfx.MonoSystem)
	for _, span := range spans {
		prefix := strings.ReplaceAll(value[:span.Start], "\t", "    ")
		selected := strings.ReplaceAll(value[span.Start:span.End], "\t", "    ")
		result = append(result, mark{cache.TextWidthStyle(prefix, style) / cell, cache.TextWidthStyle(selected, style) / cell})
	}
	return result
}
func (v *View) rebuildRows() {
	v.rows = v.rows[:0]
	v.changes = v.changes[:0]
	a, b := 0, 0
	for _, c := range v.Result.Changes {
		for a < c.StartA && b < c.StartB {
			v.rows = append(v.rows, row{a: a, b: b})
			a++
			b++
		}
		v.changes = append(v.changes, len(v.rows))
		if v.Unified {
			for a < c.EndA {
				v.rows = append(v.rows, row{a: a, b: -1, changed: true})
				a++
			}
			for b < c.EndB {
				v.rows = append(v.rows, row{a: -1, b: b, changed: true})
				b++
			}
		} else {
			for a < c.EndA || b < c.EndB {
				r := row{a: -1, b: -1, changed: true}
				if a < c.EndA {
					r.a = a
					a++
				}
				if b < c.EndB {
					r.b = b
					b++
				}
				v.rows = append(v.rows, r)
			}
		}
	}
	for a < len(v.a) && b < len(v.b) {
		v.rows = append(v.rows, row{a: a, b: b})
		a++
		b++
	}
	v.current = -1
	v.scrollX = 0
	v.scrollY = 0
	v.updateStatus()
}

func (v *View) updateStatus()            { v.status = v.Text().ChangePosition(v.current+1, len(v.changes)) }
func (v *View) CurrentChange() int       { return v.current }
func (v *View) VisibleRange() (int, int) { return v.first, v.last }
func (v *View) Next()                    { v.navigate(1) }
func (v *View) Previous()                { v.navigate(-1) }
func (v *View) navigate(delta int) {
	if len(v.changes) == 0 {
		return
	}
	if v.current < 0 && delta < 0 {
		v.current = 0
	}
	v.current = (v.current + delta + len(v.changes)) % len(v.changes)
	v.scrollY = float32(v.changes[v.current]) * lineHeight
	v.clamp()
	v.updateStatus()
}
func (v *View) clamp() {
	v.scrollY = max(0, min(v.scrollY, float32(len(v.rows))*lineHeight-v.height))
	v.scrollX = max(0, v.scrollX)
}
func (v *View) key(e gfx.KeyEvent) gfx.EventResult {
	if e.Key == gfx.KeyF7 {
		if e.Modifiers&gfx.ModifierShift != 0 {
			v.Previous()
		} else {
			v.Next()
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	switch e.Key {
	case gfx.KeyDown:
		v.scrollY += lineHeight
	case gfx.KeyUp:
		v.scrollY -= lineHeight
	case gfx.KeyPageDown:
		v.scrollY += v.height
	case gfx.KeyPageUp:
		v.scrollY -= v.height
	case gfx.KeyHome:
		v.scrollY = 0
	case gfx.KeyEnd:
		v.scrollY = float32(len(v.rows)) * lineHeight
	default:
		return gfx.EventResult{}
	}
	v.clamp()
	return gfx.EventResult{Handled: true, Repaint: true}
}
func (v *View) scroll(e gfx.ScrollEvent) gfx.EventResult {
	v.scrollY += e.DeltaY
	v.scrollX += e.DeltaX
	v.clamp()
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *View) Page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Pad(8).Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, v.Title).Bold().Grow(1)
			if controls.IconButton(c, icons.Pair(c, icons.Up, icons.Up_dark), v.Text().PreviousChange).Clicked() {
				v.Previous()
			}
			if controls.IconButton(c, icons.Pair(c, icons.Down, icons.Down_dark), v.Text().NextChange).Clicked() {
				v.Next()
			}
			ui.Text(c, v.status).Muted()
			old := v.Unified
			ui.Checkbox(c, &v.Unified, v.Text().Unified)
			if old != v.Unified {
				v.rebuildRows()
			}
			policy := policies[int(v.Policy)]
			ui.Select(c, &policy, policies, v.Text().PolicyChoice).Label(v.Text().ComparisonPolicy)
			if policy != policies[int(v.Policy)] {
				for i, p := range policies {
					if p == policy {
						v.Policy = textdiff.Policy(i)
					}
				}
				v.SetContent(v.Title, strings.Join(v.Result.A, "\n"), strings.Join(v.Result.B, "\n"))
			}
			if v.OnClose != nil && controls.IconButton(c, icons.Pair(c, icons.Close, icons.Close_dark), v.Text().CloseDiff).Clicked() {
				c.After(v.OnClose)
			}
		})
		ui.Control(c, v.Text().DiffContentF7NextShiftF7Previous, &v.input).Fill().Clip().Draw(v.draw)
	})
}

func (v *View) draw(d *ui.DrawContext) {
	v.height = d.Rect.Height
	v.clamp()
	// Runs are built on content change and recolored only when the theme changes.
	if v.foreground != d.Theme.Foreground {
		v.foreground = d.Theme.Foreground
		for _, lines := range [][]line{v.a, v.b} {
			for i := range lines {
				for j := range lines[i].runs {
					lines[i].runs[j].Color = v.foreground
				}
			}
		}
	}
	d.Canvas.Fill(d.Rect, d.Theme.Background)
	v.first = min(len(v.rows), int(v.scrollY/lineHeight))
	v.last = min(len(v.rows), v.first+int(d.Rect.Height/lineHeight)+2)
	cell, _ := d.Canvas.MonoCellSize(13, gfx.MonoSystem)
	width := d.Rect.Width / 2
	if v.Unified {
		width = d.Rect.Width
	}
	for i := v.first; i < v.last; i++ {
		r := v.rows[i]
		bounds := gfx.Rect{X: d.Rect.X, Y: d.Rect.Y + float32(i)*lineHeight - v.scrollY, Width: width, Height: lineHeight}
		if v.Unified {
			if r.b >= 0 {
				v.drawLine(d, bounds, r.b, false, r.changed && r.a < 0, cell)
			} else {
				v.drawLine(d, bounds, r.a, true, r.changed, cell)
			}
		} else {
			v.drawLine(d, bounds, r.a, true, r.changed, cell)
			bounds.X += width
			v.drawLine(d, bounds, r.b, false, r.changed, cell)
		}
		if v.current >= 0 && i == v.changes[v.current] {
			d.Canvas.Fill(gfx.Rect{X: d.Rect.X, Y: bounds.Y, Width: d.Rect.Width, Height: 2}, d.Theme.Primary)
		}
	}
	if !v.Unified {
		d.Canvas.Fill(gfx.Rect{X: d.Rect.X + width, Y: d.Rect.Y, Width: 1, Height: d.Rect.Height}, d.Theme.Border)
	}
}

func (v *View) drawLine(d *ui.DrawContext, bounds gfx.Rect, index int, left, changed bool, cell float32) {
	if index < 0 {
		return
	}
	lines := v.b
	side := uint64(1)
	color := d.Theme.Success
	if left {
		lines = v.a
		side = 0
		color = d.Theme.Danger
	}
	l := &lines[index]
	d.Canvas.Clip(bounds)
	if changed {
		tint := color
		tint.Alpha *= 0.12
		d.Canvas.Fill(bounds, tint)
	}
	d.Canvas.Label(l.number, gfx.Rect{X: bounds.X + 4, Y: bounds.Y, Width: 44, Height: lineHeight}, gfx.TextStyle{Size: 11, Color: d.Theme.MutedForeground})
	textBounds := gfx.Rect{X: bounds.X + 52 - v.scrollX, Y: bounds.Y, Width: max(bounds.Width-52+v.scrollX, 1), Height: lineHeight}
	d.Canvas.Clip(gfx.Rect{X: bounds.X + 48, Y: bounds.Y, Width: max(0, bounds.Width-48), Height: lineHeight})
	for _, m := range l.marks {
		tint := color
		tint.Alpha *= 0.32
		d.Canvas.Fill(gfx.Rect{X: textBounds.X + float32(m.column)*cell, Y: bounds.Y + 2, Width: float32(m.cells) * cell, Height: lineHeight - 4}, tint)
	}
	d.Canvas.LabelRunsWithIdentity(gfx.TextIdentity{Owner: v.owner, Item: uint64(index)*2 + side}, l.text, textBounds, gfx.TextStyle{Size: 13, Color: d.Theme.Foreground, Mono: true}, cell, l.runs)
	d.Canvas.Unclip()
	d.Canvas.Unclip()
}

func (v *View) SetMessages(messages *i18n.Messages) { v.UseMessages(messages); v.updateStatus() }
