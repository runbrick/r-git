// Package merge owns conflict selection and an editable three-pane merge.
package merge

import (
	"fmt"

	textdiff "github.com/runbrick/r-git/internal/diff"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/ui/controls"
	diffview "github.com/runbrick/r-git/internal/ui/diff"
	"github.com/runbrick/r-git/internal/ui/icons"
	"github.com/vivaui/viva/ui"
)

type View struct {
	Conflicts []ops.Conflict
	Path      string
	Content   ops.ConflictContent
	Merge     *textdiff.Merge
	Result    string
	Busy      bool
	Error     string
	OnSelect  func(ops.Conflict)
	OnResolve func(string, []byte)
	OnClose   func()
	// OnEdit returns the workflow request current when the user edits the result.
	OnEdit      func() uint64
	left, right diffview.View
	manual      bool
	resultField ui.Field
	hunks       ui.ListState
	// Edit is the workflow request current when the user last changed the
	// result. A file load requested earlier must not replace that result.
	Edit uint64
}

func (v *View) SetFile(path string, content ops.ConflictContent) {
	v.Path, v.Content = path, content
	v.Merge = textdiff.NewMerge(content.Base, content.Left, content.Right)
	v.Result = v.Merge.Result()
	v.resultField.SetText(v.Result)
	v.manual = false
	v.hunks = ui.ListState{}
	v.Error = ""
	v.Busy = false
	v.left.Unified = true
	v.right.Unified = true
	v.left.SetContent(content.LeftLabel, content.Left, content.Left)
	v.right.SetContent(content.RightLabel, content.Right, content.Right)
}

func (v *View) noteEdit() {
	v.manual = true
	if v.OnEdit != nil {
		v.Edit = v.OnEdit()
	}
}

func (v *View) choose(index int, side textdiff.Side) {
	if v.manual || v.Merge == nil || index < 0 || index >= len(v.Merge.Chunks) {
		return
	}
	v.Merge.Chunks[index].Choice = side
	v.Result = v.Merge.Result()
	v.resultField.SetText(v.Result)
	if v.OnEdit != nil {
		v.Edit = v.OnEdit()
	}
}

func (v *View) Page(c ui.C) {
	ui.Column(c).Fill().Gap(8).Pad(12).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, "Conflicts").Bold().Grow(1)
			if v.OnClose != nil && controls.IconButton(c, icons.Pair(c, icons.Close, icons.Close_dark), "Close conflicts").Clicked() {
				c.After(v.OnClose)
			}
		})
		ui.Row(c).Fill().Gap(12).Children(func() {
			ui.Scroll(c).Width(200).FillHeight().Gap(4).Children(func() {
				for _, conflict := range v.Conflicts {
					item := conflict
					if ui.Button(c.Key(item.Path), item.Path).Ghost().Disabled(v.Busy).Clicked() && v.OnSelect != nil {
						c.After(func() { v.OnSelect(item) })
					}
				}
			})
			ui.Column(c).Fill().Gap(8).Children(func() {
				if v.Error != "" {
					ui.Text(c, v.Error).Color(c.Theme().Danger)
				}
				if v.Busy {
					ui.Text(c, "Loading or saving conflict…").Muted()
				}
				if v.Merge == nil {
					ui.Text(c, "Select a conflicted file").Muted()
					return
				}
				ui.Text(c, v.Path).Bold()
				ui.Row(c).Fill().Gap(8).Children(func() {
					ui.Column(c).Grow(1).FillHeight().Gap(4).Children(func() { ui.Text(c, v.Content.LeftLabel).Bold(); v.left.Content(c, "Yours content") })
					ui.Column(c).Grow(1).FillHeight().Gap(4).Children(func() {
						ui.Text(c, "Result").Bold()
						area := ui.TextArea(c, &v.resultField, "Merge result", "").Fill().Disabled(v.Busy)
						// TextArea bound to a string keeps its own copy and writes
						// that copy back on the next frame, which replaced an
						// Accept click with the previous result.
						if area.Changed() {
							v.Result = v.resultField.Text()
							v.manual = true
						}
					})
					ui.Column(c).Grow(1).FillHeight().Gap(4).Children(func() { ui.Text(c, v.Content.RightLabel).Bold(); v.right.Content(c, "Theirs content") })
				})
				ui.List(c, &v.hunks, len(v.Merge.Chunks), func(row ui.ListRow) {
					i := row.Index
					chunk := v.Merge.Chunks[i]
					c := row.C
					row.Element.Height(40).Gap(8).Items(ui.Center).Children(func() {
						kind := "Change"
						if chunk.Conflict {
							kind = "Conflict"
						}
						ui.Text(c, fmt.Sprintf("%s %d · base lines %d–%d", kind, i+1, chunk.Start+1, max(chunk.Start+1, chunk.End))).Grow(1)
						if chunk.Choice != textdiff.Unresolved {
							ui.Text(c, "Accepted").Color(c.Theme().Success)
						}
						if controls.IconButton(c, icons.Pair(c, icons.Left, icons.Left_dark), fmt.Sprintf("Accept Yours %d", i+1)).Disabled(v.Busy || v.manual).Clicked() {
							v.choose(i, textdiff.Left)
						}
						if controls.IconButton(c, icons.Pair(c, icons.Right, icons.Right_dark), fmt.Sprintf("Accept Theirs %d", i+1)).Disabled(v.Busy || v.manual).Clicked() {
							v.choose(i, textdiff.Right)
						}
					})
				}).Label("Conflict chunks").FillWidth().Height(120).Shrink(0)
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					if ui.Button(c, "Accept Yours").Outline().Disabled(v.Busy).Clicked() {
						v.Result = v.Content.Left
						v.resultField.SetText(v.Result)
						v.noteEdit()
					}
					if ui.Button(c, "Accept Theirs").Outline().Disabled(v.Busy).Clicked() {
						v.Result = v.Content.Right
						v.resultField.SetText(v.Result)
						v.noteEdit()
					}
					if ui.Button(c, "Apply non-conflicting changes").Outline().Disabled(v.Busy || v.manual).Clicked() {
						v.Merge.ApplyNonConflicting()
						v.Result = v.Merge.Result()
						v.resultField.SetText(v.Result)
						v.noteEdit()
					}
					if v.manual {
						ui.Text(c, "Result edited manually").Muted()
						if ui.Button(c, "Reset result to chunk choices").Outline().Disabled(v.Busy).Clicked() {
							v.manual = false
							v.Result = v.Merge.Result()
							v.resultField.SetText(v.Result)
						}
					}
					ui.Spacer(c)
					if ui.Button(c, "Mark resolved").Outline().Disabled(v.Busy || !v.manual && v.Merge.Remaining() > 0).Clicked() && v.OnResolve != nil {
						path, data := v.Path, []byte(v.Result)
						c.After(func() { v.OnResolve(path, data) })
					}
				})
			})
		})
	})
}
