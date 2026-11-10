package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RebaseOptions struct {
	Interactive    bool
	Onto           string
	PreserveMerges bool
	Autosquash     bool
	UpdateRefs     bool
}

func (o Operations) Rebase(ctx context.Context, upstream string, options RebaseOptions) RebaseResult {
	args := []string{}
	if options.Interactive {
		args = append(args, "-i")
	}
	if options.Onto != "" {
		args = append(args, "--onto", options.Onto)
	}
	if options.PreserveMerges {
		args = append(args, "--rebase-merges")
	}
	if options.Autosquash {
		args = append(args, "--autosquash")
	}
	if options.UpdateRefs {
		args = append(args, "--update-refs")
	}
	args = append(args, upstream)
	res, err := o.run(ctx, "rebase", args...)
	if err == nil {
		return RebaseResult{Outcome: OutcomeDone}
	}
	out := append(res.Stdout, res.Stderr...)
	if isConflict(err, out) {
		conflicts, _ := o.Conflicts(ctx)
		return RebaseResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	text := strings.ToLower(string(out) + " " + err.Error())
	if strings.Contains(text, "nothing to do") || strings.Contains(text, "up to date") {
		return RebaseResult{Outcome: OutcomeNothingToDo}
	}
	if strings.Contains(text, "stopped at") || strings.Contains(text, "editing") || strings.Contains(text, "reword") {
		return RebaseResult{Outcome: OutcomeStopped, StoppedAction: strings.TrimSpace(string(out))}
	}
	return RebaseResult{Outcome: OutcomeError, Err: opError("rebase", err)}
}

// InteractiveRebase captures Git's generated todo, applies edit, then runs the
// rebase again with the edited todo. The first invocation stops in the editor
// before Git can change HEAD or the worktree.
func (o Operations) InteractiveRebase(ctx context.Context, onto string, edit func([]RebaseEntry) ([]RebaseEntry, error)) RebaseResult {
	if err := ctx.Err(); err != nil {
		return RebaseResult{Outcome: OutcomeError, Err: err}
	}
	executable := o.Editor
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return RebaseResult{Outcome: OutcomeError, Err: opError("locate r-git editor", err)}
		}
	}
	tempDir, err := os.MkdirTemp("", "r-git-rebase-editor-*")
	if err != nil {
		return RebaseResult{Outcome: OutcomeError, Err: opError("create rebase editor files", err)}
	}
	defer os.RemoveAll(tempDir)
	todoPath := filepath.Join(tempDir, "todo")
	args := []string{"-i", onto}
	_, captureErr := o.runWithEnv(ctx, "rebase", map[string]string{
		"GIT_SEQUENCE_EDITOR": editorCommand(executable, "capture", "R_GIT_EDITOR_OUT", todoPath),
	}, args...)
	data, err := os.ReadFile(todoPath)
	if err != nil {
		if captureErr == nil {
			return RebaseResult{Outcome: OutcomeNothingToDo}
		}
		return classifyRebaseFailure(ctx, o, captureErr, nil)
	}
	entries, err := ParseTodo(data)
	if err != nil {
		return RebaseResult{Outcome: OutcomeError, Err: err}
	}
	if edit != nil {
		entries, err = edit(entries)
		if err != nil {
			return RebaseResult{Outcome: OutcomeError, Err: err}
		}
	}
	entries, err = ApplyTodoModel(entries)
	if err != nil {
		return RebaseResult{Outcome: OutcomeError, Err: err}
	}
	if err := WriteTodo(todoPath, entries); err != nil {
		return RebaseResult{Outcome: OutcomeError, Err: err}
	}
	sequenceEditor := editorCommand(executable, "apply", "R_GIT_EDITOR_IN", todoPath)
	env := map[string]string{"GIT_SEQUENCE_EDITOR": sequenceEditor, "GIT_EDITOR": "true"}
	for _, entry := range entries {
		if entry.Action == "reword" || entry.Action == "r" {
			messagePath := filepath.Join(tempDir, "message")
			if err := os.WriteFile(messagePath, []byte(entry.Subject+"\n"), 0600); err != nil {
				return RebaseResult{Outcome: OutcomeError, Err: err}
			}
			env["GIT_EDITOR"] = editorCommand(executable, "message", "R_GIT_EDITOR_IN", messagePath)
			break
		}
	}
	res, err := o.runWithEnv(ctx, "rebase", env, args...)
	if err == nil {
		return RebaseResult{Outcome: OutcomeDone}
	}
	return classifyRebaseFailure(ctx, o, err, append(res.Stdout, res.Stderr...))
}

func editorCommand(executable, mode, pathVariable, path string) string {
	return fmt.Sprintf("R_GIT_EDITOR_MODE=%s %s=%s %s", shellQuote(mode), pathVariable, shellQuote(path), shellQuote(strings.ReplaceAll(executable, `\`, "/")))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func classifyRebaseFailure(ctx context.Context, o Operations, err error, output []byte) RebaseResult {
	if err == nil {
		return RebaseResult{Outcome: OutcomeDone}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return RebaseResult{Outcome: OutcomeError, Err: err}
	}
	if isConflict(err, output) {
		conflicts, _ := o.Conflicts(ctx)
		return RebaseResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	text := strings.ToLower(string(output) + " " + err.Error())
	if strings.Contains(text, "nothing to do") || strings.Contains(text, "up to date") {
		return RebaseResult{Outcome: OutcomeNothingToDo}
	}
	return RebaseResult{Outcome: OutcomeError, Err: opError("interactive rebase", err)}
}

// ContinueRebase resumes after conflicts or an edit stop.
func (o Operations) ContinueRebase(ctx context.Context) RebaseResult {
	return o.rebaseControl(ctx, "--continue")
}
func (o Operations) SkipRebase(ctx context.Context) RebaseResult {
	return o.rebaseControl(ctx, "--skip")
}
func (o Operations) AbortRebase(ctx context.Context) RebaseResult {
	return o.rebaseControl(ctx, "--abort")
}
func (o Operations) rebaseControl(ctx context.Context, action string) RebaseResult {
	res, err := o.runWithEnv(ctx, "rebase", map[string]string{"GIT_EDITOR": "true"}, action)
	if err == nil {
		return RebaseResult{Outcome: OutcomeDone}
	}
	if isConflict(err, append(res.Stdout, res.Stderr...)) {
		conflicts, _ := o.Conflicts(ctx)
		return RebaseResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	return RebaseResult{Outcome: OutcomeError, Err: opError("rebase "+strings.TrimPrefix(action, "--"), err)}
}
