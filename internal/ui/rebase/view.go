// Package rebase edits PR5c's todo model before Git starts rewriting commits.
package rebase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/vivaui/viva/ui"
)

var errPreview = errors.New("rebase preview captured")
var ErrHistoryChanged = errors.New("rebase history changed; load the commits again")

// Prepare asks Git for the real todo and cancels in the editor, before changes.
func Prepare(ctx context.Context, o ops.Operations, upstream string) ([]ops.RebaseEntry, error) {
	var entries []ops.RebaseEntry
	r := o.InteractiveRebase(ctx, upstream, func(todo []ops.RebaseEntry) ([]ops.RebaseEntry, error) {
		entries = append([]ops.RebaseEntry(nil), todo...)
		return nil, errPreview
	})
	if errors.Is(r.Err, errPreview) {
		return entries, nil
	}
	if r.Err != nil {
		return nil, r.Err
	}
	return nil, errors.New("no commits to rebase")
}

// Apply rechecks the captured plan, so HEAD changes cannot silently rewrite a
// different set of commits than those the user reviewed.
func Apply(ctx context.Context, o ops.Operations, upstream string, original, edited []ops.RebaseEntry) ops.RebaseResult {
	return o.InteractiveRebase(ctx, upstream, func(todo []ops.RebaseEntry) ([]ops.RebaseEntry, error) {
		if len(todo) != len(original) {
			return nil, ErrHistoryChanged
		}
		for i, e := range todo {
			if e.Commit != original[i].Commit {
				return nil, ErrHistoryChanged
			}
		}
		return Validate(edited)
	})
}

func Validate(entries []ops.RebaseEntry) ([]ops.RebaseEntry, error) {
	rewords := 0
	kept := false
	for _, entry := range entries {
		switch entry.Action {
		case "pick", "reword", "edit", "squash", "fixup", "drop":
		default:
			return nil, fmt.Errorf("%w: unsupported action %s", ops.ErrInvalidTodo, entry.Action)
		}
		if strings.ContainsAny(entry.Subject, "\r\n\x00") {
			return nil, fmt.Errorf("%w: subject must be one line", ops.ErrInvalidTodo)
		}
		if entry.Action == "reword" {
			rewords++
			if strings.TrimSpace(entry.Subject) == "" {
				return nil, fmt.Errorf("%w: reword needs a message", ops.ErrInvalidTodo)
			}
		}
		if (entry.Action == "squash" || entry.Action == "fixup") && !kept {
			return nil, fmt.Errorf("%w: %s needs a previous kept commit", ops.ErrInvalidTodo, entry.Action)
		}
		if entry.Action != "drop" {
			kept = true
		}
	}
	// PR5c currently supplies one message file for all reword actions.
	if rewords > 1 {
		return nil, fmt.Errorf("%w: reword one commit per rebase", ops.ErrInvalidTodo)
	}
	return ops.ApplyTodoModel(entries)
}

var actions = []string{"pick", "reword", "edit", "squash", "fixup", "drop"}

type View struct {
	Open              bool
	Upstream          string
	Entries, Original []ops.RebaseEntry
	Busy              bool
	Error, Status     string
	OnLoad            func(string)
	OnApply           func(string, []ops.RebaseEntry, []ops.RebaseEntry)
	loadedUpstream    string
}

func (v *View) SetEntries(upstream string, entries []ops.RebaseEntry) {
	v.loadedUpstream = upstream
	v.Entries = append([]ops.RebaseEntry(nil), entries...)
	v.Original = append([]ops.RebaseEntry(nil), entries...)
	v.Busy = false
	v.Error = ""
	v.Status = ""
}
func (v *View) Move(index, delta int) {
	other := index + delta
	if index < 0 || index >= len(v.Entries) || other < 0 || other >= len(v.Entries) {
		return
	}
	v.Entries[index], v.Entries[other] = v.Entries[other], v.Entries[index]
}

func (v *View) Page(c ui.C) {
	ui.Modal(c, &v.Open, "Interactive rebase", func() {
		ui.Column(c).Width(760).Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.TextField(c, &v.Upstream, "Upstream", "HEAD~2").Grow(1).Disabled(v.Busy)
				if ui.Button(c, "Load commits").Disabled(v.Busy || v.Upstream == "").Clicked() && v.OnLoad != nil {
					upstream := v.Upstream
					c.After(func() { v.OnLoad(upstream) })
				}
			})
			ui.Scroll(c).FillWidth().Height(280).Gap(4).Children(func() {
				for i := range v.Entries {
					index := i
					entry := &v.Entries[i]
					ui.Row(c.Key(entry.Commit)).FillWidth().Gap(8).Items(ui.Center).Children(func() {
						if !v.Busy {
							ui.Select(c, &entry.Action, actions, nil).Label(fmt.Sprintf("Action %d", i+1))
						} else {
							ui.Text(c, entry.Action)
						}
						ui.Text(c, entry.Commit).Width(76).Mono()
						if entry.Action == "reword" {
							ui.TextField(c, &entry.Subject, fmt.Sprintf("Message %d", i+1), "Commit message").Grow(1).Disabled(v.Busy)
						} else {
							ui.Text(c, entry.Subject).Grow(1)
						}
						if ui.Button(c, fmt.Sprintf("Move up %d", i+1)).Disabled(v.Busy || i == 0).Clicked() {
							c.After(func() { v.Move(index, -1) })
						}
						if ui.Button(c, fmt.Sprintf("Move down %d", i+1)).Disabled(v.Busy || i == len(v.Entries)-1).Clicked() {
							c.After(func() { v.Move(index, 1) })
						}
					})
				}
			})
			if v.Error != "" {
				ui.Text(c, v.Error).Color(c.Theme().Danger)
			}
			if v.Status != "" {
				ui.Text(c, v.Status).Muted()
			}
			if v.Busy {
				ui.Text(c, "Running Git…").Muted()
			}
		})
	}, func() {
		if ui.Button(c, "Cancel").Disabled(v.Busy).Clicked() {
			v.Open = false
		}
		_, err := Validate(v.Entries)
		if ui.Button(c, "Start rebase").Disabled(v.Busy || len(v.Entries) == 0 || err != nil || v.Upstream != v.loadedUpstream).Clicked() && v.OnApply != nil {
			upstream := v.Upstream
			original, edited := append([]ops.RebaseEntry(nil), v.Original...), append([]ops.RebaseEntry(nil), v.Entries...)
			c.After(func() { v.OnApply(upstream, original, edited) })
		}
		if err != nil {
			ui.Text(c, err.Error()).Color(c.Theme().Danger)
		}
	})
}
