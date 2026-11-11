package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/runbrick/r-git/internal/git/command"
)

type MergeOptions struct {
	NoFastForward   bool
	FastForwardOnly bool
	Squash          bool
	NoCommit        bool
	Message         string
}

func (o *Service) Merge(ctx context.Context, target string, options MergeOptions) MergeResult {
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
	err = classify(res, err, command.Merge, "merge")
	if err == nil {
		return MergeResult{Outcome: OutcomeDone}
	}
	if err == ErrConflict {
		conflicts, _ := o.Conflicts(ctx)
		return MergeResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	if strings.Contains(strings.ToLower(err.Error()), "up to date") {
		return MergeResult{Outcome: OutcomeNothingToDo}
	}
	return MergeResult{Outcome: OutcomeError, Err: err}
}

func (o *Service) ContinueMerge(ctx context.Context, message string) MergeResult {
	args := []string{"--no-edit"}
	if message != "" {
		args = []string{"-m", message}
	}
	res, err := o.run(ctx, "commit", args...)
	err = classify(res, err, command.Merge, "continue merge")
	if err == nil {
		return MergeResult{Outcome: OutcomeDone}
	}
	if err == ErrConflict {
		conflicts, _ := o.Conflicts(ctx)
		return MergeResult{Outcome: OutcomeConflicts, Conflicts: conflicts}
	}
	return MergeResult{Outcome: OutcomeError, Err: err}
}

func (o *Service) AbortMerge(ctx context.Context) error {
	_, err := o.run(ctx, "merge", "--abort")
	if err != nil {
		return fmt.Errorf("abort merge: %w", err)
	}
	return nil
}
