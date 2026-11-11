package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/runbrick/r-git/internal/git/command"
	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/ops"
	"github.com/runbrick/r-git/internal/git/repo"
	diffview "github.com/runbrick/r-git/internal/ui/diff"
	mergeview "github.com/runbrick/r-git/internal/ui/merge"
	rebaseview "github.com/runbrick/r-git/internal/ui/rebase"
	"github.com/vivaui/viva/ui"
)

type workflowKind uint8

const (
	workflowDiff workflowKind = iota
	workflowConflicts
	workflowFile
	workflowResolved
	workflowTodo
	workflowRebased
)

type workflowResult struct {
	kind       workflowKind
	request    uint64
	err        error
	path, a, b string
	conflicts  []ops.Conflict
	content    ops.ConflictContent
	entries    []ops.RebaseEntry
	rebase     ops.RebaseResult
}
type tools struct {
	diff    diffview.View
	merge   mergeview.View
	rebase  rebaseview.View
	screen  string
	busy    bool
	request uint64
	err     string
}

func (m *Model) initTools() {
	m.OnShowDiff = m.ShowDiff
	m.log.OnShowDiff = func(hash model.Hash, path string) {
		if m.OnShowDiff != nil {
			m.OnShowDiff(hash, path)
		}
	}
	m.tools.diff.OnClose = func() { m.tools.screen = "" }
	m.tools.merge.OnClose = func() { m.tools.screen = "" }
	m.tools.merge.OnSelect = m.openConflict
	m.tools.merge.OnEdit = func() uint64 { return m.tools.request }
	m.tools.merge.OnResolve = m.resolveConflict
	m.tools.rebase.Upstream = "HEAD~2"
	m.tools.rebase.OnLoad = m.prepareRebase
	m.tools.rebase.OnApply = m.applyRebase
}

func (m *Model) resetTools() {
	m.tools.request++
	m.tools.screen = ""
	m.tools.busy = false
	m.tools.rebase.Open = false
	m.tools.merge.Merge = nil
	m.tools.err = ""
	m.tools.merge.Busy = false
	m.tools.rebase.Busy = false
	m.tools.rebase.Entries = nil
	m.tools.rebase.Original = nil
}
func (m *Model) toolsPage(c ui.C) bool {
	if m.tools.err != "" {
		ui.Text(c, m.tools.err).Color(c.Theme().Danger).Pad(8)
	}
	if m.tools.busy {
		ui.Text(c, "Running Git…").Muted().Pad(8)
		// A spinner asks for the next frame. Headless WaitFor delivers that
		// frame through OnWake. A cross-thread Invalidate during WaitFor does
		// not paint on Linux.
		ui.Spinner(c)
	}
	if m.repo != nil && m.repo.State == model.StateRebasing {
		ui.Row(c).FillWidth().Pad(8).Gap(8).Children(func() {
			ui.Text(c, "Rebase in progress").Grow(1)
			if ui.Button(c, "Continue rebase").Disabled(m.tools.busy).Clicked() {
				c.After(func() { m.controlRebase("continue") })
			}
			if ui.Button(c, "Skip commit").Disabled(m.tools.busy).Clicked() {
				c.After(func() { m.controlRebase("skip") })
			}
			if ui.Button(c, "Abort rebase").Disabled(m.tools.busy).Clicked() {
				c.After(func() { m.controlRebase("abort") })
			}
		})
	}
	switch m.tools.screen {
	case "diff":
		m.tools.diff.Page(c)
		return true
	case "merge":
		m.tools.merge.Page(c)
		return true
	}
	return false
}
func (m *Model) toolsToolbar(c ui.C) {
	if m.repo == nil {
		return
	}
	if ui.Button(c, "Conflicts").Disabled(m.tools.busy).Clicked() {
		c.After(m.showConflicts)
	}
	if ui.Button(c, "Interactive rebase").Disabled(m.tools.busy).Clicked() {
		m.tools.rebase.Open = true
	}
}

// runWorkflow uses the shell's owned worker group, cancellation, publication
// queue and wake callback; every goroutine exits after one requested operation.
func (m *Model) runWorkflow(work func(context.Context, *repo.Repo) *workflowResult) {
	if m.repo == nil || m.tools.busy {
		return
	}
	m.tools.busy = true
	m.tools.err = ""
	m.tools.request++
	request, generation, repository, ctx := m.tools.request, m.generation, m.repo, m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		result := work(ctx, repository)
		result.request = request
		m.publish(&loadResult{kind: resultWorkflow, generation: generation, workflow: result})
	}()
}

