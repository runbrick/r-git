package ops

import (
	"context"
	"strings"
)

type MergeOptions struct {
	NoFastForward   bool
	FastForwardOnly bool
	Squash          bool
	NoCommit        bool
	Message         string
}

func (o Operations) Merge(ctx context.Context, target string, options MergeOptions) MergeResult {
	args := make([]string, 0, 8)
	if options.NoFastForward {
		args = append(args, "--no-ff")
	}
	if options.FastForwardOnly {
		args = append(args, "--ff-only")
	}
	if options.Squash {
		args = append(args, "--squash")
	}
	if options.NoCommit {
		args = append(args, "--no-commit")
	}
	if options.Message != "" {
		args = append(args, "-m", options.Message)
	}
	args = append(args, "--", target)
	res, err := o.run(ctx, "merge", args...)
	if err == nil {
		return MergeResult{Outcome: OutcomeDone}
	}
	if isConflict(err, append(res.Stdout, res.Stderr...)) {
		conflicts, _ := o.Conflicts(ctx)
		return MergeResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	if strings.Contains(strings.ToLower(err.Error()), "up to date") {
		return MergeResult{Outcome: OutcomeNothingToDo}
	}
	return MergeResult{Outcome: OutcomeError, Err: opError("merge", err)}
}

func (o Operations) ContinueMerge(ctx context.Context, message string) MergeResult {
	args := []string{"--no-edit"}
	if message != "" {
		args = []string{"-m", message}
	}
	res, err := o.run(ctx, "commit", args...)
	if err == nil {
		return MergeResult{Outcome: OutcomeDone}
	}
	if isConflict(err, append(res.Stdout, res.Stderr...)) {
		conflicts, _ := o.Conflicts(ctx)
		return MergeResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	return MergeResult{Outcome: OutcomeError, Err: opError("continue merge", err)}
}

func (o Operations) AbortMerge(ctx context.Context) error {
	_, err := o.run(ctx, "merge", "--abort")
	return opError("abort merge", err)
}
