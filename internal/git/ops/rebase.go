package ops

import (
	"context"
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