// ShowDiff is the changes-tree callback as well as the log callback. A zero
// hash compares HEAD with the working file; a commit compares its first parent.
func (m *Model) ShowDiff(hash model.Hash, path string) {
	m.runWorkflow(func(ctx context.Context, r *repo.Repo) *workflowResult {
		out := &workflowResult{kind: workflowDiff, path: path}
		out.a, out.b, out.err = readDiff(ctx, r, hash, path)
		return out
	})
}

func readDiff(ctx context.Context, r *repo.Repo, hash model.Hash, path string) (string, string, error) {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) == ".." || strings.HasPrefix(filepath.Clean(path), ".."+string(filepath.Separator)) {
		return "", "", errors.New("invalid diff path")
	}
	read := func(rev, path string) (string, error) {
		res, err := r.Executor.Run(ctx, command.Cmd{Dir: r.Path, Subcommand: "ls-tree", Args: []string{"--name-only", "-z", rev, "--", path}})
		if err != nil {
			return "", err
		}
		if len(res.Stdout) == 0 {
			return "", nil
		}
		res, err = r.Executor.Run(ctx, command.Cmd{Dir: r.Path, Subcommand: "show", Args: []string{"--end-of-options", rev + ":" + path}})
		return string(res.Stdout), err
	}
	var a, b string
	var err error
	if hash.IsZero() {
		if !r.HEAD.IsZero() {
			a, err = read("HEAD", path)
		}
		if err == nil {
			var data []byte
			data, err = os.ReadFile(filepath.Join(r.Path, path))
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			b = string(data)
		}
	} else {
		var res command.Result
		res, err = r.Executor.Run(ctx, command.Cmd{Dir: r.Path, Subcommand: "rev-list", Args: []string{"--parents", "-n", "1", hash.String()}})
		if err == nil {
			parents := strings.Fields(string(res.Stdout))
			oldPath := path
			var files []model.FileChange
			files, err = r.ChangedFiles(ctx, hash)
			if err == nil {
				for _, file := range files {
					if file.Path == path && file.OrigPath != "" {
						oldPath = file.OrigPath
					}
				}
				if len(parents) > 1 {
					a, err = read(parents[1], oldPath)
				}
			}
			if err == nil {
				b, err = read(hash.String(), path)
			}
		}
	}
	if err == nil && (!utf8.ValidString(a) || !utf8.ValidString(b) || strings.ContainsRune(a, 0) || strings.ContainsRune(b, 0)) {
		err = errors.New("binary file: text diff is unavailable")
	}
	return a, b, err
}

func (m *Model) showConflicts() {
	m.runWorkflow(func(ctx context.Context, r *repo.Repo) *workflowResult {
		out := &workflowResult{kind: workflowConflicts}
		out.conflicts, out.err = (ops.Operations{Dir: r.Path, Git: r.Executor}).Conflicts(ctx)
		return out
	})
}
func (m *Model) openConflict(conflict ops.Conflict) {
	m.runWorkflow(func(ctx context.Context, r *repo.Repo) *workflowResult {
		out := &workflowResult{kind: workflowFile, path: conflict.Path}
		out.content, out.err = (ops.Operations{Dir: r.Path, Git: r.Executor}).ConflictContent(ctx, conflict)
		return out
	})
}
func (m *Model) resolveConflict(path string, data []byte) {
	if m.repo == nil {
		return
	}
	// Resolve on the window thread. A background workflow's wake does not
	// paint during a headless WaitFor on Linux, so the conflict list never
	// returned to the empty state.
	m.tools.busy = true
	m.tools.merge.Busy = true
	o := ops.Operations{Dir: m.repo.Path, Git: m.repo.Executor}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	err := o.Resolve(ctx, path, data)
	var conflicts []ops.Conflict
	if err == nil {
		conflicts, err = o.Conflicts(ctx)
	}
	m.tools.busy = false
	m.tools.merge.Busy = false
	if err != nil {
		m.tools.merge.Error = err.Error()
		m.log.SetError(err)
		return
	}
	m.tools.merge.Conflicts = conflicts
	m.tools.merge.Merge = nil
	m.tools.merge.Error = ""
	if len(conflicts) > 0 {
		content, readErr := o.ConflictContent(ctx, conflicts[0])
		if readErr != nil {
			m.tools.merge.Error = readErr.Error()
			return
		}
		m.tools.merge.SetFile(conflicts[0].Path, content)
	}
}
func (m *Model) prepareRebase(upstream string) {
	m.runWorkflow(func(ctx context.Context, r *repo.Repo) *workflowResult {
		out := &workflowResult{kind: workflowTodo, path: upstream}
		out.entries, out.err = rebaseview.Prepare(ctx, ops.Operations{Dir: r.Path, Git: r.Executor}, upstream)
		return out
	})
}
func (m *Model) applyRebase(upstream string, original, edited []ops.RebaseEntry) {
	m.runWorkflow(func(ctx context.Context, r *repo.Repo) *workflowResult {
		out := &workflowResult{kind: workflowRebased}
		out.rebase = rebaseview.Apply(ctx, ops.Operations{Dir: r.Path, Git: r.Executor}, upstream, original, edited)
		out.err = out.rebase.Err
		return out
	})
}

func (m *Model) applyWorkflow(out *workflowResult) {
	if out == nil || out.request != m.tools.request {
		return
	}
	m.tools.busy = false
	m.tools.merge.Busy = false
	m.tools.rebase.Busy = false
	if out.err != nil {
		m.tools.err = out.err.Error()
		m.log.SetError(out.err)
		m.tools.merge.Error = out.err.Error()
		m.tools.rebase.Error = out.err.Error()
		return
	}
	switch out.kind {
	case workflowDiff:
		m.tools.diff.SetContent(out.path, out.a, out.b)
		m.tools.screen = "diff"
	case workflowConflicts, workflowResolved:
		m.tools.merge.Conflicts = out.conflicts
		m.tools.merge.Merge = nil
		m.tools.merge.Error = ""
		m.tools.screen = "merge"
		if len(out.conflicts) > 0 {
			// Read the first file here, on the window thread, and mark the view
			// busy for that read only. A headless WaitFor on Linux does not
			// paint a later cross-thread wake, so a background load left the
			// accept buttons disabled on "Loading…".
			first := out.conflicts[0]
			m.tools.merge.Busy = true
			content, err := (ops.Operations{Dir: m.repo.Path, Git: m.repo.Executor}).ConflictContent(context.Background(), first)
			m.tools.merge.Busy = false
			if err != nil {
				m.tools.merge.Error = err.Error()
				break
			}
			m.tools.merge.SetFile(first.Path, content)
		}
	case workflowFile:
		// Pump applies a queued load before the click that follows a tree
		// read. A load requested before the user edited the result must not
		// replace that edit.
		if out.request <= m.tools.merge.Edit && m.tools.merge.Path == out.path {
			break
		}
		m.tools.merge.Edit = 0
		m.tools.merge.SetFile(out.path, out.content)
		m.tools.screen = "merge"
	case workflowTodo:
		m.tools.rebase.SetEntries(out.path, out.entries)
	case workflowRebased:
		m.tools.rebase.Status = fmt.Sprintf("Rebase: %v", out.rebase.Outcome)
		switch out.rebase.Outcome {
		case ops.OutcomeDone, ops.OutcomeNothingToDo:
			m.tools.rebase.Open = false
			m.resetTools()
			m.startOpen(m.repo.Path)
		case ops.OutcomeConflicts:
			m.repo.State = model.StateRebasing
			m.tools.rebase.Open = false
			m.showConflicts()
		case ops.OutcomeStopped:
			m.repo.State = model.StateRebasing
			m.tools.rebase.Open = false
			m.tools.err = "Rebase stopped for edit; amend the commit, then continue the rebase."
			m.tools.rebase.Status = "Rebase stopped for edit; amend the commit, then continue the rebase."
		}
	}
}

func (m *Model) controlRebase(action string) {
	m.runWorkflow(func(ctx context.Context, r *repo.Repo) *workflowResult {
		o := ops.Operations{Dir: r.Path, Git: r.Executor}
		out := &workflowResult{kind: workflowRebased}
		switch action {
		case "continue":
			out.rebase = o.ContinueRebase(ctx)
		case "skip":
			out.rebase = o.SkipRebase(ctx)
		case "abort":
			out.rebase = o.AbortRebase(ctx)
		}
		out.err = out.rebase.Err
		return out
	})
}
